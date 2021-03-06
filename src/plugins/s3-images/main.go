// This file is an example of an S3-pulling plugin.  This is a real-world
// plugin that can actually be used in a production environment (compared to
// the more general but dangerous "external-images" plugin).  This requires you
// to put your AWS access key information into the environment per AWS's
// standard credential management: AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY.
// You may also put access keys in $HOME/.aws/credentials (or
// docker/s3credentials if you're using the docker-compose example override
// setup).  See docker/s3credentials.example for an example credentials file.
//
// When a resource is requested, if its IIIF id begins with "s3://", we treat
// the rest of the id as an s3 bucket and id to be pulled from S3 object
// storage.  As credentials are configured on the server end, attack vectors
// seen in the external images plugin are effectively nullified.
//
// We assume the asset is already a format RAIS can serve (preferably JP2), and
// we cache it locally with the same extension it has in S3.  The IDToPath
// return is the cached path so that RAIS can use the cached file immediately
// after download.  The JP2 cache is configurable via `S3Cache` in the RAIS
// toml file or by setting `RAIS_S3CACHE` in the environment, and defaults to
// `/var/cache/rais-s3`.
//
// Expiration of cached files must be managed externally (to avoid
// over-complicating this plugin).  A simple approach could be a cron job that
// wipes out all cached data if it hasn't been accessed in the past 24 hours:
//
//     find /var/cache/rais-s3 -type f -atime +1 -exec rm {} \;
//
// Depending how fast the cache grows, how much disk space you have available,
// and how much variety you have in S3, you may want to monitor the cache
// closely and tweak this cron job example as needed, or come up with something
// more sophisticated.

package main

import (
	"errors"
	"rais/src/iiif"
	"rais/src/plugins"
	"time"

	"github.com/spf13/viper"
	"github.com/uoregon-libraries/gopkg/fileutil"
	"github.com/uoregon-libraries/gopkg/logger"
)

var l = logger.Named("rais/s3-plugin", logger.Debug)

var s3cache, s3zone, s3endpoint string
var cacheLifetime time.Duration

// Disabled lets the plugin manager know not to add this plugin's functions to
// the global list unless sanity checks in Initialize() pass
var Disabled = true

// Initialize sets up package variables for the s3 pulls and verifies sanity of
// some of the configuration
func Initialize() {
	viper.SetDefault("S3Cache", "/var/local/rais-s3")
	s3cache = viper.GetString("S3Cache")
	s3zone = viper.GetString("S3Zone")
	s3endpoint = viper.GetString("S3Endpoint")

	if s3zone == "" {
		l.Infof("S3 plugin will not be enabled: S3Zone must be set in rais.toml or RAIS_S3ZONE must be set in the environment")
		return
	}

	// This is an undocumented feature: it's a bit experimental, and really not
	// something that should be relied upon until it gets some testing.
	viper.SetDefault("S3CacheLifetime", "0")
	var lifetimeString = viper.GetString("S3CacheLifetime")
	var err error
	cacheLifetime, err = time.ParseDuration(lifetimeString)
	if err != nil {
		l.Fatalf("S3 plugin failure: malformed S3CacheLifetime (%q): %s", lifetimeString, err)
	}

	l.Debugf("Setting S3 cache location to %q", s3cache)
	l.Debugf("Setting S3 zone to %q", s3zone)
	if cacheLifetime > time.Duration(0) {
		l.Debugf("Setting S3 cache expiration to %s", cacheLifetime)
		go purgeLoop()
	}
	Disabled = false

	if fileutil.IsDir(s3cache) {
		return
	}
	if !fileutil.MustNotExist(s3cache) {
		l.Fatalf("S3 plugin failure: %q must not exist or else must be a directory", s3cache)
	}
}

// SetLogger is called by the RAIS server's plugin manager to let plugins use
// the central logger
func SetLogger(raisLogger *logger.Logger) {
	l = raisLogger
}

// IDToPath implements the auto-download logic when a IIIF ID
// starts with "s3://"
func IDToPath(id iiif.ID) (path string, err error) {
	var a, _ = lookupAsset(id)
	if a.key == "" {
		return "", plugins.ErrSkipped
	}

	// See if this file is currently being downloaded; if so we need to wait
	var timeout = time.Now().Add(time.Second * 10)
	for a.tryFLock() == false {
		time.Sleep(time.Millisecond * 250)
		if time.Now().After(timeout) {
			return "", errors.New("timed out waiting for locked asset (probably very slow download)")
		}
	}

	// Let the asset know it's being read
	a.read()

	// Attempt to download the asset content
	err = a.download()
	a.fUnlock()

	return a.path, err
}

// PurgeCaches deletes all cached files this plugin is tracking.  Deletion
// happens in the background so the API isn't sitting for potentially many
// minutes prior to responding to the caller.
//
// TODO: this plugin should index files on the filesystem to see if there are
// any it should be tracking (this happens if RAIS is ever shut down while
// tracking files).  We don't want to delay startup, though.  Options:
//     - On shutdown, write out the assets map - then on startup we can just
//       read it in again and reset purge times
//     - On startup fire up a background thread that just instantiates assets
//       via lookupAsset(basename(file-".ext")).  If the filename is always the
//       IIIF ID, this should work, and doesn't need to block since it'll only
//       lock on the lookupAsset call.
func PurgeCaches() {
	// lock all assets while indexing them so we can index everything RAIS
	// *currently* knows about without things getting weird if new stuff is being
	// indexed during the process
	assetMutex.Lock()
	var ids []iiif.ID
	for _, a := range assets {
		ids = append(ids, a.id)
	}
	assetMutex.Unlock()
	go purgeCaches(ids)
}

// purgeCaches synchronously purges a list of assets from the filesystem cache,
// pausing briefly between each purge so this can run in the background without
// hammering the disk.
func purgeCaches(ids []iiif.ID) {
	for _, id := range ids {
		ExpireCachedImage(id)
		time.Sleep(time.Millisecond * 250)
	}
	l.Infof("s3-images plugin: mass-purged %d assets", len(ids))
}

// ExpireCachedImage gets rid of any cached image for the given id, should it
// exist.  We don't really care if it doesn't exist, though, as that can mean
// it's already been purged, or RAIS was restarted and the whole cache removed,
// etc.
func ExpireCachedImage(id iiif.ID) {
	var a, ok = lookupAsset(id)
	var infoMsgFmt = "s3-images plugin: purging %q: %s"
	if ok {
		doPurge(a)
		l.Infof(infoMsgFmt, id, "success")
	} else {
		assetMutex.Lock()
		delete(assets, a.id)
		assetMutex.Unlock()
		l.Debugf(infoMsgFmt, id, "no local asset cached")
	}
}

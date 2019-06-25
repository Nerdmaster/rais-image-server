//+build ignore

package openjpeg

import (
	"io"
	"os"
	"rais/src/iiif"
	"sort"
	"sync"
	"time"
)

// FSAssetManager tracks and manages the lifecycle of all JP2 assets that have
// been read from the filesystem
type FSAssetManager struct {
	sync.RWMutex
	lookup  map[string]fsAsset
	ramSize uint64
	maxRAM  uint64
}

// fsAsset represents in-memory data for a JP2 on the filesystem
type fsAsset struct {
	sync.Mutex
	identifier string
	lastAccess time.Time
	data       []byte
}

// NewFSAssetManager initializes an asset manager that will hold no more than
// maxram bytes of JP2s in memory.  Assets looked up are expected to be given a
// file path for the identifier.
func NewFSAssetManager(maxram uint64) *FSAssetManager {
	return &FSAssetManager{lookup: make(map[string]*FSAsset), maxRAM: maxram}
}

// LookupAsset checks the cached assets for the given identifier, otherwise it
// tries to find a JP2 on the filesystem where the file path is the given
// identifier.  If found, the asset's entire JP2 is read into memory.
func (fam *FSAssetManager) LookupAsset(identifier string) (*FSAsset, error) {
	fam.Lock()
	defer fam.Unlock()

	var a, ok = fam.lookup[identifier]
	if ok {
		a.accessed()
		return a, nil
	}

	var a = &FSAsset{identifier: identifier, lastAccess: time.Now()}
	var err = a.read()
	if err != nil {
		return nil, err
	}

	fam.lookup[identifier] = a
	fam.ramSize += len(a.data)
	fam.purgeOldCachedItems()

	return a, nil
}

func (a *FSAsset) read() error {
	var f, err = os.Open(a.identifier)
	if err != nil {
		return err
	}

	var info os.FileInfo
	info, err = f.Stat()
	if err != nil {
		return err
	}

	a.data = make([]byte, info.Size())
	_, err = io.ReadFull(f, a.data)
	if err != nil {
		return err
	}

	// We can ignore the error here as it was a read-only operation and to get to
	// this point in the code we must have read all data
	f.Close()

	return nil
}

// accessed just says the asset has been used in some way, so its last access
// timestamp needs to be updated
func (a *FSAsset) accessed() {
	a.Lock()
	a.lastAccess = time.Now()
	a.Unlock()
}

func (a *FSAsset) free() {
	assetMutex.Lock()
	delete(assetCache, a.identifier)
	a.data = nil
	assetMutex.Unlock()
}

// purgeOldCachedItems removes the oldest items (by access time) from the cache
// until the cached data is below our maximum RAM usage unless there are two or
// fewer items in the cache.  No RAM limits would make sense if it means we
// purge the cache every time a JP2 is looked up.  Actually, even two assets is
// probably too small, but we have to draw the line somewhere....
//
// To avoid purging on every new asset, we don't simply remove a single asset
// from the list or something - we purge down until our RAM usage is *half* the
// maximum.
//
// The lock must be held before calling this.
func (fam *FSAssetManager) purgeOldCachedItems() {
	if len(fam.lookup) <= 2 || fam.ramSize > fam.maxRAM {
		return
	}

	// Aggregate all assets and sort them by access time
	for _, a := range fam.lookup {
	}
	sort.Slice(allAssets, func(i, j int) bool {
		return allAssets[i].lastAccess.Before(allAssets[j].lastAccess)
	})

	var a *FSAsset
	for fam.ramSize > fam.maxRAM/2 {
		a, allAssets = allAssets[0], allAssets[1:]
		heldBytes -= len(a.data)
		a.free()
	}

	assetMutex.Unlock()
}

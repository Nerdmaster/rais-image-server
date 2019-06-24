package openjpeg

import (
	"io"
	"os"
	"sync"
	"time"
)

// assetCache stores the last few JP2s in memory to speed up processing of
// subsequent image requests
var assetCache = make(map[string]*asset)
var assetMutex sync.Mutex

var cacheLifetime time.Duration = 5 * time.Minute

type asset struct {
	filename   string
	inUse      bool
	fs         sync.Mutex
	lockreader sync.Mutex
	lastAccess time.Time
	data       []byte
	cached     bool
}

func newAsset(filename string) (*asset, error) {
	var a = &asset{filename: filename}
	return a, a.read()
}

func lookupAsset(filename string) (*asset, error) {
	assetMutex.Lock()
	var a, ok = assetCache[filename]
	if !ok {
		var err error
		a, err = newAsset(filename)
		if err != nil {
			return nil, err
		}
		assetCache[filename] = a
	}
	assetMutex.Unlock()

	a.lastAccess = time.Now().Add(cacheLifetime)
	return a, nil
}

// tryFLock attempts to lock for file writing in a non-blocking way.  If the
// lock can be acquired, the return is true, otherwise false.
func (a *asset) tryFLock() bool {
	a.lockreader.Lock()
	var inUse = a.inUse
	if !inUse {
		a.fs.Lock()
		a.inUse = true
	}
	a.lockreader.Unlock()

	return !inUse
}

// For when master Yoda's around.  There is no try.
func (a *asset) fLock() {
	a.lockreader.Lock()
	a.fs.Lock()
	a.inUse = true
	a.lockreader.Unlock()
}

func (a *asset) fUnlock() {
	a.lockreader.Lock()
	a.inUse = false
	a.fs.Unlock()
	a.lockreader.Unlock()
}

func (a *asset) read() error {
	var f, err = os.Open(a.filename)
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

	a.cached = true
	return nil
}

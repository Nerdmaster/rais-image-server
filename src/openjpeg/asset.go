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
	sync.Mutex
	filename   string
	inUse      bool
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

	a.Lock()
	a.lastAccess = time.Now().Add(cacheLifetime)
	a.Unlock()
	return a, nil
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

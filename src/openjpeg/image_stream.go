package openjpeg

// #cgo pkg-config: libopenjp2
// #include <openjpeg.h>
import "C"
import (
	"io"
	"reflect"
	"sync"
	"unsafe"
)

var nextStreamID uint64
var imageStreams = make(map[uint64]*imageStream)
var imageStreamMutex sync.RWMutex

// These are stupid, but we need to return what openjpeg considers failure
// numbers, and Go doesn't allow a direct translation of negative values to an
// unsigned type
var opjZero64 C.OPJ_UINT64 = 0
var opjMinusOne64 = opjZero64 - 1
var opjZeroSizeT C.OPJ_SIZE_T = 0
var opjMinusOneSizeT = opjZeroSizeT - 1

type imageStream struct {
	id     uint64
	stream io.ReadSeeker
}

func newImageStream(stream io.ReadSeeker) *imageStream {
	imageStreamMutex.Lock()

	nextStreamID++
	var s = &imageStream{id: nextStreamID, stream: stream}
	imageStreams[s.id] = s

	imageStreamMutex.Unlock()

	return s
}

func lookupStream(id uint64) (*imageStream, bool) {
	imageStreamMutex.Lock()
	var ms, ok = imageStreams[id]
	imageStreamMutex.Unlock()

	return ms, ok
}

//export freeStream
func freeStream(id uint64) {
	imageStreamMutex.Lock()

	delete(imageStreams, id)

	imageStreamMutex.Unlock()
}

//export opjStreamRead
func opjStreamRead(writeBuffer unsafe.Pointer, numBytes C.OPJ_SIZE_T, streamID C.OPJ_UINT64) C.OPJ_SIZE_T {
	var s, ok = lookupStream(uint64(streamID))
	if !ok {
		Logger.Errorf("Unable to find stream %d", streamID)
		return opjMinusOne64
	}

	var data []byte
	var dataSlice = (*reflect.SliceHeader)(unsafe.Pointer(&data))
	dataSlice.Cap = int(numBytes)
	dataSlice.Len = int(numBytes)
	dataSlice.Data = uintptr(unsafe.Pointer(writeBuffer))

	var n, err = s.stream.Read(data)
	if err != nil {
		if err != io.EOF {
			Logger.Errorf("Unable to read from stream %d: %s", streamID, err)
		}
		return opjMinusOne64
	}

	return C.OPJ_SIZE_T(n)
}

//export opjStreamSkip
//
// opjStreamSkip jumps numBytes ahead in the stream, discarding any data that would be read
func opjStreamSkip(numBytes C.OPJ_OFF_T, streamID C.OPJ_UINT64) C.OPJ_SIZE_T {
	var s, ok = lookupStream(uint64(streamID))
	if !ok {
		Logger.Errorf("Unable to find stream ID %d", streamID)
		return opjMinusOneSizeT
	}
	var _, err = s.stream.Seek(int64(numBytes), io.SeekCurrent)
	if err != nil {
		Logger.Errorf("Unable to seek %d bytes forward: %s", numBytes, err)
		return opjMinusOneSizeT
	}

	// For some reason, success here seems to be a return value of the number of bytes passed in
	return C.OPJ_SIZE_T(numBytes)
}

//export opjStreamSeek
//
// opjStreamSeek jumps to the absolute position offset in the stream
func opjStreamSeek(offset C.OPJ_OFF_T, streamID C.OPJ_UINT64) C.OPJ_BOOL {
	var s, ok = lookupStream(uint64(streamID))
	if !ok {
		Logger.Errorf("Unable to find stream ID %d", streamID)
		return C.OPJ_FALSE
	}
	var _, err = s.stream.Seek(int64(offset), io.SeekStart)
	if err != nil {
		Logger.Errorf("Unable to seek to offset %d: %s", offset, err)
		return C.OPJ_FALSE
	}

	return C.OPJ_TRUE
}
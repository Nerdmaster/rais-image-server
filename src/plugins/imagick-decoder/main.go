// Package magick is a hacked up port of the minimal functionality we need
// to satisfy the img.Decoder interface.  Code is based in part on
// github.com/quirkey/magick
package main

/*
#cgo pkg-config: MagickCore
#include <magick/MagickCore.h>
*/
import "C"
import (
	"fmt"
	"os"
	"path/filepath"
	"rais/src/img"
	"unsafe"

	"github.com/uoregon-libraries/gopkg/logger"
)

var l *logger.Logger

// SetLogger is called by the RAIS server's plugin manager to let plugins use
// the central logger
func SetLogger(raisLogger *logger.Logger) {
	l = raisLogger
}

// Initialize sets up the MagickCore stuff
func Initialize() {
	path, _ := os.Getwd()
	cPath := C.CString(path)
	defer C.free(unsafe.Pointer(cPath))
	C.MagickCoreGenesis(cPath, C.MagickFalse)
}

func makeError(exception *C.ExceptionInfo) error {
	return fmt.Errorf("%v: %v - %v", exception.severity, exception.reason, exception.description)
}

// ImageDecoders returns our list of one: the magick decoder used for the image
// types we support
func ImageDecoders() []img.DecodeFn {
	return []img.DecodeFn{decodeCommonFile}
}

func decodeCommonFile(path string) (img.Decoder, error) {
	switch filepath.Ext(path) {
	case ".tif", ".tiff", ".png", ".jpg", "jpeg", ".gif":
		return NewImage(path)
	default:
		return nil, img.ErrNotHandled
	}
}

package main

import (
	"path/filepath"
	"rais/src/img"
	"rais/src/openjpeg"
)

func decodeJP2(path string) (img.Decoder, error) {
	if filepath.Ext(path) == ".jp2" {
		return openjpeg.NewJP2Image(path)
	}
	return nil, img.ErrNotHandled
}

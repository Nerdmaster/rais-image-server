package openjpeg

// #cgo pkg-config: libopenjp2
// #include <openjpeg.h>
// #include <stdlib.h>
// #include "handlers.h"
// #include "stream.h"
import "C"

import (
	"bytes"
	"fmt"
)

// rawDecode runs the low-level operations necessary to actually get the
// desired tile/resized image
func (i *JP2Image) rawDecode() (jp2 *C.opj_image_t, err error) {
	// Setup the parameters for decode
	var parameters C.opj_dparameters_t
	C.opj_set_default_decoder_parameters(&parameters)

	// Calculate cp_reduce - this seems smarter to put in a parameter than to call an extra function
	parameters.cp_reduce = C.OPJ_UINT32(i.computeProgressionLevel())

	// Setup file stream
	stream, err := initializeStream(i.filename)
	if err != nil {
		return jp2, err
	}
	defer C.opj_stream_destroy(stream)

	// Create codec
	codec := C.opj_create_decompress(C.OPJ_CODEC_JP2)
	defer C.opj_destroy_codec(codec)

	// Connect our info/warning/error handlers
	C.set_handlers(codec)

	// Fill in codec configuration from parameters
	if C.opj_setup_decoder(codec, &parameters) == C.OPJ_FALSE {
		return jp2, fmt.Errorf("unable to setup decoder")
	}

	// Read the header to set up the image data
	if C.opj_read_header(stream, codec, &jp2) == C.OPJ_FALSE {
		return jp2, fmt.Errorf("failed to read the header")
	}

	// Set the decode area if it isn't the full image
	if i.decodeArea != i.srcRect {
		r := i.decodeArea
		if C.opj_set_decode_area(codec, jp2, C.OPJ_INT32(r.Min.X), C.OPJ_INT32(r.Min.Y), C.OPJ_INT32(r.Max.X), C.OPJ_INT32(r.Max.Y)) == C.OPJ_FALSE {
			return jp2, fmt.Errorf("failed to set the decoded area")
		}
	}

	// Decode the JP2 into the image stream
	if C.opj_decode(codec, stream, jp2) == C.OPJ_FALSE || C.opj_end_decompress(codec, stream) == C.OPJ_FALSE {
		return jp2, fmt.Errorf("failed to decode image")
	}

	return jp2, nil
}

func findJP2Stream(id iiif.ID) {
	var filepath = iiif
}

func initializeStream(id iiif.ID) (*C.opj_stream_t, error) {
	var asset, err = lookupAsset(id)
	if err != nil {
		return nil, fmt.Errorf("Unable to lookup %q: %s", filename, err)
	}

	var s = newImageStream(bytes.NewReader(asset.data))
	stream := C.new_stream(C.OPJ_UINT64(1024*10), C.OPJ_UINT64(s.id), C.OPJ_UINT64(len(asset.data)))
	if stream == nil {
		return nil, fmt.Errorf("failed to create stream in %#v", filename)
	}

	return stream, nil
}

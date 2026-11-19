// Package imagemeta removes privacy-sensitive metadata (EXIF, XMP, IPTC,
// text chunks, comments, thumbnails, ...) from JPEG, PNG and WebP images
// without re-encoding the pixel data.
//
// Formats are identified by sniffing the leading bytes, never by file name or
// client-supplied Content-Type. Anything that is not one of the supported
// formats is returned untouched.
//
// Stripping works on the container structure only: segments/chunks that are
// not needed to render the image are dropped, everything else is copied
// byte-for-byte. The EXIF Orientation tag is the one exception to "drop all
// EXIF": without it phone photos would display rotated, so when a non-default
// orientation is present a minimal EXIF block that contains only that tag is
// written back.
//
// Policy is allow-list based: unknown segments/chunks are dropped, except
// unknown critical PNG chunks which are kept so the image is not broken.
package imagemeta

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
)

// Format identifies a container format handled by this package.
type Format int

const (
	// FormatUnknown is any content that is not a supported image format.
	FormatUnknown Format = iota
	FormatJPEG
	FormatPNG
	FormatWebP
)

func (f Format) String() string {
	switch f {
	case FormatJPEG:
		return "jpeg"
	case FormatPNG:
		return "png"
	case FormatWebP:
		return "webp"
	default:
		return "unknown"
	}
}

// ErrInvalidImage is returned (wrapped) when the content was identified as a
// supported image format but is truncated or structurally corrupt, so it
// cannot be safely stripped.
var ErrInvalidImage = errors.New("invalid or corrupt image")

// sniffLen is the number of leading bytes Detect needs.
const sniffLen = 12

var pngSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

// Detect identifies the image format from the first bytes of a file.
func Detect(head []byte) Format {
	switch {
	case len(head) >= 3 && head[0] == 0xFF && head[1] == 0xD8 && head[2] == 0xFF:
		return FormatJPEG
	case bytes.HasPrefix(head, pngSignature):
		return FormatPNG
	case len(head) >= sniffLen && string(head[0:4]) == "RIFF" && string(head[8:12]) == "WEBP":
		return FormatWebP
	default:
		return FormatUnknown
	}
}

// Strip returns data with metadata removed. Content that is not a supported
// image is returned as-is with FormatUnknown. If the content is a supported
// format but malformed, the returned error wraps ErrInvalidImage.
func Strip(data []byte) ([]byte, Format, error) {
	f := Detect(data)
	var (
		out []byte
		err error
	)
	switch f {
	case FormatJPEG:
		out, err = stripJPEG(data)
	case FormatPNG:
		out, err = stripPNG(data)
	case FormatWebP:
		out, err = stripWebP(data)
	default:
		return data, FormatUnknown, nil
	}
	if err != nil {
		return nil, f, fmt.Errorf("%w: %s: %v", ErrInvalidImage, f, err)
	}
	return out, f, nil
}

// StripReader is the streaming-friendly entry point. Non-image content is
// passed through as a stream without being buffered. Supported images are
// read fully into memory (callers are expected to bound the input size, e.g.
// via the HTTP body limit) and returned stripped.
func StripReader(r io.Reader) (io.Reader, Format, error) {
	br := bufio.NewReaderSize(r, 4096)
	head, err := br.Peek(sniffLen)
	if err != nil && err != io.EOF && !errors.Is(err, bufio.ErrBufferFull) {
		return nil, FormatUnknown, err
	}
	if Detect(head) == FormatUnknown {
		return br, FormatUnknown, nil
	}
	data, err := io.ReadAll(br)
	if err != nil {
		return nil, FormatUnknown, err
	}
	out, f, err := Strip(data)
	if err != nil {
		return nil, f, err
	}
	return bytes.NewReader(out), f, nil
}

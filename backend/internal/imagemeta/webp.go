package imagemeta

import (
	"encoding/binary"
	"errors"
)

const (
	webpFlagEXIF  = 0x08
	webpFlagXMP   = 0x04
	webpHeaderLen = 12
)

// webpKeep lists the chunks copied through: the image data and what is needed
// to render it (ICC profile, animation). EXIF, XMP and unknown chunks are
// dropped.
var webpKeep = map[string]bool{
	"VP8X": true, "VP8 ": true, "VP8L": true, "ALPH": true,
	"ICCP": true, "ANIM": true, "ANMF": true,
}

// stripWebP rewrites the RIFF container without EXIF/XMP chunks, clears the
// matching VP8X flags and fixes the RIFF size. If an EXIF chunk carried a
// non-default Orientation, a minimal EXIF chunk with only that tag is
// appended (after the image data, as the spec orders it) and its flag set.
func stripWebP(data []byte) ([]byte, error) {
	riffSize := uint64(binary.LittleEndian.Uint32(data[4:8]))
	if riffSize < 4 || riffSize+8 > uint64(len(data)) {
		return nil, errors.New("truncated: RIFF size exceeds data")
	}
	data = data[:riffSize+8] // ignore anything after the RIFF payload

	out := make([]byte, webpHeaderLen, len(data))
	copy(out, data[:webpHeaderLen])

	var (
		pos         = webpHeaderLen
		vp8xAt      = -1
		sawImage    bool
		orientation int
	)
	for pos < len(data) {
		if pos+8 > len(data) {
			return nil, errors.New("truncated chunk header")
		}
		fourcc := string(data[pos : pos+4])
		size := uint64(binary.LittleEndian.Uint32(data[pos+4 : pos+8]))
		padded := size + size&1
		end := uint64(pos) + 8 + padded
		if end > uint64(len(data)) {
			// Tolerate only a missing final pad byte.
			if size&1 == 1 && end-1 == uint64(len(data)) {
				end--
			} else {
				return nil, errors.New("truncated chunk")
			}
		}
		body := data[pos+8 : pos+8+int(size)]
		chunk := data[pos:end]
		pos = int(end)

		switch {
		case fourcc == "EXIF":
			if orientation == 0 {
				orientation = exifOrientation(body)
			}
		case webpKeep[fourcc]:
			switch fourcc {
			case "VP8X":
				if size < 10 || vp8xAt >= 0 {
					return nil, errors.New("invalid VP8X chunk")
				}
				vp8xAt = len(out)
			case "VP8 ", "VP8L", "ANMF":
				sawImage = true
			}
			out = append(out, chunk...)
		}
	}
	if !sawImage {
		return nil, errors.New("no image data")
	}

	// EXIF can only be declared in an extended-format file (VP8X); a file
	// without one cannot legitimately carry it, so the tag is just dropped.
	if vp8xAt < 0 {
		orientation = 0
	}
	if orientation != 0 {
		body := minimalExif(orientation)
		var hdr [8]byte
		copy(hdr[:4], "EXIF")
		binary.LittleEndian.PutUint32(hdr[4:], uint32(len(body)))
		out = append(out, hdr[:]...)
		out = append(out, body...) // 26 bytes, already even
	}
	if vp8xAt >= 0 {
		flags := &out[vp8xAt+8]
		*flags &^= webpFlagEXIF | webpFlagXMP
		if orientation != 0 {
			*flags |= webpFlagEXIF
		}
	}
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(out)-8))
	return out, nil
}

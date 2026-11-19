package imagemeta

import (
	"bytes"
	"errors"
)

// stripJPEG walks the marker segments up to and including EOI. Kept segments
// are copied verbatim; entropy-coded scan data is copied untouched.
//
// Kept: APP0 (JFIF), APP2 carrying an ICC profile, APP14 (Adobe, which
// determines the colour transform), and every non-APPn/COM segment (SOFn,
// DQT, DHT, DRI, SOS, ...). Dropped: APP1 (Exif, XMP), APP13 (IPTC/Photoshop),
// JFXX thumbnails, MPF and every other APPn, plus COM comments. Anything after
// the first EOI (trailers, appended thumbnails or motion-photo payloads) is
// dropped as well.
func stripJPEG(data []byte) ([]byte, error) {
	out := make([]byte, 0, len(data))
	out = append(out, 0xFF, 0xD8)

	var (
		i           = 2
		sawSOF      bool
		sawSOS      bool
		orientation int
		insertAt    = -1 // where a minimal EXIF segment is spliced in
	)

	for {
		if i >= len(data) {
			return nil, errors.New("truncated: missing EOI")
		}
		if data[i] != 0xFF {
			return nil, errors.New("expected marker")
		}
		for i < len(data) && data[i] == 0xFF { // fill bytes
			i++
		}
		if i >= len(data) {
			return nil, errors.New("truncated: missing EOI")
		}
		m := data[i]
		i++

		switch {
		case m == 0xD9: // EOI
			if !sawSOS {
				return nil, errors.New("EOI before image data")
			}
			out = append(out, 0xFF, 0xD9)
			if orientation != 0 {
				out = spliceAt(out, insertAt, exifSegment(orientation))
			}
			return out, nil
		case m == 0x00 || m == 0xD8:
			return nil, errors.New("unexpected marker")
		case m == 0x01 || (m >= 0xD0 && m <= 0xD7): // standalone markers
			out = append(out, 0xFF, m)
			continue
		}

		if i+2 > len(data) {
			return nil, errors.New("truncated segment header")
		}
		l := int(data[i])<<8 | int(data[i+1])
		if l < 2 || i+l > len(data) {
			return nil, errors.New("invalid or truncated segment")
		}
		payload := data[i+2 : i+l]
		i += l

		keep := true
		switch {
		case m == 0xE1:
			keep = false
			if orientation == 0 && bytes.HasPrefix(payload, []byte("Exif\x00\x00")) {
				orientation = exifOrientation(payload)
			}
		case m == 0xE0:
			keep = bytes.HasPrefix(payload, []byte("JFIF\x00"))
		case m == 0xE2:
			keep = bytes.HasPrefix(payload, []byte("ICC_PROFILE\x00"))
		case m == 0xEE:
			keep = bytes.HasPrefix(payload, []byte("Adobe"))
		case m >= 0xE3 && m <= 0xEF, m == 0xFE:
			keep = false
		case m >= 0xC0 && m <= 0xCF && m != 0xC4 && m != 0xC8 && m != 0xCC:
			sawSOF = true
		}

		if keep {
			// The minimal EXIF segment goes right after a leading JFIF APP0.
			isJFIF := m == 0xE0
			if insertAt < 0 && !isJFIF {
				insertAt = len(out)
			}
			out = append(out, 0xFF, m, byte(l>>8), byte(l))
			out = append(out, payload...)
		}

		if m == 0xDA { // SOS: copy entropy-coded data up to the next real marker
			if !sawSOF {
				return nil, errors.New("scan before frame header")
			}
			sawSOS = true
			start := i
			for i < len(data) {
				if data[i] != 0xFF {
					i++
					continue
				}
				if i+1 >= len(data) {
					return nil, errors.New("truncated scan data")
				}
				n := data[i+1]
				if n == 0x00 || (n >= 0xD0 && n <= 0xD7) {
					i += 2
					continue
				}
				if n == 0xFF {
					i++
					continue
				}
				break
			}
			if i >= len(data) {
				return nil, errors.New("truncated scan data")
			}
			out = append(out, data[start:i]...)
		}
	}
}

func exifSegment(orientation int) []byte {
	tiff := minimalExif(orientation)
	l := 2 + 6 + len(tiff)
	seg := []byte{0xFF, 0xE1, byte(l >> 8), byte(l)}
	seg = append(seg, "Exif\x00\x00"...)
	return append(seg, tiff...)
}

// spliceAt inserts ins into b at position at (appending if at < 0).
func spliceAt(b []byte, at int, ins []byte) []byte {
	if at < 0 || at > len(b) {
		at = len(b)
	}
	b = append(b, ins...)
	copy(b[at+len(ins):], b[at:len(b)-len(ins)])
	copy(b[at:], ins)
	return b
}

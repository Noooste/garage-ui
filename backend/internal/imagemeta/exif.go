package imagemeta

import "encoding/binary"

const orientationTag = 0x0112

// exifOrientation returns the EXIF Orientation value (2..8) stored in IFD0 of
// a TIFF block, optionally preceded by the "Exif\0\0" prefix. It returns 0 if
// the tag is absent, malformed, or has the default value 1 (nothing to keep).
func exifOrientation(b []byte) int {
	if len(b) >= 6 && string(b[:6]) == "Exif\x00\x00" {
		b = b[6:]
	}
	if len(b) < 8 {
		return 0
	}
	var bo binary.ByteOrder
	switch string(b[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 0
	}
	if bo.Uint16(b[2:4]) != 42 {
		return 0
	}
	off := uint64(bo.Uint32(b[4:8]))
	if off+2 > uint64(len(b)) {
		return 0
	}
	n := uint64(bo.Uint16(b[off : off+2]))
	for i := uint64(0); i < n; i++ {
		p := off + 2 + 12*i
		if p+12 > uint64(len(b)) {
			return 0
		}
		if bo.Uint16(b[p:p+2]) != orientationTag {
			continue
		}
		// type SHORT (3), count 1, value stored in the first two bytes.
		if bo.Uint16(b[p+2:p+4]) != 3 || bo.Uint32(b[p+4:p+8]) != 1 {
			return 0
		}
		if v := int(bo.Uint16(b[p+8 : p+10])); v >= 2 && v <= 8 {
			return v
		}
		return 0
	}
	return 0
}

// minimalExif builds a 26-byte big-endian TIFF block whose only IFD0 entry is
// the Orientation tag (no "Exif\0\0" prefix).
func minimalExif(orientation int) []byte {
	return []byte{
		'M', 'M', 0x00, 0x2A, 0, 0, 0, 8, // header, IFD0 at offset 8
		0x00, 0x01, // one entry
		0x01, 0x12, 0x00, 0x03, 0, 0, 0, 1, 0x00, byte(orientation), 0, 0, // Orientation, SHORT, count 1
		0, 0, 0, 0, // no next IFD
	}
}

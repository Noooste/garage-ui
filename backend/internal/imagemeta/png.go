package imagemeta

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
)

// pngKeep lists the chunks that are copied through. Everything else that is
// ancillary (lowercase first letter), in particular eXIf, tEXt, zTXt, iTXt,
// tIME and unknown private chunks, is dropped. Unknown critical chunks are
// kept so the image is never broken.
var pngKeep = map[string]bool{
	"IHDR": true, "PLTE": true, "IDAT": true, "IEND": true,
	// colour / rendering
	"tRNS": true, "gAMA": true, "cHRM": true, "sRGB": true, "iCCP": true,
	"sBIT": true, "cICP": true, "mDCV": true, "cLLI": true, "bKGD": true,
	"hIST": true, "pHYs": true, "sPLT": true,
	// APNG
	"acTL": true, "fcTL": true, "fdAT": true,
}

// stripPNG copies the chunk stream, dropping metadata chunks. Data after IEND
// is dropped.
func stripPNG(data []byte) ([]byte, error) {
	out := make([]byte, 0, len(data))
	out = append(out, data[:len(pngSignature)]...)

	var (
		pos         = len(pngSignature)
		first       = true
		sawIDAT     bool
		orientation int
		insertAt    = -1
	)
	for {
		if pos+8 > len(data) {
			return nil, errors.New("truncated: missing IEND")
		}
		length := uint64(binary.BigEndian.Uint32(data[pos : pos+4]))
		typ := string(data[pos+4 : pos+8])
		end := uint64(pos) + 8 + length + 4
		if length > 0x7FFFFFFF || end > uint64(len(data)) {
			return nil, errors.New("truncated or oversized chunk")
		}
		chunk := data[pos:end]
		body := data[pos+8 : pos+8+int(length)]
		pos = int(end)

		if first {
			if typ != "IHDR" || length != 13 {
				return nil, errors.New("missing IHDR")
			}
		}

		switch {
		case typ == "eXIf":
			if orientation == 0 {
				orientation = exifOrientation(body)
			}
		case pngKeep[typ] || typ[0]&0x20 == 0: // allow-listed, or unknown critical
			if typ == "IDAT" {
				sawIDAT = true
			}
			out = append(out, chunk...)
			if first {
				insertAt = len(out)
			}
		}
		first = false

		if typ == "IEND" {
			if !sawIDAT {
				return nil, errors.New("no image data")
			}
			if orientation != 0 {
				out = spliceAt(out, insertAt, pngChunk("eXIf", minimalExif(orientation)))
			}
			return out, nil
		}
	}
}

func pngChunk(typ string, body []byte) []byte {
	var buf bytes.Buffer
	var n [4]byte
	binary.BigEndian.PutUint32(n[:], uint32(len(body)))
	buf.Write(n[:])
	buf.WriteString(typ)
	buf.Write(body)
	crc := crc32.NewIEEE()
	crc.Write([]byte(typ))
	crc.Write(body)
	binary.BigEndian.PutUint32(n[:], crc.Sum32())
	buf.Write(n[:])
	return buf.Bytes()
}

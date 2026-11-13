package imagemeta

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"strings"
	"testing"
)

// ---- fixture helpers -------------------------------------------------------

func testImage() *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 24, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 24; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 10), uint8(y * 15), uint8(x*y) ^ 0x5a, 255})
		}
	}
	return img
}

// buildExif returns a TIFF block with Make, Orientation and a GPS IFD.
func buildExif(bo binary.ByteOrder, orientation uint16) []byte {
	b := make([]byte, 110)
	if bo == binary.BigEndian {
		copy(b, "MM")
	} else {
		copy(b, "II")
	}
	bo.PutUint16(b[2:], 42)
	bo.PutUint32(b[4:], 8)
	// IFD0 at 8: 3 entries
	bo.PutUint16(b[8:], 3)
	entry := func(p int, tag, typ uint16, count, val uint32) {
		bo.PutUint16(b[p:], tag)
		bo.PutUint16(b[p+2:], typ)
		bo.PutUint32(b[p+4:], count)
		bo.PutUint32(b[p+8:], val)
	}
	entry(10, 0x010F, 2, 6, 50) // Make -> "Canon\0" at 50
	copy(b[50:], "Canon\x00")
	// Orientation: SHORT inline, left-justified.
	bo.PutUint16(b[22:], orientationTag)
	bo.PutUint16(b[24:], 3)
	bo.PutUint32(b[26:], 1)
	bo.PutUint16(b[30:], orientation)
	entry(34, 0x8825, 4, 1, 56) // GPS IFD pointer
	bo.PutUint32(b[46:], 0)
	// GPS IFD at 56: 2 entries
	bo.PutUint16(b[56:], 2)
	copy(b[58+8:], "N\x00")
	bo.PutUint16(b[58:], 1)
	bo.PutUint16(b[60:], 2)
	bo.PutUint32(b[62:], 2)
	copy(b[66:], "N\x00")
	entry(70, 2, 5, 3, 86) // GPSLatitude -> 3 rationals at 86
	for i := 0; i < 3; i++ {
		bo.PutUint32(b[86+8*i:], uint32(48+i))
		bo.PutUint32(b[90+8*i:], 1)
	}
	return b
}

func jpegSeg(marker byte, payload ...[]byte) []byte {
	var p []byte
	for _, x := range payload {
		p = append(p, x...)
	}
	l := len(p) + 2
	return append([]byte{0xFF, marker, byte(l >> 8), byte(l)}, p...)
}

func baseJPEG(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, testImage(), &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// withJPEGSegments inserts segments right after SOI.
func withJPEGSegments(base []byte, segs ...[]byte) []byte {
	out := append([]byte{}, base[:2]...)
	for _, s := range segs {
		out = append(out, s...)
	}
	return append(out, base[2:]...)
}

const (
	secretXMP = "http://ns.adobe.com/xap/1.0/\x00<x:xmpmeta>SECRET-XMP-LOCATION</x:xmpmeta>"
	fakeICC   = "ICC_PROFILE\x00\x01\x01FAKE-ICC-PROFILE-BYTES"
)

func dirtyJPEG(t *testing.T, bo binary.ByteOrder, orientation uint16) []byte {
	base := baseJPEG(t)
	return withJPEGSegments(base,
		jpegSeg(0xE0, []byte("JFIF\x00\x01\x01\x00\x00\x01\x00\x01\x00\x00")),
		jpegSeg(0xE1, []byte("Exif\x00\x00"), buildExif(bo, orientation)),
		jpegSeg(0xE1, []byte(secretXMP)),
		jpegSeg(0xE2, []byte(fakeICC)),
		jpegSeg(0xE2, []byte("MPF\x00SECRET-MPF")),
		jpegSeg(0xED, []byte("Photoshop 3.0\x008BIMSECRET-IPTC")),
		jpegSeg(0xEE, []byte("Adobe\x00\x64\x00\x00\x00\x00\x01")),
		jpegSeg(0xFE, []byte("SECRET-COMMENT")),
	)
}

func pngChunks(t *testing.T, data []byte) (types []string, bodies map[string][]byte) {
	t.Helper()
	if !bytes.HasPrefix(data, pngSignature) {
		t.Fatal("not a png")
	}
	bodies = map[string][]byte{}
	pos := 8
	for pos < len(data) {
		n := int(binary.BigEndian.Uint32(data[pos:]))
		typ := string(data[pos+4 : pos+8])
		body := data[pos+8 : pos+8+n]
		crc := binary.BigEndian.Uint32(data[pos+8+n:])
		if crc != crc32.ChecksumIEEE(data[pos+4:pos+8+n]) {
			t.Fatalf("bad CRC on chunk %s", typ)
		}
		types = append(types, typ)
		bodies[typ] = body
		pos += 12 + n
	}
	return
}

func dirtyPNG(t *testing.T, orientation uint16) (dirty, clean []byte) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, testImage()); err != nil {
		t.Fatal(err)
	}
	clean = buf.Bytes()
	ihdrEnd := 8 + 12 + 13
	extra := [][]byte{
		pngChunk("tEXt", []byte("Comment\x00SECRET-TEXT")),
		pngChunk("zTXt", []byte("Comment\x00\x00SECRET-ZTXT")),
		pngChunk("iTXt", []byte("XML:com.adobe.xmp\x00\x00\x00\x00\x00SECRET-XMP")),
		pngChunk("tIME", []byte{0x07, 0xE9, 1, 2, 3, 4, 5}),
		pngChunk("eXIf", buildExif(binary.LittleEndian, orientation)),
		pngChunk("prVW", []byte("SECRET-PRIVATE")),
		pngChunk("iCCP", []byte("icc\x00\x00FAKE-ICC")),
		pngChunk("gAMA", []byte{0, 1, 0x86, 0xA0}),
		pngChunk("pHYs", []byte{0, 0, 0x0B, 0x13, 0, 0, 0x0B, 0x13, 1}),
	}
	dirty = append([]byte{}, clean[:ihdrEnd]...)
	for _, c := range extra {
		dirty = append(dirty, c...)
	}
	dirty = append(dirty, clean[ihdrEnd:]...)
	return dirty, clean
}

func riffChunk(fourcc string, body []byte) []byte {
	b := make([]byte, 8, 8+len(body)+1)
	copy(b, fourcc)
	binary.LittleEndian.PutUint32(b[4:], uint32(len(body)))
	b = append(b, body...)
	if len(body)&1 == 1 {
		b = append(b, 0)
	}
	return b
}

func riff(chunks ...[]byte) []byte {
	b := []byte("RIFF\x00\x00\x00\x00WEBP")
	for _, c := range chunks {
		b = append(b, c...)
	}
	binary.LittleEndian.PutUint32(b[4:], uint32(len(b)-8))
	return b
}

func vp8x(flags byte) []byte {
	return riffChunk("VP8X", []byte{flags, 0, 0, 0, 23, 0, 0, 15, 0, 0})
}

func webpChunks(t *testing.T, data []byte) (names []string, bodies map[string][]byte) {
	t.Helper()
	if string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
		t.Fatal("not a webp")
	}
	if got := binary.LittleEndian.Uint32(data[4:]); int(got) != len(data)-8 {
		t.Fatalf("RIFF size = %d, want %d", got, len(data)-8)
	}
	bodies = map[string][]byte{}
	pos := 12
	for pos < len(data) {
		n := int(binary.LittleEndian.Uint32(data[pos+4:]))
		name := string(data[pos : pos+4])
		names = append(names, name)
		bodies[name] = data[pos+8 : pos+8+n]
		pos += 8 + n + n&1
	}
	if pos != len(data) {
		t.Fatalf("chunks overrun file: %d != %d", pos, len(data))
	}
	return
}

func assertNoSecrets(t *testing.T, data []byte) {
	t.Helper()
	for _, s := range []string{"SECRET", "Canon", "xmpmeta", "Photoshop", "MPF"} {
		if bytes.Contains(data, []byte(s)) {
			t.Errorf("output still contains %q", s)
		}
	}
}

// ---- JPEG ------------------------------------------------------------------

func TestJPEG_StripsMetadataKeepsICCAndOrientation(t *testing.T) {
	for _, bo := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		dirty := dirtyJPEG(t, bo, 6)
		out, f, err := Strip(dirty)
		if err != nil || f != FormatJPEG {
			t.Fatalf("Strip: f=%v err=%v", f, err)
		}
		assertNoSecrets(t, out)
		if !bytes.Contains(out, []byte(fakeICC)) {
			t.Error("ICC profile was dropped")
		}
		if !bytes.Contains(out, []byte("JFIF\x00")) || !bytes.Contains(out, []byte("Adobe")) {
			t.Error("JFIF/Adobe segment was dropped")
		}
		// The only EXIF left is the minimal one holding orientation.
		i := bytes.Index(out, []byte("Exif\x00\x00"))
		if i < 0 {
			t.Fatal("orientation EXIF missing")
		}
		tiff := out[i+6 : i+6+26]
		if got := exifOrientation(tiff); got != 6 {
			t.Errorf("orientation = %d, want 6", got)
		}
		if binary.BigEndian.Uint16(tiff[8:]) != 1 {
			t.Error("minimal EXIF should have exactly one entry")
		}
		if bytes.Count(out, []byte("Exif\x00\x00")) != 1 {
			t.Error("expected exactly one EXIF segment")
		}
		if out[2] != 0xFF || out[3] != 0xE0 {
			t.Error("JFIF APP0 should stay first")
		}
		if len(out) >= len(dirty) {
			t.Error("output not smaller than input")
		}

		// Idempotent.
		again, _, err := Strip(out)
		if err != nil || !bytes.Equal(again, out) {
			t.Errorf("second strip changed output (err=%v)", err)
		}
	}
}

func TestJPEG_PixelsUntouched(t *testing.T) {
	base := baseJPEG(t)
	dirty := dirtyJPEG(t, binary.LittleEndian, 3)

	// A clean file (no metadata) must be byte-identical after stripping.
	clean, _, err := Strip(base)
	if err != nil || !bytes.Equal(clean, base) {
		t.Fatalf("clean JPEG changed (err=%v)", err)
	}

	out, _, err := Strip(dirty)
	if err != nil {
		t.Fatal(err)
	}
	want, err := jpeg.Decode(bytes.NewReader(base))
	if err != nil {
		t.Fatal(err)
	}
	got, err := jpeg.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("stripped JPEG does not decode: %v", err)
	}
	if !bytes.Equal(want.(*image.YCbCr).Y, got.(*image.YCbCr).Y) ||
		!bytes.Equal(want.(*image.YCbCr).Cb, got.(*image.YCbCr).Cb) ||
		!bytes.Equal(want.(*image.YCbCr).Cr, got.(*image.YCbCr).Cr) {
		t.Error("pixel data differs")
	}
	// Everything from the first DQT on (tables, frame, scan) is byte-identical.
	dqt := []byte{0xFF, 0xDB}
	if !bytes.Equal(base[bytes.Index(base, dqt):], out[bytes.Index(out, dqt):]) {
		t.Error("image segments differ from original")
	}
}

func TestJPEG_OrientationDefaultNotWritten(t *testing.T) {
	out, _, err := Strip(dirtyJPEG(t, binary.LittleEndian, 1))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(out, []byte("Exif")) {
		t.Error("EXIF should be dropped entirely for orientation 1")
	}
}

func TestJPEG_TrailerAndLateMetadataDropped(t *testing.T) {
	base := baseJPEG(t)
	d := append([]byte{}, base...)
	d = append(d, "SECRET-TRAILER"...)
	out, _, err := Strip(d)
	if err != nil || !bytes.Equal(out, base) {
		t.Fatalf("trailer not removed (err=%v)", err)
	}
	// Metadata between scan and EOI.
	late := append([]byte{}, base[:len(base)-2]...)
	late = append(late, jpegSeg(0xFE, []byte("SECRET-LATE"))...)
	late = append(late, 0xFF, 0xD9)
	out, _, err = Strip(late)
	if err != nil || !bytes.Equal(out, base) {
		t.Fatalf("late COM not removed (err=%v)", err)
	}
}

// ---- PNG -------------------------------------------------------------------

func TestPNG_StripsMetadataKeepsRenderingChunks(t *testing.T) {
	dirty, clean := dirtyPNG(t, 8)
	out, f, err := Strip(dirty)
	if err != nil || f != FormatPNG {
		t.Fatalf("Strip: f=%v err=%v", f, err)
	}
	assertNoSecrets(t, out)
	types, bodies := pngChunks(t, out)
	got := strings.Join(types, ",")
	if !strings.HasPrefix(got, "IHDR,eXIf,iCCP,gAMA,pHYs,IDAT") || !strings.HasSuffix(got, "IEND") {
		t.Errorf("chunks = %s", got)
	}
	if string(bodies["iCCP"]) != "icc\x00\x00FAKE-ICC" {
		t.Error("iCCP changed")
	}
	if o := exifOrientation(bodies["eXIf"]); o != 8 || len(bodies["eXIf"]) != 26 {
		t.Errorf("eXIf orientation = %d len=%d", o, len(bodies["eXIf"]))
	}

	want, _ := png.Decode(bytes.NewReader(clean))
	have, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("stripped PNG does not decode: %v", err)
	}
	if !bytes.Equal(want.(*image.RGBA).Pix, have.(*image.RGBA).Pix) {
		t.Error("pixels differ")
	}

	again, _, _ := Strip(out)
	if !bytes.Equal(again, out) {
		t.Error("not idempotent")
	}
}

func TestPNG_NoOrientationDropsExifAndTrailer(t *testing.T) {
	dirty, clean := dirtyPNG(t, 1)
	out, _, err := Strip(append(dirty, "SECRET-TRAILER"...))
	if err != nil {
		t.Fatal(err)
	}
	types, _ := pngChunks(t, out)
	for _, ty := range types {
		if ty == "eXIf" {
			t.Error("eXIf kept for default orientation")
		}
	}
	// Cleaning a PNG without any metadata is an identity.
	same, _, err := Strip(clean)
	if err != nil || !bytes.Equal(same, clean) {
		t.Errorf("clean PNG changed (err=%v)", err)
	}
}

// ---- WebP ------------------------------------------------------------------

func TestWebP_StripsMetadataFixesFlagsAndSize(t *testing.T) {
	image := []byte("fake-lossless-bitstream-odd") // 27 bytes: exercises padding
	icc := []byte("FAKE-ICC-PROFILE")
	dirty := riff(
		vp8x(0x20|0x10|0x08|0x04),
		riffChunk("ICCP", icc),
		riffChunk("VP8L", image),
		riffChunk("EXIF", append([]byte("Exif\x00\x00"), buildExif(binary.LittleEndian, 6)...)),
		riffChunk("XMP ", []byte("<x:xmpmeta>SECRET-XMP</x:xmpmeta>")),
		riffChunk("JUNK", []byte("SECRET-JUNK")),
	)
	out, f, err := Strip(append(dirty, "SECRET-TRAILER"...))
	if err != nil || f != FormatWebP {
		t.Fatalf("Strip: f=%v err=%v", f, err)
	}
	assertNoSecrets(t, out)
	names, bodies := webpChunks(t, out)
	if got := strings.Join(names, ","); got != "VP8X,ICCP,VP8L,EXIF" {
		t.Errorf("chunks = %s", got)
	}
	if flags := bodies["VP8X"][0]; flags != 0x20|0x10|0x08 {
		t.Errorf("VP8X flags = %#x, want ICC|alpha|EXIF", flags)
	}
	if !bytes.Equal(bodies["VP8L"], image) || !bytes.Equal(bodies["ICCP"], icc) {
		t.Error("image/ICC chunk changed")
	}
	if o := exifOrientation(bodies["EXIF"]); o != 6 {
		t.Errorf("orientation = %d", o)
	}
	again, _, _ := Strip(out)
	if !bytes.Equal(again, out) {
		t.Error("not idempotent")
	}
}

func TestWebP_NoOrientationClearsFlags(t *testing.T) {
	dirty := riff(
		vp8x(0x10|0x08|0x04),
		riffChunk("VP8L", []byte("bits")),
		riffChunk("EXIF", buildExif(binary.BigEndian, 1)),
		riffChunk("XMP ", []byte("SECRET")),
	)
	out, _, err := Strip(dirty)
	if err != nil {
		t.Fatal(err)
	}
	names, bodies := webpChunks(t, out)
	if strings.Join(names, ",") != "VP8X,VP8L" || bodies["VP8X"][0] != 0x10 {
		t.Errorf("chunks=%v flags=%#x", names, bodies["VP8X"][0])
	}
}

func TestWebP_SimpleFormatUnchanged(t *testing.T) {
	simple := riff(riffChunk("VP8 ", []byte("lossy-bitstream")))
	out, _, err := Strip(simple)
	if err != nil || !bytes.Equal(out, simple) {
		t.Fatalf("simple WebP changed (err=%v)", err)
	}
}

// ---- shared behaviour ------------------------------------------------------

func TestTruncatedInputFails(t *testing.T) {
	pngDirty, _ := dirtyPNG(t, 6)
	webp := riff(vp8x(0x10), riffChunk("VP8L", []byte("bits")), riffChunk("EXIF", buildExif(binary.BigEndian, 6)))
	fixtures := map[string][]byte{
		"jpeg": dirtyJPEG(t, binary.LittleEndian, 6),
		"png":  pngDirty,
		"webp": webp,
	}
	for name, full := range fixtures {
		if _, _, err := Strip(full); err != nil {
			t.Fatalf("%s: full fixture failed: %v", name, err)
		}
		// Every strict prefix must be rejected and must never panic.
		for n := sniffLen; n < len(full); n++ {
			_, _, err := Strip(full[:n])
			if !errors.Is(err, ErrInvalidImage) {
				t.Fatalf("%s: prefix %d: err = %v, want ErrInvalidImage", name, n, err)
			}
		}
	}
}

func TestCorruptStructureFails(t *testing.T) {
	cases := map[string][]byte{
		"jpeg garbage after SOI": append([]byte{0xFF, 0xD8, 0xFF, 0xDB}, bytes.Repeat([]byte{0x00}, 8)...),
		"jpeg bad length":        {0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x01, 0, 0, 0, 0},
		"jpeg no frame":          {0xFF, 0xD8, 0xFF, 0xD9, 0, 0, 0, 0},
		"png only signature":     append(append([]byte{}, pngSignature...), make([]byte, 4)...),
		"png no IHDR":            append(append([]byte{}, pngSignature...), pngChunk("IDAT", []byte("x"))...),
		"webp bad size":          append([]byte("RIFF\xff\xff\xff\x7fWEBP"), riffChunk("VP8 ", []byte("x"))...),
		"webp no image":          riff(vp8x(0)),
	}
	for name, data := range cases {
		if _, _, err := Strip(data); !errors.Is(err, ErrInvalidImage) {
			t.Errorf("%s: err = %v, want ErrInvalidImage", name, err)
		}
	}
}

func TestNonImagePassesThroughUntouched(t *testing.T) {
	inputs := map[string][]byte{
		"text":      []byte("hello world, not an image at all"),
		"empty":     {},
		"short":     {0xFF, 0xD8},
		"gif":       []byte("GIF89a\x01\x00\x01\x00\x00\x00\x00;SECRET"),
		"svg":       []byte(`<svg xmlns="http://www.w3.org/2000/svg"><metadata>SECRET</metadata></svg>`),
		"riff-wav":  []byte("RIFF\x04\x00\x00\x00WAVE"),
		"jpeg-like": []byte("\xFF\xD8\x00not-really"),
	}
	for name, in := range inputs {
		out, f, err := Strip(in)
		if err != nil || f != FormatUnknown || !bytes.Equal(out, in) {
			t.Errorf("%s: f=%v err=%v changed=%v", name, f, err, !bytes.Equal(out, in))
		}
		r, f, err := StripReader(bytes.NewReader(in))
		if err != nil || f != FormatUnknown {
			t.Errorf("%s: StripReader f=%v err=%v", name, f, err)
			continue
		}
		got, _ := io.ReadAll(r)
		if !bytes.Equal(got, in) {
			t.Errorf("%s: StripReader changed the bytes", name)
		}
	}
}

func TestDetectIgnoresEverythingButBytes(t *testing.T) {
	// A JPEG is detected as such regardless of what the caller calls it.
	if Detect(baseJPEG(t)) != FormatJPEG {
		t.Error("jpeg not detected")
	}
}

func TestStripReader(t *testing.T) {
	dirty := dirtyJPEG(t, binary.LittleEndian, 6)
	r, f, err := StripReader(bytes.NewReader(dirty))
	if err != nil || f != FormatJPEG {
		t.Fatalf("f=%v err=%v", f, err)
	}
	got, _ := io.ReadAll(r)
	want, _, _ := Strip(dirty)
	if !bytes.Equal(got, want) {
		t.Error("StripReader output differs from Strip")
	}

	_, f, err = StripReader(bytes.NewReader(dirty[:len(dirty)-5]))
	if !errors.Is(err, ErrInvalidImage) || f != FormatJPEG {
		t.Errorf("truncated: f=%v err=%v", f, err)
	}

	boom := errors.New("boom")
	_, _, err = StripReader(io.MultiReader(bytes.NewReader(dirty[:100]), errReader{boom}))
	if !errors.Is(err, boom) || errors.Is(err, ErrInvalidImage) {
		t.Errorf("read error not propagated as-is: %v", err)
	}
}

type errReader struct{ err error }

func (e errReader) Read([]byte) (int, error) { return 0, e.err }

func TestExifOrientationParsing(t *testing.T) {
	for _, bo := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
		for o := uint16(1); o <= 8; o++ {
			want := int(o)
			if o == 1 {
				want = 0
			}
			if got := exifOrientation(buildExif(bo, o)); got != want {
				t.Errorf("orientation %d: got %d, want %d", o, got, want)
			}
		}
		if got := exifOrientation(buildExif(bo, 9)); got != 0 {
			t.Errorf("out-of-range orientation: got %d", got)
		}
	}
	// Hostile inputs must not panic.
	good := buildExif(binary.LittleEndian, 6)
	for n := 0; n < len(good); n++ {
		exifOrientation(good[:n])
	}
	bad := append([]byte{}, good...)
	binary.LittleEndian.PutUint32(bad[4:], 0xFFFFFFF0)
	if exifOrientation(bad) != 0 {
		t.Error("huge IFD offset accepted")
	}
}

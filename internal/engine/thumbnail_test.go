package engine_test

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalThumbnailAllowsFileLargerThanFormerByteCap(t *testing.T) {
	t.Parallel()
	img := image.NewRGBA(image.Rect(0, 0, 32, 24))
	img.Set(1, 1, color.RGBA{R: 240, G: 10, B: 10, A: 255})
	var enc bytes.Buffer
	if err := jpeg.Encode(&enc, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	raw := make([]byte, enc.Len()+21<<20)
	copy(raw, enc.Bytes())
	path := filepath.Join(t.TempDir(), "padded.jpg")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	eng, _ := newTestEngine(t, nil)
	th, err := eng.LocalThumbnail(path)
	if err != nil {
		t.Fatalf("LocalThumbnail: %v", err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(th.Data))
	if err != nil || cfg.Width != 32 || cfg.Height != 24 {
		t.Fatalf("thumbnail = %dx%d (%v), want 32x24", cfg.Width, cfg.Height, err)
	}
}

func TestLocalThumbnailScalesOversizedJPEG(t *testing.T) {
	t.Parallel()
	const w, h = 2400, 1600
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		img.Set(x, 0, color.RGBA{R: uint8(x % 255), A: 255})
	}
	var enc bytes.Buffer
	if err := jpeg.Encode(&enc, img, &jpeg.Options{Quality: 50}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "wide.jpg")
	if err := os.WriteFile(path, enc.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	eng, _ := newTestEngine(t, nil)
	th, err := eng.LocalThumbnail(path)
	if err != nil {
		t.Fatalf("LocalThumbnail: %v", err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(th.Data))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Width != 320 || cfg.Height != 213 {
		t.Fatalf("thumbnail = %dx%d, want 320x213", cfg.Width, cfg.Height)
	}
	if th.Width != w || th.Height != h {
		t.Fatalf("source dims = %dx%d, want %dx%d", th.Width, th.Height, w, h)
	}
}

func TestLocalThumbnailUsesEmbeddedJPEG(t *testing.T) {
	t.Parallel()
	img := image.NewRGBA(image.Rect(0, 0, 40, 20))
	img.Set(0, 0, color.RGBA{G: 200, A: 255})
	var enc bytes.Buffer
	if err := jpeg.Encode(&enc, img, nil); err != nil {
		t.Fatal(err)
	}
	raw := append([]byte("heic-container-placeholder"), enc.Bytes()...)
	path := filepath.Join(t.TempDir(), "shot.heic")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	eng, _ := newTestEngine(t, nil)
	th, err := eng.LocalThumbnail(path)
	if err != nil {
		t.Fatalf("LocalThumbnail: %v", err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(th.Data))
	if err != nil || cfg.Width != 40 || cfg.Height != 20 {
		t.Fatalf("thumbnail = %dx%d (%v), want 40x20", cfg.Width, cfg.Height, err)
	}
}

func TestLocalThumbnailUsesTIFFPreview(t *testing.T) {
	t.Parallel()
	img := image.NewRGBA(image.Rect(0, 0, 48, 32))
	img.Set(2, 2, color.RGBA{B: 220, A: 255})
	var enc bytes.Buffer
	if err := jpeg.Encode(&enc, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "preview.dng")
	if err := os.WriteFile(path, tiffWithJPEG(enc.Bytes()), 0o644); err != nil {
		t.Fatal(err)
	}
	eng, _ := newTestEngine(t, nil)
	th, err := eng.LocalThumbnail(path)
	if err != nil {
		t.Fatalf("LocalThumbnail: %v", err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(th.Data))
	if err != nil || cfg.Width != 48 || cfg.Height != 32 {
		t.Fatalf("thumbnail = %dx%d (%v), want 48x32", cfg.Width, cfg.Height, err)
	}
}

func tiffWithJPEG(preview []byte) []byte {
	const ifd = 8
	jpegAt := ifd + 2 + 2*12 + 4
	var b bytes.Buffer
	b.Write([]byte{'I', 'I', '*', 0})
	_ = binary.Write(&b, binary.LittleEndian, uint32(ifd))
	_ = binary.Write(&b, binary.LittleEndian, uint16(2))
	writeLongTag(&b, 0x0201, uint32(jpegAt))
	writeLongTag(&b, 0x0202, uint32(len(preview)))
	_ = binary.Write(&b, binary.LittleEndian, uint32(0))
	b.Write(preview)
	return b.Bytes()
}

func writeLongTag(b *bytes.Buffer, tag uint16, value uint32) {
	_ = binary.Write(b, binary.LittleEndian, tag)
	_ = binary.Write(b, binary.LittleEndian, uint16(4))
	_ = binary.Write(b, binary.LittleEndian, uint32(1))
	_ = binary.Write(b, binary.LittleEndian, value)
}

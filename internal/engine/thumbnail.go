package engine

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	_ "image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"sync"

	"github.com/evanoberholster/imagemeta"
	jpegscaled "github.com/m8rge/go-scaled-jpeg"
	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

const (
	thumbnailMaxDimension = 320
	thumbnailCacheEntries = 128
	jpegThumbnailQuality  = 85
	// fullDecodePixels is the point where a JPEG is decoded at full
	// resolution. Larger files use 1/8-step IDCT scaling.
	fullDecodePixels = 2_000_000
	// maxDecodedPixels caps the bitmap a thumbnail decode may allocate.
	maxDecodedPixels = 24_000_000
	maxSourceEdge    = 65535
	// maxEmbeddedPreviewBytes caps a RAW or HEIC preview JPEG before it is scaled.
	maxEmbeddedPreviewBytes = 20 << 20
	embeddedScanBytes       = 8 << 20
	orientationProbeBytes   = 256 << 10
	maxEmbeddedCandidates   = 16
)

var errThumbnailUnavailable = errors.New("thumbnail unavailable")

type Thumbnail struct {
	Data   []byte
	Width  int
	Height int
}

type thumbnailKey struct {
	path  string
	size  int64
	mtime int64
}

type thumbnailCache struct {
	mu      sync.Mutex
	entries map[thumbnailKey]Thumbnail
}

func (c *thumbnailCache) get(k thumbnailKey) (Thumbnail, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	t, ok := c.entries[k]
	return t, ok
}

func (c *thumbnailCache) put(k thumbnailKey, t Thumbnail) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = make(map[thumbnailKey]Thumbnail)
	}
	if len(c.entries) >= thumbnailCacheEntries {
		for old := range c.entries {
			delete(c.entries, old)
			break
		}
	}
	c.entries[k] = t
}

func (e *Engine) LocalThumbnail(localPath string) (Thumbnail, error) {
	info, err := os.Stat(localPath)
	if err != nil {
		return Thumbnail{}, err
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 {
		return Thumbnail{}, errThumbnailUnavailable
	}
	key := thumbnailKey{path: localPath, size: info.Size(), mtime: info.ModTime().UnixNano()}
	if t, ok := e.thumbs.get(key); ok {
		return t, nil
	}
	t, err := renderThumbnail(localPath, thumbnailMaxDimension)
	if err != nil {
		return Thumbnail{}, err
	}
	e.thumbs.put(key, t)
	return t, nil
}

type imageKind int

const (
	kindOther imageKind = iota
	kindJPEG
	kindGeneric
	kindTIFF
	kindCR3
)

func renderThumbnail(localPath string, maxDim int) (Thumbnail, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return Thumbnail{}, err
	}
	defer f.Close()

	kind, err := sniffImage(f)
	if err != nil {
		return Thumbnail{}, err
	}
	switch kind {
	case kindJPEG:
		return renderJPEG(f, maxDim)
	case kindGeneric:
		return renderGeneric(f, maxDim)
	case kindTIFF:
		if thumb, err := renderPreview(f, maxDim, func(r io.ReadSeeker) ([]byte, error) {
			return imagemeta.PreviewTIFF(r, imagemeta.WithMaxPreviewLength(maxEmbeddedPreviewBytes))
		}); err == nil {
			return thumb, nil
		}
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return Thumbnail{}, err
		}
		return renderGeneric(f, maxDim)
	case kindCR3:
		return renderPreview(f, maxDim, func(r io.ReadSeeker) ([]byte, error) {
			raw, err := imagemeta.PreviewCR3(r)
			if err != nil || len(raw) == 0 || len(raw) > maxEmbeddedPreviewBytes {
				return nil, errThumbnailUnavailable
			}
			return raw, nil
		})
	default:
		return renderEmbeddedScan(f, maxDim)
	}
}

func sniffImage(f *os.File) (imageKind, error) {
	var head [32]byte
	n, err := io.ReadFull(f, head[:])
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return kindOther, err
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return kindOther, err
	}
	headN := head[:n]
	switch {
	case n >= 3 && headN[0] == 0xFF && headN[1] == 0xD8:
		return kindJPEG, nil
	case n >= 8 && bytes.Equal(headN[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}):
		return kindGeneric, nil
	case n >= 6 && (bytes.Equal(headN[:6], []byte("GIF87a")) || bytes.Equal(headN[:6], []byte("GIF89a"))):
		return kindGeneric, nil
	case n >= 12 && bytes.Equal(headN[:4], []byte("RIFF")) && bytes.Equal(headN[8:12], []byte("WEBP")):
		return kindGeneric, nil
	case n >= 2 && headN[0] == 'B' && headN[1] == 'M':
		return kindGeneric, nil
	case n >= 4 && (bytes.Equal(headN[:4], []byte{'I', 'I', '*', 0}) || bytes.Equal(headN[:4], []byte{'M', 'M', 0, '*'})):
		return kindTIFF, nil
	case n >= 12 && bytes.Equal(headN[4:8], []byte("ftyp")) && bytes.Contains(headN, []byte("crx ")):
		return kindCR3, nil
	default:
		return kindOther, nil
	}
}

func renderJPEG(f *os.File, maxDim int) (Thumbnail, error) {
	cfg, err := jpeg.DecodeConfig(f)
	if err != nil {
		return Thumbnail{}, errThumbnailUnavailable
	}
	orientation := readJPEGOrientation(f)
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return Thumbnail{}, err
	}
	img, err := decodeScaledJPEG(f, cfg, maxDim)
	if err != nil {
		return Thumbnail{}, errThumbnailUnavailable
	}
	return encodeThumb(img, cfg.Width, cfg.Height, orientation, maxDim)
}

func renderGeneric(f *os.File, maxDim int) (Thumbnail, error) {
	cfg, _, err := image.DecodeConfig(f)
	if err != nil || !fullPixelsOK(cfg.Width, cfg.Height) {
		return Thumbnail{}, errThumbnailUnavailable
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return Thumbnail{}, err
	}
	img, _, err := image.Decode(f)
	if err != nil {
		return Thumbnail{}, errThumbnailUnavailable
	}
	return encodeThumb(img, cfg.Width, cfg.Height, 1, maxDim)
}

func renderPreview(f *os.File, maxDim int, read func(io.ReadSeeker) ([]byte, error)) (Thumbnail, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return Thumbnail{}, err
	}
	raw, err := read(f)
	if err != nil || len(raw) == 0 {
		return Thumbnail{}, errThumbnailUnavailable
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return Thumbnail{}, errThumbnailUnavailable
	}
	img, err := decodeScaledJPEG(bytes.NewReader(raw), cfg, maxDim)
	if err != nil {
		return Thumbnail{}, errThumbnailUnavailable
	}
	return encodeThumb(img, cfg.Width, cfg.Height, jpegOrientation(raw), maxDim)
}

func renderEmbeddedScan(f *os.File, maxDim int) (Thumbnail, error) {
	info, err := f.Stat()
	if err != nil {
		return Thumbnail{}, err
	}
	window := embeddedScanBytes
	if info.Size() < int64(window) {
		window = int(info.Size())
	}
	buf := make([]byte, window)
	n, err := f.ReadAt(buf, 0)
	if err != nil && err != io.EOF {
		return Thumbnail{}, errThumbnailUnavailable
	}
	buf = buf[:n]
	attempts := 0
	for i := 0; i+3 < len(buf) && attempts < maxEmbeddedCandidates; i++ {
		if buf[i] != 0xFF || buf[i+1] != 0xD8 || buf[i+2] != 0xFF {
			continue
		}
		attempts++
		section := io.NewSectionReader(f, int64(i), maxEmbeddedPreviewBytes)
		cfg, err := jpeg.DecodeConfig(section)
		if err != nil || cfg.Width < 8 || cfg.Height < 8 {
			continue
		}
		if _, err := section.Seek(0, io.SeekStart); err != nil {
			continue
		}
		img, err := decodeScaledJPEG(section, cfg, maxDim)
		if err != nil {
			continue
		}
		probe := buf[i:]
		if len(probe) > orientationProbeBytes {
			probe = probe[:orientationProbeBytes]
		}
		return encodeThumb(img, cfg.Width, cfg.Height, jpegOrientation(probe), maxDim)
	}
	return Thumbnail{}, errThumbnailUnavailable
}

func decodeScaledJPEG(r io.ReadSeeker, cfg image.Config, maxDim int) (image.Image, error) {
	pixels := int64(cfg.Width) * int64(cfg.Height)
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > maxSourceEdge || cfg.Height > maxSourceEdge {
		return nil, errThumbnailUnavailable
	}
	if pixels <= fullDecodePixels {
		if _, err := r.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
		return jpeg.Decode(r)
	}
	scale := dctScale(cfg.Width, cfg.Height, maxDim)
	if !fitsDecode(cfg.Width, cfg.Height, scale) {
		return nil, errThumbnailUnavailable
	}
	if _, err := r.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	img, err := jpegscaled.Decode(r, jpegscaled.DecodeOptions{DCTSizeScaled: scale})
	if err == nil {
		return img, nil
	}
	if pixels > maxDecodedPixels {
		return nil, err
	}
	if _, seekErr := r.Seek(0, io.SeekStart); seekErr != nil {
		return nil, seekErr
	}
	return jpeg.Decode(r)
}

func dctScale(w, h, maxDim int) int {
	long := max(h, w)
	for n := 1; n <= 8; n++ {
		if int64(long)*int64(n)/8 >= int64(maxDim) {
			return n
		}
	}
	return 8
}

func fitsDecode(w, h, scale int) bool {
	if w <= 0 || h <= 0 || scale < 1 || scale > 8 {
		return false
	}
	sw := int64(w) * int64(scale) / 8
	sh := int64(h) * int64(scale) / 8
	if sw < 1 || sh < 1 {
		return false
	}
	return sw*sh <= maxDecodedPixels
}

func fullPixelsOK(w, h int) bool {
	if w <= 0 || h <= 0 || w > maxSourceEdge || h > maxSourceEdge {
		return false
	}
	return int64(w)*int64(h) <= maxDecodedPixels
}

func readJPEGOrientation(f *os.File) int {
	buf := make([]byte, orientationProbeBytes)
	n, _ := f.ReadAt(buf, 0)
	if n < 4 {
		return 1
	}
	return jpegOrientation(buf[:n])
}

func encodeThumb(img image.Image, srcW, srcH, orientation, maxDim int) (Thumbnail, error) {
	scaled := orient(downscale(img, maxDim), orientation)
	var buf bytes.Buffer
	var err error
	if o, ok := scaled.(interface{ Opaque() bool }); ok && !o.Opaque() {
		err = png.Encode(&buf, scaled)
	} else {
		err = jpeg.Encode(&buf, scaled, &jpeg.Options{Quality: jpegThumbnailQuality})
	}
	if err != nil {
		return Thumbnail{}, err
	}
	return Thumbnail{Data: buf.Bytes(), Width: srcW, Height: srcH}, nil
}

func downscale(src image.Image, maxDim int) *image.RGBA {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dstW, dstH := w, h
	if w > maxDim || h > maxDim {
		if w >= h {
			dstW, dstH = maxDim, max(1, h*maxDim/w)
		} else {
			dstW, dstH = max(1, w*maxDim/h), maxDim
		}
	}
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
	return dst
}

func orient(src *image.RGBA, orientation int) image.Image {
	if orientation < 2 || orientation > 8 {
		return src
	}
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	dw, dh := w, h
	if orientation >= 5 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := range h {
		for x := range w {
			var nx, ny int
			switch orientation {
			case 2:
				nx, ny = w-1-x, y
			case 3:
				nx, ny = w-1-x, h-1-y
			case 4:
				nx, ny = x, h-1-y
			case 5:
				nx, ny = y, x
			case 6:
				nx, ny = h-1-y, x
			case 7:
				nx, ny = h-1-y, w-1-x
			default:
				nx, ny = y, w-1-x
			}
			dst.SetRGBA(nx, ny, src.RGBAAt(x, y))
		}
	}
	return dst
}

func jpegOrientation(data []byte) int {
	if len(data) < 4 || data[0] != 0xFF || data[1] != 0xD8 {
		return 1
	}
	r := bytes.NewReader(data[2:])
	for {
		var marker [4]byte
		if _, err := io.ReadFull(r, marker[:]); err != nil || marker[0] != 0xFF {
			return 1
		}
		size := int(binary.BigEndian.Uint16(marker[2:])) - 2
		if size < 0 || marker[1] == 0xDA {
			return 1
		}
		seg := make([]byte, size)
		if _, err := io.ReadFull(r, seg); err != nil {
			return 1
		}
		if marker[1] == 0xE1 && bytes.HasPrefix(seg, []byte("Exif\x00\x00")) {
			return tiffOrientation(seg[6:])
		}
	}
}

func tiffOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	off := int(bo.Uint32(t[4:]))
	if off < 0 || off+2 > len(t) {
		return 1
	}
	n := int(bo.Uint16(t[off:]))
	for i := range n {
		e := off + 2 + i*12
		if e+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[e:]) == 0x0112 {
			return int(bo.Uint16(t[e+8:]))
		}
	}
	return 1
}

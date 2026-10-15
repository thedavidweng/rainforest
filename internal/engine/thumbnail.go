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

	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

const (
	thumbnailMaxDimension = 320
	maxThumbnailPixels    = 8192
	thumbnailCacheEntries = 128
	jpegThumbnailQuality  = 85
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
	if !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxThumbnailSourceBytes {
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

func renderThumbnail(localPath string, maxDim int) (Thumbnail, error) {
	raw, err := os.ReadFile(localPath)
	if err != nil {
		return Thumbnail{}, err
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return Thumbnail{}, errThumbnailUnavailable
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > maxThumbnailPixels || cfg.Height > maxThumbnailPixels {
		return Thumbnail{}, errThumbnailUnavailable
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return Thumbnail{}, errThumbnailUnavailable
	}
	scaled := orient(downscale(img, maxDim), jpegOrientation(raw))
	var buf bytes.Buffer
	if o, ok := scaled.(interface{ Opaque() bool }); ok && !o.Opaque() {
		err = png.Encode(&buf, scaled)
	} else {
		err = jpeg.Encode(&buf, scaled, &jpeg.Options{Quality: jpegThumbnailQuality})
	}
	if err != nil {
		return Thumbnail{}, err
	}
	return Thumbnail{Data: buf.Bytes(), Width: cfg.Width, Height: cfg.Height}, nil
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
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Src, nil)
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

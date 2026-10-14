package engine

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/thedavidweng/amazon-photos/pkg/aphotos"
	"github.com/thedavidweng/rainforest/internal/store"
)

const (
	maxThumbnailSourceBytes = 20 * 1024 * 1024
	maxThumbnailDimension   = 128
	maxPlanPreviewThumbs    = 32
)

var photoExtensions = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".webp": true,
	".heic": true,
	".heif": true,
	".avif": true,
	".bmp":  true,
	".tif":  true,
	".tiff": true,
	".dng":  true,
	".cr2":  true,
	".cr3":  true,
	".nef":  true,
	".arw":  true,
	".orf":  true,
	".rw2":  true,
	".raf":  true,
}

var videoExtensions = map[string]bool{
	".mp4":  true,
	".mov":  true,
	".m4v":  true,
	".avi":  true,
	".mkv":  true,
	".webm": true,
	".3gp":  true,
	".mts":  true,
	".m2ts": true,
}

func ClassifyMediaFile(name string) (store.MediaType, bool) {
	ext := strings.ToLower(filepath.Ext(strings.TrimSpace(name)))
	if photoExtensions[ext] {
		return store.MediaTypePhoto, true
	}
	if videoExtensions[ext] {
		return store.MediaTypeVideo, true
	}
	return "", false
}

type DestinationCapability struct {
	Region            aphotos.Region         `json:"region"`
	Evidence          aphotos.EvidenceStatus `json:"evidence"`
	RootOnly          bool                   `json:"rootOnly"`
	DestinationNodeID string                 `json:"destinationNodeId"`
	Message           string                 `json:"message"`
}

type PlanItem struct {
	LocalPath        string          `json:"localPath"`
	Name             string          `json:"name"`
	MediaType        store.MediaType `json:"mediaType"`
	Size             int64           `json:"size"`
	Width            int             `json:"width,omitempty"`
	Height           int             `json:"height,omitempty"`
	ThumbnailDataURL string          `json:"thumbnailDataUrl,omitempty"`
}

type UploadPlan struct {
	SourceSummary      string                 `json:"sourceSummary"`
	PhotoCount         int                    `json:"photoCount"`
	VideoCount         int                    `json:"videoCount"`
	TotalCount         int                    `json:"totalCount"`
	TotalBytes         int64                  `json:"totalBytes"`
	SkippedUnsupported int                    `json:"skippedUnsupported"`
	ConflictPolicy     aphotos.ConflictPolicy `json:"conflictPolicy"`
	Destination        DestinationCapability  `json:"destination"`
	Items              []PlanItem             `json:"items"`
}

type PlanOptions struct {
	DestinationNodeID string
	ConflictPolicy    aphotos.ConflictPolicy
	IncludeVideos     *bool
	IncludePreviews   bool
}

func (e *Engine) resolveDestinationCapability(ctx context.Context, destNodeID string) DestinationCapability {
	region := aphotos.RegionUS
	evidence := aphotos.EvidenceExperimental

	if c := e.Client(); c != nil {
		rc := c.RegionConfig()
		region = rc.Region
		evidence = rc.Evidence
	} else {
		if st := e.Store(); st != nil {
			if cfg, err := st.GetSettings(ctx); err == nil && strings.TrimSpace(cfg.Region) != "" {
				if rc, rErr := aphotos.ResolveRegion(aphotos.Region(strings.ToLower(strings.TrimSpace(cfg.Region)))); rErr == nil {
					region = rc.Region
					evidence = rc.Evidence
				}
			}
		}
	}

	cleanDest := strings.TrimSpace(destNodeID)
	rootOnly := evidence != aphotos.EvidenceVerified
	if rootOnly && cleanDest != "" {
		cleanDest = ""
	}

	msg := "Verified US region: supports root and folder/album destinations."
	if rootOnly {
		msg = fmt.Sprintf("Region %q has experimental protocol status: uploads target the account root node only.", region)
	}
	return DestinationCapability{
		Region:            region,
		Evidence:          evidence,
		RootOnly:          rootOnly,
		DestinationNodeID: cleanDest,
		Message:           msg,
	}
}

func (e *Engine) PlanUpload(ctx context.Context, paths []string, opts PlanOptions) (*UploadPlan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, &aphotos.ConfigError{
			Field:   "paths",
			Message: "at least one file or directory path is required",
		}
	}

	includeVideos := true
	if opts.IncludeVideos != nil {
		includeVideos = *opts.IncludeVideos
	}

	conflict := opts.ConflictPolicy
	if conflict == "" {
		if st := e.Store(); st != nil {
			if cfg, err := st.GetSettings(ctx); err == nil {
				conflict = cfg.DefaultConflictPolicy
			}
		}
		if conflict == "" {
			conflict = aphotos.ConflictSkip
		}
	}

	seen := make(map[string]struct{})
	var items []PlanItem
	var photoCount, videoCount, skipped int
	var totalBytes int64
	thumbsGenerated := 0

	for _, rawPath := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		cleaned := strings.TrimSpace(rawPath)
		if cleaned == "" {
			continue
		}
		absPath, err := filepath.Abs(cleaned)
		if err != nil {
			return nil, fmt.Errorf("resolve path %q: %w", cleaned, err)
		}
		info, err := os.Stat(absPath)
		if err != nil {
			return nil, fmt.Errorf("stat %q: %w", absPath, err)
		}

		if !info.IsDir() {
			if _, ok := seen[absPath]; ok {
				continue
			}
			seen[absPath] = struct{}{}
			mt, supported := ClassifyMediaFile(info.Name())
			if !supported || (!includeVideos && mt == store.MediaTypeVideo) {
				skipped++
				continue
			}
			it := buildPlanItem(absPath, info, mt, opts.IncludePreviews && thumbsGenerated < maxPlanPreviewThumbs)
			if it.ThumbnailDataURL != "" {
				thumbsGenerated++
			}
			items = append(items, it)
			totalBytes += info.Size()
			if mt == store.MediaTypePhoto {
				photoCount++
			} else {
				videoCount++
			}
			continue
		}

		walkErr := filepath.WalkDir(absPath, func(p string, d fs.DirEntry, wErr error) error {
			if wErr != nil {
				return wErr
			}
			if err := ctx.Err(); err != nil {
				return err
			}
			if d.IsDir() {
				if p != absPath && strings.HasPrefix(d.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasPrefix(d.Name(), ".") {
				return nil
			}
			if !d.Type().IsRegular() {
				return nil
			}
			if _, ok := seen[p]; ok {
				return nil
			}
			seen[p] = struct{}{}

			mt, supported := ClassifyMediaFile(d.Name())
			if !supported || (!includeVideos && mt == store.MediaTypeVideo) {
				skipped++
				return nil
			}
			fInfo, iErr := d.Info()
			if iErr != nil {
				return iErr
			}
			it := buildPlanItem(p, fInfo, mt, opts.IncludePreviews && thumbsGenerated < maxPlanPreviewThumbs)
			if it.ThumbnailDataURL != "" {
				thumbsGenerated++
			}
			items = append(items, it)
			totalBytes += fInfo.Size()
			if mt == store.MediaTypePhoto {
				photoCount++
			} else {
				videoCount++
			}
			return nil
		})
		if walkErr != nil {
			return nil, walkErr
		}
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].LocalPath < items[j].LocalPath
	})

	destCap := e.resolveDestinationCapability(ctx, opts.DestinationNodeID)
	summary := summarizePaths(paths, len(items))

	return &UploadPlan{
		SourceSummary:      summary,
		PhotoCount:         photoCount,
		VideoCount:         videoCount,
		TotalCount:         len(items),
		TotalBytes:         totalBytes,
		SkippedUnsupported: skipped,
		ConflictPolicy:     conflict,
		Destination:        destCap,
		Items:              items,
	}, nil
}

func summarizePaths(paths []string, itemCount int) string {
	if len(paths) == 1 {
		base := filepath.Base(strings.TrimSpace(paths[0]))
		if base != "" && base != "." {
			return fmt.Sprintf("%s (%d items)", base, itemCount)
		}
	}
	return fmt.Sprintf("%d selected paths (%d items)", len(paths), itemCount)
}

func buildPlanItem(absPath string, info os.FileInfo, mt store.MediaType, wantPreview bool) PlanItem {
	item := PlanItem{
		LocalPath: absPath,
		Name:      info.Name(),
		MediaType: mt,
		Size:      info.Size(),
	}
	if !wantPreview || mt != store.MediaTypePhoto || info.Size() <= 0 || info.Size() > maxThumbnailSourceBytes {
		return item
	}

	f, err := os.Open(absPath)
	if err != nil {
		return item
	}
	defer func() { _ = f.Close() }()

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return item
	}
	item.Width = cfg.Width
	item.Height = cfg.Height

	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 8192 || cfg.Height > 8192 {
		return item
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return item
	}
	img, _, err := image.Decode(f)
	if err != nil {
		return item
	}

	thumb := downscaleNearest(img, maxThumbnailDimension)
	var buf bytes.Buffer
	if err := png.Encode(&buf, thumb); err != nil {
		return item
	}
	item.ThumbnailDataURL = "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
	return item
}

func downscaleNearest(src image.Image, maxDim int) image.Image {
	b := src.Bounds()
	w := b.Dx()
	h := b.Dy()
	if w <= maxDim && h <= maxDim {
		return src
	}
	dstW := maxDim
	dstH := max(1, (h*maxDim)/w)
	if h > w {
		dstH = maxDim
		dstW = max(1, (w*maxDim)/h)
	}
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	for y := range dstH {
		srcY := b.Min.Y + (y*h)/dstH
		for x := range dstW {
			srcX := b.Min.X + (x*w)/dstW
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
}

func computeFileMD5(path string) (string, error) {
	infoBefore, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	h := md5.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", err
	}
	infoAfter, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if infoBefore.Size() != infoAfter.Size() || infoBefore.ModTime().UnixNano() != infoAfter.ModTime().UnixNano() || n != infoBefore.Size() {
		return "", errFileModifiedMidFlight
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

package view

import (
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/thedavidweng/rainforest/locales"
)

func writeJPEGs(t *testing.T, dir string, n int) {
	t.Helper()
	for i := range n {
		img := image.NewRGBA(image.Rect(0, 0, 48, 36))
		for y := range 36 {
			for x := range 48 {
				img.Set(x, y, color.RGBA{R: uint8(40 + i*30), G: uint8(x * 4), B: uint8(y * 6), A: 255})
			}
		}
		f, err := os.Create(filepath.Join(dir, "photo-"+strconv.Itoa(i)+".jpg"))
		if err != nil {
			t.Fatal(err)
		}
		if err := jpeg.Encode(f, img, nil); err != nil {
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestReviewPlansAndConfirmStartsUpload(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	dir := t.TempDir()
	writeJPEGs(t, dir, 3)

	h.app.openUploadReview([]string{dir})
	h.drain()
	h.wantText(locales.ReviewTitle())
	h.wantText(locales.ReviewPhotos())
	h.wantText(locales.ReviewConfirm(3))
	h.wantText(locales.DestLibrary())
	h.wantText(locales.DestReady())
	if h.app.pages.review.plan == nil || h.app.pages.review.plan.TotalCount != 3 {
		t.Fatalf("plan = %+v", h.app.pages.review.plan)
	}

	if err := h.tt.Click(locales.ReviewConfirm(3)); err != nil {
		t.Fatal(err)
	}
	h.drain()
	if h.app.pages.review.open {
		t.Error("dialog still open after confirm")
	}
	h.wantText(locales.ToastUploadStarted())
	if len(h.app.active)+len(h.app.history) != 1 {
		t.Fatalf("transfers: active=%d history=%d", len(h.app.active), len(h.app.history))
	}
}

func TestReviewConfirmDisabledWithoutItems(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	h.app.openUploadReview([]string{t.TempDir()})
	h.drain()
	h.wantText(locales.ReviewNoFiles())
	if h.app.pages.review.canConfirm() {
		t.Fatal("canConfirm with 0 items")
	}
	_ = h.tt.Click(locales.ReviewConfirm(0))
	h.drain()
	if len(h.app.active)+len(h.app.history) != 0 {
		t.Fatal("upload started with 0 items")
	}
}

func TestReviewReplansWhenOptionsChange(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	dir := t.TempDir()
	writeJPEGs(t, dir, 1)
	h.app.openUploadReview([]string{dir})
	h.drain()
	seq := h.app.pages.review.seq

	if err := h.tt.Click(locales.OptionIncludeVideos()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	if h.app.pages.review.includeVideos || h.app.pages.review.seq == seq {
		t.Fatalf("includeVideos=%v seq %d -> %d", h.app.pages.review.includeVideos, seq, h.app.pages.review.seq)
	}
	h.wantText("—")
}

func TestReviewCancelClosesDialog(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	dir := t.TempDir()
	writeJPEGs(t, dir, 1)
	h.app.openUploadReview([]string{dir})
	h.drain()
	if err := h.tt.Click(locales.CommonCancel()); err != nil {
		t.Fatal(err)
	}
	h.drain()
	if h.app.pages.review.open {
		t.Fatal("dialog still open")
	}
	h.noText(locales.ReviewTitle())
}

func TestUploadPageShowsRecentActivity(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	h.wantText(locales.UploadDropTitle())
	h.wantText(locales.UploadNothingYet())
	h.app.active = nil
	h.app.history = append(h.app.history, sampleTransfers()[3])
	h.drain()
	h.noText(locales.UploadNothingYet())
	h.wantText("Holiday")
}

func TestNewReviewRejectsPreviousPlan(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	first, second := t.TempDir(), t.TempDir()
	writeJPEGs(t, first, 1)
	writeJPEGs(t, second, 2)
	h.app.openUploadReview([]string{first})
	old := h.queue
	h.queue = nil
	h.app.openUploadReview([]string{second})
	h.drain()
	for _, fn := range old {
		fn()
	}
	h.drain()
	if got := h.app.pages.review.plan; got == nil || got.TotalCount != 2 {
		t.Fatalf("previous review replaced current plan (expected two files)")
	}
}

func TestOpeningReviewDefersDirectoryInspection(t *testing.T) {
	h := newHarness(t, true)
	h.start()
	dir := t.TempDir()
	h.app.openUploadReview([]string{dir})
	if h.app.pages.review.hasFolder {
		t.Fatal("directory inspection ran before background work")
	}
	h.drain()
	if !h.app.pages.review.hasFolder {
		t.Fatal("directory inspection result missing")
	}
}

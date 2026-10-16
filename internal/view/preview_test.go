package view

import (
	"testing"

	"github.com/thedavidweng/rainforest/locales"
)

func TestPreviewDisablesUploadAndDropsButKeepsNavigation(t *testing.T) {
	h := newHarness(t, true)
	h.app.preview = true
	h.start()
	for _, label := range []string{locales.UploadChooseFiles(), locales.UploadChooseFolder()} {
		if err := h.tt.Click(label); err != nil {
			t.Fatal(err)
		}
		h.drain()
		if h.app.pages.review.open {
			t.Fatal("preview opened upload review")
		}
	}
	h.app.openUploadReview([]string{"/visitor/photo.jpg"})
	if h.app.pages.review.open {
		t.Fatal("preview accepted dropped files")
	}
	for _, path := range []string{"/activity", "/backup", "/restore", "/settings"} {
		h.app.router.Push(path)
		h.drain()
		if h.app.router.Path() != path {
			t.Fatalf("route = %s", h.app.router.Path())
		}
	}
}

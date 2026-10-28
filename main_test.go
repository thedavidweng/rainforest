package main

import (
	"bytes"
	"context"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestTrayIconValidPNG(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(trayIconPNG))
	if err != nil {
		t.Fatalf("trayIconPNG is not a valid PNG: %v", err)
	}
	b := img.Bounds()
	if b.Dx() != 32 || b.Dy() != 32 {
		t.Fatalf("expected 32x32 tray icon, got %dx%d", b.Dx(), b.Dy())
	}
}

func TestNativeFilePickerEnvOverrides(t *testing.T) {
	picker := &nativeFilePicker{}
	ctx := context.Background()

	tmpDir := t.TempDir()
	f1 := filepath.Join(tmpDir, "photo1.jpg")
	f2 := filepath.Join(tmpDir, "video2.mp4")
	backupDir := filepath.Join(tmpDir, "backup-dir")
	joined := f1 + string(os.PathListSeparator) + f2

	t.Setenv("RAINFOREST_PICK_FILES", joined)
	t.Setenv("RAINFOREST_PICK_DIR", backupDir)
	files, err := picker.PickFiles(ctx)
	if err != nil {
		t.Fatalf("PickFiles error: %v", err)
	}
	if len(files) != 2 || files[0] != f1 || files[1] != f2 {
		t.Fatalf("unexpected files: %#v", files)
	}

	cookieFile, err := picker.PickCookieFile(ctx)
	if err != nil {
		t.Fatalf("PickCookieFile error: %v", err)
	}
	if cookieFile != f1 {
		t.Fatalf("expected %q, got %q", f1, cookieFile)
	}

	dir, err := picker.PickDirectory(ctx, "Pick Folder")
	if err != nil {
		t.Fatalf("PickDirectory error: %v", err)
	}
	if dir != backupDir {
		t.Fatalf("expected %q, got %q", backupDir, dir)
	}
}

func TestResolveDesktopDataDirRejectsTempFallback(t *testing.T) {
	_, err := resolveDesktopDataDir(filepath.Join(os.TempDir(), "rainforest"))
	if err == nil {
		t.Fatal("expected resolveDesktopDataDir to reject os.TempDir() fallback")
	}
}

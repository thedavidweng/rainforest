package main

import (
	"bytes"
	"context"
	"image/png"
	"os"
	"path/filepath"
	"strings"
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
	picker := newFilePicker()
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

func TestDocumentationAndReleaseWorkflowsContract(t *testing.T) {
	requiredFiles := []string{
		"README.md",
		"CHANGELOG.md",
		"CONTRIBUTING.md",
		"SECURITY.md",
		"CONTEXT.md",
		"GLOSSARY.md",
		"AGENTS.md",
		"LICENSE",
		"docs/adr/0001-rainforest-architecture.md",
		"docs/adr/0002-mygo-desktop-shell.md",
		".github/workflows/ci.yml",
		".github/workflows/release.yml",
	}

	contents := make(map[string]string, len(requiredFiles))
	for _, relPath := range requiredFiles {
		raw, err := os.ReadFile(relPath)
		if err != nil {
			t.Fatalf("required repository file %s missing or unreadable: %v", relPath, err)
		}
		text := string(raw)
		if strings.TrimSpace(text) == "" {
			t.Fatalf("required repository file %s is empty", relPath)
		}
		contents[relPath] = text
	}

	readme := contents["README.md"]
	for _, snippet := range []string{
		"Not affiliated with, endorsed by, or sponsored by Amazon.com, Inc.",
		"one-time upload",
		"one-way",
		"transfer queue",
		"recovery download",
		"internal/credstore",
		"#343",
		"#161",
		"verified",
		"experimental",
		"github.com/thedavidweng/amazon-photos",
		"internal/store",
		"internal/engine",
		"internal/gui",
		"mise run check",
		"mygo build",
		"AGPL-3.0-only",
		"thedavidweng/tg-drive",
		"egoist/mygo",
		"trevorhobenshield/amazon_photos",
		"kpfitzgerald/amazon-photos-mcp",
		"vanessa/immich-amazon-sync",
	} {
		if !strings.Contains(readme, snippet) {
			t.Errorf("README.md missing required snippet %q", snippet)
		}
	}

	changelog := contents["CHANGELOG.md"]
	for _, snippet := range []string{
		"0.1.0",
		"Rainforest",
		"MyGo",
		"ReconcileOnStartup",
	} {
		if !strings.Contains(changelog, snippet) {
			t.Errorf("CHANGELOG.md missing required snippet %q", snippet)
		}
	}

	contributing := contents["CONTRIBUTING.md"]
	for _, snippet := range []string{
		"mise run check",
		"AGPL-3.0-only",
		"internal/gui",
		"MYGO_GENERATE",
	} {
		if !strings.Contains(contributing, snippet) {
			t.Errorf("CONTRIBUTING.md missing required snippet %q", snippet)
		}
	}

	security := contents["SECURITY.md"]
	for _, snippet := range []string{
		"aphotos.Redact",
		"0600",
		"icacls",
		"#343",
		"session-id",
	} {
		if !strings.Contains(security, snippet) {
			t.Errorf("SECURITY.md missing required snippet %q", snippet)
		}
	}

	ci := contents[".github/workflows/ci.yml"]
	for _, snippet := range []string{
		"mise run check",
		"ubuntu-latest",
		"macos-latest",
		"windows-latest",
		"go test ./...",
		"bun --cwd frontend test",
	} {
		if !strings.Contains(ci, snippet) {
			t.Errorf(".github/workflows/ci.yml missing required snippet %q", snippet)
		}
	}

	release := contents[".github/workflows/release.yml"]
	for _, snippet := range []string{
		"v*",
		"mygo build",
		"sha256",
		"darwin",
		"linux",
		"windows",
	} {
		if !strings.Contains(release, snippet) {
			t.Errorf(".github/workflows/release.yml missing required snippet %q", snippet)
		}
	}
}

package autostart

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const AppBundleID = "com.thedavidweng.rainforest"

type Manager struct {
	OS         string
	BaseDir    string
	Executable string
}

func NewDefaultManager() *Manager {
	execPath, err := os.Executable()
	if err != nil || strings.TrimSpace(execPath) == "" {
		execPath = "rainforest"
	}
	return &Manager{
		OS:         runtime.GOOS,
		Executable: execPath,
	}
}

func (m *Manager) SetEnabled(enabled bool) error {
	targetPath, content, err := m.entrySpec()
	if err != nil {
		return err
	}
	if !enabled {
		if rmErr := os.Remove(targetPath); rmErr != nil && !os.IsNotExist(rmErr) {
			return fmt.Errorf("remove launch-at-login entry: %w", rmErr)
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create launch-at-login directory: %w", err)
	}
	if err := os.WriteFile(targetPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write launch-at-login entry: %w", err)
	}
	return nil
}

func (m *Manager) EntryPath() (string, error) {
	p, _, err := m.entrySpec()
	return p, err
}

func (m *Manager) entrySpec() (path, content string, err error) {
	goos := strings.TrimSpace(m.OS)
	if goos == "" {
		goos = runtime.GOOS
	}
	execPath := strings.TrimSpace(m.Executable)
	if execPath == "" {
		execPath = "rainforest"
	}
	baseDir := strings.TrimSpace(m.BaseDir)
	if baseDir == "" {
		home, hErr := os.UserHomeDir()
		if hErr != nil || strings.TrimSpace(home) == "" {
			return "", "", fmt.Errorf("resolve user home directory: %w", hErr)
		}
		baseDir = home
	}

	switch goos {
	case "darwin":
		target := filepath.Join(baseDir, "Library", "LaunchAgents", AppBundleID+".plist")
		body := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`, AppBundleID, xmlEscape(execPath))
		return target, body, nil

	case "windows":
		appData := os.Getenv("APPDATA")
		if strings.TrimSpace(m.BaseDir) != "" || strings.TrimSpace(appData) == "" {
			appData = filepath.Join(baseDir, "AppData", "Roaming")
		}
		target := filepath.Join(appData, "Microsoft", "Windows", "Start Menu", "Programs", "Startup", "Rainforest.cmd")
		body := fmt.Sprintf("@echo off\r\nstart \"\" %q\r\n", execPath)
		return target, body, nil

	default:
		target := filepath.Join(baseDir, ".config", "autostart", "rainforest.desktop")
		body := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Rainforest
Comment=Amazon Photos Transfer & Backup
Exec="%s"
Terminal=false
X-GNOME-Autostart-enabled=true
`, execPath)
		return target, body, nil
	}
}

func xmlEscape(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return r.Replace(s)
}

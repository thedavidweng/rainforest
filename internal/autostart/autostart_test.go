package autostart_test

import (
	"os"
	"strings"
	"testing"

	"github.com/thedavidweng/rainforest/internal/autostart"
)

func TestAutostartManagerCrossPlatformEntries(t *testing.T) {
	t.Parallel()

	cases := []struct {
		osName     string
		wantSuffix string
		wantToken  string
	}{
		{
			osName:     "darwin",
			wantSuffix: "Library/LaunchAgents/com.thedavidweng.rainforest.plist",
			wantToken:  "<key>RunAtLoad</key>",
		},
		{
			osName:     "linux",
			wantSuffix: ".config/autostart/rainforest.desktop",
			wantToken:  "X-GNOME-Autostart-enabled=true",
		},
		{
			osName:     "windows",
			wantSuffix: "Start Menu/Programs/Startup/Rainforest.cmd",
			wantToken:  "start \"\"",
		},
	}

	for _, tc := range cases {
		t.Run(tc.osName, func(t *testing.T) {
			t.Parallel()
			base := t.TempDir()
			mgr := &autostart.Manager{
				OS:         tc.osName,
				BaseDir:    base,
				Executable: "/opt/rainforest/bin/rainforest",
			}

			if err := mgr.SetEnabled(true); err != nil {
				t.Fatalf("SetEnabled(true): %v", err)
			}
			entryPath, err := mgr.EntryPath()
			if err != nil {
				t.Fatalf("EntryPath: %v", err)
			}
			normPath := strings.ReplaceAll(entryPath, `\`, "/")
			if !strings.HasSuffix(normPath, tc.wantSuffix) {
				t.Errorf("EntryPath = %q, want suffix %q", normPath, tc.wantSuffix)
			}

			raw, err := os.ReadFile(entryPath)
			if err != nil {
				t.Fatalf("ReadFile(%s): %v", entryPath, err)
			}
			if !strings.Contains(string(raw), tc.wantToken) || !strings.Contains(string(raw), "/opt/rainforest/bin/rainforest") {
				t.Errorf("unexpected entry content for %s:\n%s", tc.osName, string(raw))
			}

			if err := mgr.SetEnabled(false); err != nil {
				t.Fatalf("SetEnabled(false): %v", err)
			}
			if _, statErr := os.Stat(entryPath); !os.IsNotExist(statErr) {
				t.Errorf("expected entry file to be removed when disabled, got err=%v", statErr)
			}
		})
	}
}

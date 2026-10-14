package credstore_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/thedavidweng/amazon-photos/pkg/aphotos"
	"github.com/thedavidweng/rainforest/internal/credstore"
)

func TestStoreImportFormatsAndNormalization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		raw          string
		region       string
		wantRegion   aphotos.Region
		wantCookies  map[string]string
		wantEvidence aphotos.EvidenceStatus
	}{
		{
			name: "Netscape cookies.txt with HttpOnly and underscore alias",
			raw: `# Netscape HTTP Cookie File
.amazon.com	TRUE	/	TRUE	1893456000	session_id	140-1111111-2222222
#HttpOnly_.amazon.com	TRUE	/	TRUE	1893456000	ubid_main	130-3333333-4444444
.amazon.com	TRUE	/	TRUE	1893456000	at_main	Atza|IwEBIGExampleUS
`,
			region:     "",
			wantRegion: aphotos.RegionUS,
			wantCookies: map[string]string{
				"session-id": "140-1111111-2222222",
				"ubid-main":  "130-3333333-4444444",
				"at-main":    "Atza|IwEBIGExampleUS",
			},
			wantEvidence: aphotos.EvidenceVerified,
		},
		{
			name: "JSON array of browser cookies for Canada",
			raw: `[
				{"name": "session-id", "value": "140-ca-111", "domain": ".amazon.ca"},
				{"name": "ubid-acbca", "value": "130-ca-222", "domain": ".amazon.ca"},
				{"name": "at_acbca", "value": "Atza|IwEBICanada", "domain": ".amazon.ca"}
			]`,
			region:     "",
			wantRegion: aphotos.RegionCA,
			wantCookies: map[string]string{
				"session-id": "140-ca-111",
				"ubid-acbca": "130-ca-222",
				"at-acbca":   "Atza|IwEBICanada",
			},
			wantEvidence: aphotos.EvidenceExperimental,
		},
		{
			name: "JSON map for UK with nested cookies object",
			raw: `{
				"cookies": {
					"session_id": "140-uk-111",
					"ubid_acbuk": "130-uk-222",
					"at-acbuk": "Atza|IwEBIUK"
				}
			}`,
			region:     "uk",
			wantRegion: aphotos.RegionUK,
			wantCookies: map[string]string{
				"session-id": "140-uk-111",
				"ubid-acbuk": "130-uk-222",
				"at-acbuk":   "Atza|IwEBIUK",
			},
			wantEvidence: aphotos.EvidenceExperimental,
		},
		{
			name:       "Raw Cookie header string for JP",
			raw:        `Cookie: session-id=140-jp-111; ubid-acbjp=130-jp-222; at-acbjp="Atza|IwEBIJP"`,
			region:     "jp",
			wantRegion: aphotos.RegionJP,
			wantCookies: map[string]string{
				"session-id": "140-jp-111",
				"ubid-acbjp": "130-jp-222",
				"at-acbjp":   "Atza|IwEBIJP",
			},
			wantEvidence: aphotos.EvidenceExperimental,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			vaultPath := filepath.Join(t.TempDir(), "vault", "credentials.json")
			cs := credstore.NewFileStore(vaultPath)

			creds, err := cs.ImportCookies(tc.raw, tc.region, credstore.HostOverrides{})
			if err != nil {
				t.Fatalf("ImportCookies returned unexpected error: %v", err)
			}
			if creds.Region != tc.wantRegion {
				t.Fatalf("Region = %q, want %q", creds.Region, tc.wantRegion)
			}
			for k, wantVal := range tc.wantCookies {
				if got := creds.Cookies[k]; got != wantVal {
					t.Errorf("cookie[%q] = %q, want %q", k, got, wantVal)
				}
			}

			loaded, err := cs.Load()
			if err != nil {
				t.Fatalf("Load returned error: %v", err)
			}
			if loaded.Region != tc.wantRegion {
				t.Errorf("loaded.Region = %q, want %q", loaded.Region, tc.wantRegion)
			}

			rc, err := aphotos.ResolveRegion(loaded.Region)
			if err != nil {
				t.Fatalf("ResolveRegion(%q) error: %v", loaded.Region, err)
			}
			if rc.Evidence != tc.wantEvidence {
				t.Errorf("Evidence = %q, want %q", rc.Evidence, tc.wantEvidence)
			}

			if runtime.GOOS != "windows" {
				info, statErr := os.Stat(vaultPath)
				if statErr != nil {
					t.Fatalf("Stat(%s): %v", vaultPath, statErr)
				}
				if perm := info.Mode().Perm(); perm != 0o600 {
					t.Errorf("vault file perm = %04o, want 0600", perm)
				}
			}
			ok, msg, _ := cs.CheckPermissions()
			if !ok {
				t.Errorf("CheckPermissions failed: %s", msg)
			}
		})
	}
}

func TestStoreNeverSilentlyFallsBackToDotCom(t *testing.T) {
	t.Parallel()
	vaultPath := filepath.Join(t.TempDir(), "credentials.json")
	cs := credstore.NewFileStore(vaultPath)

	// Missing regional ubid/at cookies and no explicit region -> must fail, never default to US .com
	_, err := cs.ImportCookies("session-id=140-only-session", "", credstore.HostOverrides{})
	if err == nil {
		t.Fatal("expected error when region cannot be inferred, got nil")
	}

	// Conflicting regional cookies (US + UK) with no explicit region -> must fail
	_, err = cs.ImportCookies("session-id=140-mix; ubid-main=130-us; at-main=AtzaUS; ubid-acbuk=130-uk; at-acbuk=AtzaUK", "", credstore.HostOverrides{})
	if err == nil {
		t.Fatal("expected error on ambiguous multi-region cookies, got nil")
	}

	// Unsupported region -> must fail
	_, err = cs.ImportCookies("session-id=140-xx; ubid-main=130-us; at-main=AtzaUS", "mars", credstore.HostOverrides{})
	if err == nil {
		t.Fatal("expected error on unsupported region, got nil")
	}
}

func TestStoreImportFileClearAndRedaction(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	vaultPath := filepath.Join(dir, "credentials.json")
	cookieFile := filepath.Join(dir, "cookies.txt")

	secretAt := "Atza|SecretTokenValue999"
	content := "session-id=140-secret-session; ubid-main=130-secret-ubid; at-main=" + secretAt
	if err := os.WriteFile(cookieFile, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cs := credstore.NewFileStore(vaultPath)
	_, err := cs.ImportCookieFile(cookieFile, "us", credstore.HostOverrides{
		DriveURL: "https://www.amazon.com/drive/v1",
	})
	if err != nil {
		t.Fatalf("ImportCookieFile: %v", err)
	}

	// Missing required cookie error must redact any secret values if present in error messages
	_, badErr := cs.ImportCookies("session-id=140-secret-session; at-main="+secretAt, "us", credstore.HostOverrides{})
	if badErr == nil {
		t.Fatal("expected error when ubid-main is missing")
	}
	if strings.Contains(badErr.Error(), secretAt) {
		t.Fatalf("error message leaked secret cookie: %v", badErr)
	}

	if err := cs.Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	_, loadErr := cs.Load()
	if !errors.Is(loadErr, credstore.ErrNotConfigured) {
		t.Fatalf("expected ErrNotConfigured after Clear, got %v", loadErr)
	}
}

func TestResolveDataDirRefusesTempDirAndMissingConfig(t *testing.T) {
	t.Parallel()

	validBase := t.TempDir()
	// Note: t.TempDir() is inside os.TempDir(), so ResolveDataDir must reject paths inside os.TempDir()!
	_, err := credstore.ResolveDataDir(filepath.Join(os.TempDir(), "rainforest"), nil, nil)
	if err == nil || !strings.Contains(err.Error(), "refusing temporary directory") {
		t.Fatalf("expected ResolveDataDir to reject os.TempDir(), got %v", err)
	}

	_, err = credstore.ResolveDataDir(filepath.Join(validBase, "sub"), nil, nil)
	if err == nil || !strings.Contains(err.Error(), "refusing temporary directory") {
		t.Fatalf("expected ResolveDataDir to reject subdirectory of os.TempDir(), got %v", err)
	}

	_, err = credstore.ResolveDataDir("", func() (string, error) {
		return "", errors.New("no config dir")
	}, func() (string, error) {
		return "", errors.New("no home dir")
	})
	if err == nil || !strings.Contains(err.Error(), "cannot resolve persistent user data directory") {
		t.Fatalf("expected error when both configDir and homeDir fail, got %v", err)
	}

	_, err = credstore.ResolveDataDir("relative/path", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "refusing non-absolute") {
		t.Fatalf("expected error on relative path, got %v", err)
	}
}

package credstore

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/thedavidweng/amazon-photos/pkg/aphotos"
)

var ErrNotConfigured = errors.New("credentials not configured")

type HostOverrides struct {
	DriveURL     string `json:"driveUrl,omitempty"`
	UploadURL    string `json:"uploadUrl,omitempty"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
}

type Credentials struct {
	Region       aphotos.Region    `json:"region"`
	Cookies      map[string]string `json:"cookies"`
	DriveURL     string            `json:"driveUrl,omitempty"`
	UploadURL    string            `json:"uploadUrl,omitempty"`
	ThumbnailURL string            `json:"thumbnailUrl,omitempty"`
}

type Store interface {
	Load() (Credentials, error)
	Save(creds Credentials) error
	ImportCookies(rawInput, explicitRegion string, overrides HostOverrides) (Credentials, error)
	ImportCookieFile(path, explicitRegion string, overrides HostOverrides) (Credentials, error)
	Clear() error
	CheckPermissions() (ok bool, msg, hint string)
	Path() string
}

type FileStore struct {
	mu   sync.Mutex
	path string
}

func NewFileStore(path string) *FileStore {
	cleaned := strings.TrimSpace(path)
	if cleaned == "" {
		cleaned = DefaultVaultPath()
	}
	if cleaned != "" {
		cleaned = filepath.Clean(cleaned)
	}
	return &FileStore{path: cleaned}
}

func ResolveDataDir(primary string, configDirFn, homeDirFn func() (string, error)) (string, error) {
	var raw string
	if strings.TrimSpace(primary) != "" {
		raw = strings.TrimSpace(primary)
	} else {
		if configDirFn == nil {
			configDirFn = os.UserConfigDir
		}
		if homeDirFn == nil {
			homeDirFn = os.UserHomeDir
		}
		if cfgDir, err := configDirFn(); err == nil && strings.TrimSpace(cfgDir) != "" {
			raw = filepath.Join(strings.TrimSpace(cfgDir), "rainforest")
		} else if homeDir, hErr := homeDirFn(); hErr == nil && strings.TrimSpace(homeDir) != "" {
			raw = filepath.Join(strings.TrimSpace(homeDir), ".config", "rainforest")
		}
	}
	if raw == "" {
		return "", errors.New("cannot resolve persistent user data directory: neither user config directory nor home directory is available")
	}
	cleaned := filepath.Clean(raw)
	if !filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("refusing non-absolute user data directory %q", cleaned)
	}
	tmpClean := filepath.Clean(os.TempDir())
	if tmpClean != "" && tmpClean != "." && tmpClean != string(filepath.Separator) {
		rel, rErr := filepath.Rel(tmpClean, cleaned)
		if rErr == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))) {
			return "", fmt.Errorf("refusing temporary directory %q for persistent credentials and state", cleaned)
		}
	}
	if err := os.MkdirAll(cleaned, 0o700); err != nil {
		return "", fmt.Errorf("create persistent user data directory: %w", err)
	}
	if err := enforceDirPermissions(cleaned); err != nil {
		return "", fmt.Errorf("secure persistent user data directory: %w", err)
	}
	return cleaned, nil
}

func DefaultDataDir() string {
	if dir, err := ResolveDataDir("", os.UserConfigDir, os.UserHomeDir); err == nil {
		return dir
	}
	if dir, err := os.UserConfigDir(); err == nil && strings.TrimSpace(dir) != "" {
		return filepath.Join(strings.TrimSpace(dir), "rainforest")
	}
	if home, err := os.UserHomeDir(); err == nil && strings.TrimSpace(home) != "" {
		return filepath.Join(strings.TrimSpace(home), ".config", "rainforest")
	}
	return ""
}

func DefaultVaultPath() string {
	dir := DefaultDataDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "credentials.json")
}

func (s *FileStore) Path() string {
	return s.path
}

func (s *FileStore) Load() (Credentials, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.path == "" {
		return Credentials{}, errors.New("credential vault path is not configured: persistent user data directory unavailable")
	}
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Credentials{}, ErrNotConfigured
		}
		return Credentials{}, fmt.Errorf("read credential vault: %s", aphotos.Redact(err.Error()))
	}

	var stored Credentials
	if err := json.Unmarshal(data, &stored); err != nil {
		return Credentials{}, fmt.Errorf("parse credential vault: %s", aphotos.Redact(err.Error()))
	}
	return ValidateAndNormalize(stored.Cookies, string(stored.Region), HostOverrides{
		DriveURL:     stored.DriveURL,
		UploadURL:    stored.UploadURL,
		ThumbnailURL: stored.ThumbnailURL,
	})
}

func (s *FileStore) Save(creds Credentials) error {
	normalized, err := ValidateAndNormalize(creds.Cookies, string(creds.Region), HostOverrides{
		DriveURL:     creds.DriveURL,
		UploadURL:    creds.UploadURL,
		ThumbnailURL: creds.ThumbnailURL,
	})
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.path == "" {
		return errors.New("credential vault path is not configured: persistent user data directory unavailable")
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create credential directory: %w", err)
	}
	if err := enforceDirPermissions(dir); err != nil {
		return fmt.Errorf("secure credential directory: %w", err)
	}

	payload, err := json.MarshalIndent(normalized, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal credentials: %w", err)
	}
	payload = append(payload, '\n')

	tmpFile, err := os.CreateTemp(dir, ".credentials-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp credential file: %w", err)
	}
	tmpPath := tmpFile.Name()
	cleanup := func() {
		_ = os.Remove(tmpPath)
	}

	if err := enforceFilePermissions(tmpPath); err != nil {
		_ = tmpFile.Close()
		cleanup()
		return fmt.Errorf("secure temp credential file: %w", err)
	}
	if _, err := tmpFile.Write(payload); err != nil {
		_ = tmpFile.Close()
		cleanup()
		return fmt.Errorf("write temp credential file: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		cleanup()
		return fmt.Errorf("sync temp credential file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close temp credential file: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		cleanup()
		return fmt.Errorf("commit credential file: %w", err)
	}
	if err := enforceFilePermissions(s.path); err != nil {
		return fmt.Errorf("secure credential file: %w", err)
	}
	return nil
}

func (s *FileStore) ImportCookies(rawInput, explicitRegion string, overrides HostOverrides) (Credentials, error) {
	parsed, err := ParseCookiesData([]byte(rawInput))
	if err != nil {
		return Credentials{}, err
	}
	creds, err := ValidateAndNormalize(parsed, explicitRegion, overrides)
	if err != nil {
		return Credentials{}, err
	}
	if err := s.Save(creds); err != nil {
		return Credentials{}, err
	}
	return creds, nil
}

func (s *FileStore) ImportCookieFile(path, explicitRegion string, overrides HostOverrides) (Credentials, error) {
	parsed, err := ParseCookieFile(path)
	if err != nil {
		return Credentials{}, err
	}
	creds, err := ValidateAndNormalize(parsed, explicitRegion, overrides)
	if err != nil {
		return Credentials{}, err
	}
	if err := s.Save(creds); err != nil {
		return Credentials{}, err
	}
	return creds, nil
}

func (s *FileStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove credential vault: %w", err)
	}
	return nil
}

func (s *FileStore) CheckPermissions() (ok bool, msg, hint string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return checkVaultPermissions(s.path)
}

func ValidateAndNormalize(rawCookies map[string]string, explicitRegion string, overrides HostOverrides) (Credentials, error) {
	if len(rawCookies) == 0 {
		return Credentials{}, &aphotos.ConfigError{
			Field:   "cookies",
			Message: "no cookies provided",
		}
	}

	normalized := make(map[string]string, len(rawCookies))
	for k, v := range rawCookies {
		ck := aphotos.CanonicalCookieKey(k)
		val := strings.TrimSpace(v)
		if ck != "" && val != "" {
			normalized[ck] = val
		}
	}
	if len(normalized) == 0 {
		return Credentials{}, &aphotos.ConfigError{
			Field:   "cookies",
			Message: "no valid cookies provided",
		}
	}

	region := aphotos.Region(strings.ToLower(strings.TrimSpace(explicitRegion)))
	if region == "" {
		inferred, err := aphotos.InferRegionFromCookies(normalized)
		if err != nil {
			return Credentials{}, redactError(err)
		}
		region = inferred
	}

	rc, err := aphotos.ResolveRegion(region)
	if err != nil {
		return Credentials{}, redactError(err)
	}

	canonicalCookies := make(map[string]string, len(rc.RequiredCookies))
	for _, reqCookie := range rc.RequiredCookies {
		val := strings.TrimSpace(normalized[reqCookie])
		if val == "" {
			return Credentials{}, &aphotos.ConfigError{
				Field:   "cookies",
				Message: fmt.Sprintf("missing required cookie %q for region %q", reqCookie, rc.Region),
			}
		}
		canonicalCookies[reqCookie] = val
	}

	opts := aphotos.Options{
		Region:       rc.Region,
		Cookies:      canonicalCookies,
		DriveURL:     strings.TrimSpace(overrides.DriveURL),
		UploadURL:    strings.TrimSpace(overrides.UploadURL),
		ThumbnailURL: strings.TrimSpace(overrides.ThumbnailURL),
	}
	client, err := aphotos.NewClient(opts)
	if err != nil {
		return Credentials{}, redactError(err)
	}
	_ = client.Close()

	return Credentials{
		Region:       rc.Region,
		Cookies:      canonicalCookies,
		DriveURL:     opts.DriveURL,
		UploadURL:    opts.UploadURL,
		ThumbnailURL: opts.ThumbnailURL,
	}, nil
}

type browserCookieJSON struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Domain string `json:"domain"`
}

func ParseCookieFile(path string) (map[string]string, error) {
	cleaned := strings.TrimSpace(path)
	if cleaned == "" {
		return nil, &aphotos.ConfigError{
			Field:   "cookie_file",
			Message: "cookie file path is required",
		}
	}
	data, err := os.ReadFile(cleaned)
	if err != nil {
		return nil, &aphotos.ConfigError{
			Field:   "cookie_file",
			Message: fmt.Sprintf("read cookie file %s: %v", cleaned, err),
			Cause:   err,
		}
	}
	return ParseCookiesData(data)
}

func ParseCookiesData(data []byte) (map[string]string, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, &aphotos.ConfigError{
			Field:   "cookies",
			Message: "cookie input is empty",
		}
	}

	switch trimmed[0] {
	case '[':
		var arr []browserCookieJSON
		if err := json.Unmarshal(trimmed, &arr); err != nil {
			return nil, &aphotos.ConfigError{
				Field:   "cookies",
				Message: fmt.Sprintf("invalid JSON cookie array: %s", aphotos.Redact(err.Error())),
				Cause:   err,
			}
		}
		out := make(map[string]string, len(arr))
		for _, item := range arr {
			name := aphotos.CanonicalCookieKey(item.Name)
			val := strings.TrimSpace(item.Value)
			if name == "" || val == "" {
				continue
			}
			out[name] = val
		}
		if len(out) == 0 {
			return nil, &aphotos.ConfigError{
				Field:   "cookies",
				Message: "JSON cookie array contained no valid cookies",
			}
		}
		return out, nil

	case '{':
		var m map[string]any
		if err := json.Unmarshal(trimmed, &m); err != nil {
			return nil, &aphotos.ConfigError{
				Field:   "cookies",
				Message: fmt.Sprintf("invalid JSON cookie map: %s", aphotos.Redact(err.Error())),
				Cause:   err,
			}
		}
		if rawCookies, ok := m["cookies"]; ok {
			if subMap, ok := rawCookies.(map[string]any); ok {
				m = subMap
			}
		}
		out := make(map[string]string, len(m))
		for k, v := range m {
			strVal, ok := v.(string)
			if !ok {
				continue
			}
			name := aphotos.CanonicalCookieKey(k)
			val := strings.TrimSpace(strVal)
			if name == "" || val == "" {
				continue
			}
			out[name] = val
		}
		if len(out) == 0 {
			return nil, &aphotos.ConfigError{
				Field:   "cookies",
				Message: "JSON cookie map contained no valid cookies",
			}
		}
		return out, nil
	}

	if cookies := parseNetscapeCookies(trimmed); len(cookies) > 0 {
		return cookies, nil
	}

	if cookies := parseRawCookieHeader(string(trimmed)); len(cookies) > 0 {
		return cookies, nil
	}

	return nil, &aphotos.ConfigError{
		Field:   "cookies",
		Message: "unrecognized cookie format (expected Netscape cookies.txt, JSON array/map, or Cookie header)",
	}
}

func parseNetscapeCookies(data []byte) map[string]string {
	out := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if after, ok := strings.CutPrefix(line, "#HttpOnly_"); ok {
			line = after
		} else if strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 7 {
			fields = strings.Fields(line)
			if len(fields) < 7 {
				continue
			}
		}
		name := aphotos.CanonicalCookieKey(fields[5])
		val := strings.TrimSpace(fields[6])
		if name != "" && val != "" {
			out[name] = val
		}
	}
	return out
}

func parseRawCookieHeader(raw string) map[string]string {
	out := make(map[string]string)
	lines := strings.SplitSeq(raw, "\n")
	for line := range lines {
		l := strings.TrimSpace(line)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		if len(l) >= 7 && strings.EqualFold(l[:7], "cookie:") {
			l = strings.TrimSpace(l[7:])
		}
		parts := strings.SplitSeq(l, ";")
		for part := range parts {
			kv := strings.TrimSpace(part)
			if kv == "" {
				continue
			}
			eqIdx := strings.IndexByte(kv, '=')
			if eqIdx <= 0 {
				continue
			}
			k := aphotos.CanonicalCookieKey(kv[:eqIdx])
			v := strings.TrimSpace(kv[eqIdx+1:])
			v = strings.Trim(v, `"`)
			if k != "" && v != "" && !strings.ContainsAny(k, " \t") {
				out[k] = v
			}
		}
	}
	return out
}

func redactError(err error) error {
	if err == nil {
		return nil
	}
	if cfgErr, ok := errors.AsType[*aphotos.ConfigError](err); ok {
		return &aphotos.ConfigError{
			Field:   cfgErr.Field,
			Message: aphotos.Redact(cfgErr.Message),
		}
	}
	return errors.New(aphotos.Redact(err.Error()))
}

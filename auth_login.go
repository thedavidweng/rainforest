//go:build !js

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/thedavidweng/amazon-photos/pkg/aphotos"
	"github.com/thedavidweng/rainforest/internal/credstore"
	"github.com/thedavidweng/rainforest/internal/store"
	"golang.org/x/term"
)

func desktopDataDir() (string, error) {
	mygo.App.SetName("Rainforest")
	primary, _ := mygo.App.Path(mygo.PathUserData)
	return resolveDesktopDataDir(primary)
}

const authLoginUsage = `usage: rainforest auth login [--region CODE] [--cookie DATA | --cookie-file PATH]

Sign in by storing Amazon Photos session cookies in the Rainforest vault.
A read-only account check runs before the saved account is replaced.

  --region CODE       marketplace (us, ca, uk, jp, de, fr, it, es, ...). Inferred when omitted.
  --cookie DATA       Cookie header, JSON, or Netscape data. Visible in the process list.
  --cookie-file PATH  file in those same formats.

With neither --cookie nor --cookie-file, a terminal asks for a hidden Cookie header.
Any other stdin is read as cookie data:

  rainforest auth login --region us --cookie-file ./cookies.txt
  rainforest auth login --region us < ./cookies.txt
`

type authIO struct {
	stdin     io.Reader
	stdout    io.Writer
	stderr    io.Writer
	term      func() bool
	line      func(prompt string) (string, error)
	secret    func(prompt string) (string, error)
	transport http.RoundTripper
}

func runAuthCommand(args []string) int {
	return runAuthCommandIO(args, defaultAuthIO(), "")
}

func defaultAuthIO() authIO {
	fd := int(os.Stdin.Fd())
	return authIO{
		stdin:  os.Stdin,
		stdout: os.Stdout,
		stderr: os.Stderr,
		term:   func() bool { return term.IsTerminal(fd) },
		line: func(prompt string) (string, error) {
			fmt.Fprint(os.Stderr, prompt)
			var line string
			_, err := fmt.Fscanln(os.Stdin, &line)
			return strings.TrimSpace(line), err
		},
		secret: func(prompt string) (string, error) {
			fmt.Fprint(os.Stderr, prompt)
			line, err := term.ReadPassword(fd)
			fmt.Fprintln(os.Stderr)
			return strings.TrimSpace(string(line)), err
		},
	}
}

func runAuthCommandIO(args []string, streams authIO, dataDir string) int {
	if streams.stderr == nil {
		streams.stderr = os.Stderr
	}
	if streams.stdout == nil {
		streams.stdout = os.Stdout
	}
	if len(args) == 0 || args[0] != "login" || (len(args) > 1 && (args[1] == "-h" || args[1] == "--help")) {
		fmt.Fprint(streams.stderr, authLoginUsage)
		if len(args) > 0 && args[0] == "login" {
			return 0
		}
		return 2
	}
	fs := flag.NewFlagSet("auth login", flag.ContinueOnError)
	fs.SetOutput(streams.stderr)
	region := fs.String("region", "", "Amazon marketplace code")
	cookie := fs.String("cookie", "", "Cookie header, JSON, or Netscape data")
	cookieFile := fs.String("cookie-file", "", "path to a cookie file")
	fs.Usage = func() { fmt.Fprint(streams.stderr, authLoginUsage) }
	if err := fs.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprint(streams.stderr, authLoginUsage)
		return 2
	}
	asked := strings.TrimSpace(*region)
	interactive := strings.TrimSpace(*cookie) == "" && strings.TrimSpace(*cookieFile) == "" && streams.term != nil && streams.term()
	if interactive && asked == "" {
		if streams.line == nil {
			fmt.Fprintln(streams.stderr, "auth login requires --region when marketplace cannot be prompted")
			return 1
		}
		line, err := streams.line("Marketplace (us, ca, uk, jp, de, fr, it, es): ")
		if err != nil {
			fmt.Fprintln(streams.stderr, aphotos.Redact(err.Error()))
			return 1
		}
		asked = line
	}
	raw, err := readLoginCookies(streams, *cookie, *cookieFile)
	if err != nil {
		fmt.Fprintln(streams.stderr, aphotos.Redact(err.Error()))
		return 1
	}
	if dataDir == "" {
		dataDir, err = desktopDataDir()
		if err != nil {
			fmt.Fprintln(streams.stderr, aphotos.Redact(err.Error()))
			return 1
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := commitCookieLogin(ctx, cookieLogin{
		Raw:       raw,
		Region:    asked,
		VaultPath: filepath.Join(dataDir, "credentials.json"),
		DBPath:    filepath.Join(dataDir, "rainforest.db"),
		Transport: streams.transport,
	})
	if err != nil {
		fmt.Fprintln(streams.stderr, aphotos.Redact(err.Error()))
		return 1
	}
	fmt.Fprintf(streams.stdout, "Signed in to Amazon Photos (%s, %s).\nSaved %s\n", result.Region, result.Evidence, result.VaultPath)
	return 0
}

func readLoginCookies(streams authIO, cookie, cookieFile string) ([]byte, error) {
	cookie = strings.TrimSpace(cookie)
	cookieFile = strings.TrimSpace(cookieFile)
	switch {
	case cookie != "" && cookieFile != "":
		return nil, fmt.Errorf("pass either --cookie or --cookie-file")
	case cookie != "":
		return []byte(cookie), nil
	case cookieFile != "":
		data, err := os.ReadFile(cookieFile)
		if err != nil {
			return nil, fmt.Errorf("read cookie file: %w", err)
		}
		return data, nil
	case streams.term != nil && streams.term():
		if streams.secret == nil {
			return nil, fmt.Errorf("auth login requires a cookie; pass --cookie, --cookie-file, or pipe cookie data")
		}
		line, err := streams.secret("Cookie (hidden; Ctrl+C to cancel): ")
		if err != nil {
			return nil, err
		}
		return []byte(line), nil
	default:
		if streams.stdin == nil {
			return nil, fmt.Errorf("auth login requires a cookie; pass --cookie, --cookie-file, or pipe cookie data")
		}
		data, err := io.ReadAll(io.LimitReader(streams.stdin, 1<<20))
		if err != nil {
			return nil, fmt.Errorf("read cookies from stdin: %w", err)
		}
		if strings.TrimSpace(string(data)) == "" {
			return nil, fmt.Errorf("auth login requires a cookie; pass --cookie, --cookie-file, or pipe cookie data")
		}
		return data, nil
	}
}

type cookieLogin struct {
	Raw       []byte
	Region    string
	VaultPath string
	DBPath    string
	Transport http.RoundTripper
}

type cookieLoginResult struct {
	Region    string
	Evidence  string
	VaultPath string
}

func commitCookieLogin(ctx context.Context, in cookieLogin) (cookieLoginResult, error) {
	parsed, err := credstore.ParseCookiesData(in.Raw)
	if err != nil {
		return cookieLoginResult{}, err
	}
	creds, err := credstore.ValidateAndNormalize(parsed, in.Region, credstore.HostOverrides{})
	if err != nil {
		return cookieLoginResult{}, err
	}
	client, err := aphotos.NewClient(aphotos.Options{
		Region:       creds.Region,
		Cookies:      creds.Cookies,
		DriveURL:     creds.DriveURL,
		UploadURL:    creds.UploadURL,
		ThumbnailURL: creds.ThumbnailURL,
		Transport:    in.Transport,
	})
	if err != nil {
		return cookieLoginResult{}, err
	}
	defer func() { _ = client.Close() }()
	if _, err = client.GetRoot(ctx); err != nil {
		return cookieLoginResult{}, err
	}
	cs := credstore.NewFileStore(in.VaultPath)
	if err := cs.Save(creds); err != nil {
		return cookieLoginResult{}, err
	}
	if err := syncSettingsRegion(ctx, in.DBPath, string(creds.Region)); err != nil {
		return cookieLoginResult{}, err
	}
	rc, err := aphotos.ResolveRegion(creds.Region)
	if err != nil {
		return cookieLoginResult{}, err
	}
	return cookieLoginResult{Region: string(creds.Region), Evidence: string(rc.Evidence), VaultPath: in.VaultPath}, nil
}

func syncSettingsRegion(ctx context.Context, dbPath, region string) error {
	st, err := store.Open(ctx, dbPath)
	if err != nil {
		return err
	}
	defer st.Close()
	cfg, err := st.GetSettings(ctx)
	if err != nil {
		return err
	}
	if cfg.Region == region {
		return nil
	}
	cfg.Region = region
	return st.UpdateSettings(ctx, cfg)
}

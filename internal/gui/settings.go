package gui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/thedavidweng/amazon-photos/pkg/aphotos"
	"github.com/thedavidweng/rainforest/internal/credstore"
	"github.com/thedavidweng/rainforest/internal/store"
)

type Settings struct {
	state *appState
}

type SettingsDTO struct {
	Region                 string `json:"region"`
	Evidence               string `json:"evidence,omitempty"`
	MaxMetadataConcurrency int    `json:"maxMetadataConcurrency"`
	MaxUploadConcurrency   int    `json:"maxUploadConcurrency"`
	DefaultConflictPolicy  string `json:"defaultConflictPolicy"`
	LaunchAtLogin          bool   `json:"launchAtLogin"`
	CloseToTray            bool   `json:"closeToTray"`
	NotificationsEnabled   bool   `json:"notificationsEnabled"`
	BackupsStopped         bool   `json:"backupsStopped"`
}

type SettingsInput struct {
	Region                 *string `json:"region,omitempty"`
	MaxMetadataConcurrency *int    `json:"maxMetadataConcurrency,omitempty"`
	MaxUploadConcurrency   *int    `json:"maxUploadConcurrency,omitempty"`
	DefaultConflictPolicy  *string `json:"defaultConflictPolicy,omitempty"`
	LaunchAtLogin          *bool   `json:"launchAtLogin,omitempty"`
	CloseToTray            *bool   `json:"closeToTray,omitempty"`
	NotificationsEnabled   *bool   `json:"notificationsEnabled,omitempty"`
	BackupsStopped         *bool   `json:"backupsStopped,omitempty"`
}

type DoctorCheckDTO struct {
	Grade   int    `json:"grade"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	Hint    string `json:"hint,omitempty"`
}

type DoctorReportDTO struct {
	Healthy      bool             `json:"healthy"`
	Unconfigured bool             `json:"unconfigured,omitempty"`
	Region       string           `json:"region,omitempty"`
	Evidence     string           `json:"evidence,omitempty"`
	Checks       []DoctorCheckDTO `json:"checks"`
}

func (s *Settings) Get(ctx context.Context) (*SettingsDTO, error) {
	st, _, err := s.state.deps()
	if err != nil {
		return nil, err
	}
	cfg, err := st.GetSettings(ctx)
	if err != nil {
		return nil, toError(err)
	}
	if cs, cErr := s.state.credVault(); cErr == nil {
		if creds, lErr := cs.Load(); lErr == nil && creds.Region != "" && cfg.Region == "" {
			cfg.Region = string(creds.Region)
		}
	}
	return toSettingsDTO(cfg), nil
}

func (s *Settings) Update(ctx context.Context, in SettingsInput) (*SettingsDTO, error) {
	st, _, err := s.state.deps()
	if err != nil {
		return nil, err
	}
	cfg, err := st.GetSettings(ctx)
	if err != nil {
		return nil, toError(err)
	}

	regionChanged := false
	concurrencyChanged := false

	if in.Region != nil {
		r := strings.ToLower(strings.TrimSpace(*in.Region))
		if r != "" {
			if _, rErr := aphotos.ResolveRegion(aphotos.Region(r)); rErr != nil {
				return nil, toError(rErr)
			}
		}
		if r != cfg.Region {
			cfg.Region = r
			regionChanged = true
		}
	}

	if in.MaxMetadataConcurrency != nil {
		if *in.MaxMetadataConcurrency < 1 || *in.MaxMetadataConcurrency > 16 {
			return nil, newError(ErrValidationFailed, "maxMetadataConcurrency must be between 1 and 16", map[string]any{"field": "maxMetadataConcurrency"})
		}
		if *in.MaxMetadataConcurrency != cfg.MaxMetadataConcurrency {
			cfg.MaxMetadataConcurrency = *in.MaxMetadataConcurrency
			concurrencyChanged = true
		}
	}

	if in.MaxUploadConcurrency != nil {
		if *in.MaxUploadConcurrency < 1 || *in.MaxUploadConcurrency > 8 {
			return nil, newError(ErrValidationFailed, "maxUploadConcurrency must be between 1 and 8", map[string]any{"field": "maxUploadConcurrency"})
		}
		if *in.MaxUploadConcurrency != cfg.MaxUploadConcurrency {
			cfg.MaxUploadConcurrency = *in.MaxUploadConcurrency
			concurrencyChanged = true
		}
	}

	if in.DefaultConflictPolicy != nil {
		pol := aphotos.ConflictPolicy(strings.ToLower(strings.TrimSpace(*in.DefaultConflictPolicy)))
		switch pol {
		case aphotos.ConflictSkip, aphotos.ConflictRename, aphotos.ConflictFail:
			cfg.DefaultConflictPolicy = pol
		default:
			return nil, newError(ErrValidationFailed, fmt.Sprintf("invalid conflict policy %q", *in.DefaultConflictPolicy), map[string]any{"field": "defaultConflictPolicy"})
		}
	}

	if in.LaunchAtLogin != nil && *in.LaunchAtLogin != cfg.LaunchAtLogin {
		s.state.mu.RLock()
		hook := s.state.launchAtLoginFn
		s.state.mu.RUnlock()
		if hook != nil {
			if hErr := hook(*in.LaunchAtLogin); hErr != nil {
				return nil, toError(hErr)
			}
		}
		cfg.LaunchAtLogin = *in.LaunchAtLogin
	}

	if in.CloseToTray != nil {
		cfg.CloseToTray = *in.CloseToTray
	}
	if in.NotificationsEnabled != nil {
		cfg.NotificationsEnabled = *in.NotificationsEnabled
	}
	if in.BackupsStopped != nil {
		cfg.BackupsStopped = *in.BackupsStopped
	}

	if err := st.UpdateSettings(ctx, cfg); err != nil {
		return nil, toError(err)
	}

	if regionChanged && cfg.Region != "" {
		if cs, cErr := s.state.credVault(); cErr == nil {
			if creds, lErr := cs.Load(); lErr == nil {
				creds.Region = aphotos.Region(cfg.Region)
				if saveErr := cs.Save(creds); saveErr == nil {
					_ = s.state.reopen(ctx)
				}
			}
		}
	} else if concurrencyChanged {
		_ = s.state.reopen(ctx)
	}

	return toSettingsDTO(cfg), nil
}

func (s *Settings) Doctor(ctx context.Context) (*DoctorReportDTO, error) {
	st, _, err := s.state.deps()
	if err != nil {
		return nil, err
	}
	cs, err := s.state.credVault()
	if err != nil {
		return nil, err
	}
	s.state.mu.RLock()
	opts := s.state.opts
	s.state.mu.RUnlock()

	var checks []DoctorCheckDTO
	vaultPath := cs.Path()

	if _, statErr := os.Stat(vaultPath); statErr != nil {
		if errors.Is(statErr, os.ErrNotExist) {
			checks = append(checks, DoctorCheckDTO{
				Grade:   1,
				Name:    "vault_permissions",
				Status:  "skipped",
				OK:      true,
				Message: fmt.Sprintf("credential vault %s not yet created", vaultPath),
			})
		} else {
			checks = append(checks, DoctorCheckDTO{
				Grade:   1,
				Name:    "vault_permissions",
				Status:  "fail",
				OK:      false,
				Message: aphotos.Redact(statErr.Error()),
			})
			return &DoctorReportDTO{Healthy: false, Checks: checks}, nil
		}
	} else {
		permOK, msg, hint := cs.CheckPermissions()
		status := "pass"
		if !permOK {
			status = "warn"
		}
		checks = append(checks, DoctorCheckDTO{
			Grade:   1,
			Name:    "vault_permissions",
			Status:  status,
			OK:      permOK,
			Message: aphotos.Redact(msg),
			Hint:    aphotos.Redact(hint),
		})
	}

	cfg, _ := st.GetSettings(ctx)
	creds, credErr := cs.Load()
	if credErr != nil {
		if errors.Is(credErr, credstore.ErrNotConfigured) {
			checks = append(checks,
				DoctorCheckDTO{
					Grade:   2,
					Name:    "credentials_and_region",
					Status:  "warn",
					OK:      false,
					Message: "No Amazon Photos credentials configured.",
					Hint:    "Import browser cookies in Settings -> Account to connect your account.",
				},
				DoctorCheckDTO{
					Grade:   3,
					Name:    "account_root_and_usage",
					Status:  "skipped",
					OK:      true,
					Message: "Skipped while offline.",
				},
				DoctorCheckDTO{
					Grade:   4,
					Name:    "search_read_probe",
					Status:  "skipped",
					OK:      true,
					Message: "Skipped while offline.",
				},
			)
			return &DoctorReportDTO{
				Healthy:      false,
				Unconfigured: true,
				Region:       cfg.Region,
				Checks:       checks,
			}, nil
		}

		checks = append(checks, DoctorCheckDTO{
			Grade:   2,
			Name:    "credentials_and_region",
			Status:  "fail",
			OK:      false,
			Message: aphotos.Redact(credErr.Error()),
			Hint:    "Re-import valid session cookies and select your explicit Amazon marketplace region.",
		})
		return &DoctorReportDTO{
			Healthy: false,
			Region:  cfg.Region,
			Checks:  checks,
		}, nil
	}

	client, clientErr := aphotos.NewClient(aphotos.Options{
		Region:                 creds.Region,
		Cookies:                creds.Cookies,
		DriveURL:               creds.DriveURL,
		UploadURL:              creds.UploadURL,
		ThumbnailURL:           creds.ThumbnailURL,
		HTTPClient:             opts.HTTPClient,
		Transport:              opts.Transport,
		MaxMetadataConcurrency: cfg.MaxMetadataConcurrency,
		MaxUploadConcurrency:   cfg.MaxUploadConcurrency,
	})
	if clientErr != nil {
		checks = append(checks, DoctorCheckDTO{
			Grade:   2,
			Name:    "credentials_and_region",
			Status:  "fail",
			OK:      false,
			Message: aphotos.Redact(clientErr.Error()),
		})
		return &DoctorReportDTO{
			Healthy: false,
			Region:  string(creds.Region),
			Checks:  checks,
		}, nil
	}
	defer func() { _ = client.Close() }()

	rc := client.RegionConfig()
	checks = append(checks, DoctorCheckDTO{
		Grade:   2,
		Name:    "credentials_and_region",
		Status:  "pass",
		OK:      true,
		Message: fmt.Sprintf("Region %s (%s) routed to %s with %d required cookies.", rc.Region, rc.Evidence, rc.Domain, len(rc.RequiredCookies)),
	})

	root, rootErr := client.GetRoot(ctx)
	if rootErr != nil {
		checks = append(checks, DoctorCheckDTO{
			Grade:   3,
			Name:    "account_root_and_usage",
			Status:  "fail",
			OK:      false,
			Message: aphotos.Redact(rootErr.Error()),
			Hint:    doctorHintForError(rootErr),
		})
		return &DoctorReportDTO{
			Healthy:  false,
			Region:   string(rc.Region),
			Evidence: string(rc.Evidence),
			Checks:   checks,
		}, nil
	}

	usage, usageErr := client.GetUsage(ctx)
	if usageErr != nil {
		checks = append(checks, DoctorCheckDTO{
			Grade:   3,
			Name:    "account_root_and_usage",
			Status:  "fail",
			OK:      false,
			Message: aphotos.Redact(usageErr.Error()),
			Hint:    doctorHintForError(usageErr),
		})
		return &DoctorReportDTO{
			Healthy:  false,
			Region:   string(rc.Region),
			Evidence: string(rc.Evidence),
			Checks:   checks,
		}, nil
	}

	checks = append(checks, DoctorCheckDTO{
		Grade:   3,
		Name:    "account_root_and_usage",
		Status:  "pass",
		OK:      true,
		Message: fmt.Sprintf("Root node %s verified (photos: %d, videos: %d).", root.ID, usage.Photo.Total.Count, usage.Video.Total.Count),
	})

	searchPage, searchErr := client.Search(ctx, aphotos.SearchOptions{Limit: 1})
	if searchErr != nil {
		checks = append(checks, DoctorCheckDTO{
			Grade:   4,
			Name:    "search_read_probe",
			Status:  "fail",
			OK:      false,
			Message: aphotos.Redact(searchErr.Error()),
			Hint:    doctorHintForError(searchErr),
		})
		return &DoctorReportDTO{
			Healthy:  false,
			Region:   string(rc.Region),
			Evidence: string(rc.Evidence),
			Checks:   checks,
		}, nil
	}

	checks = append(checks, DoctorCheckDTO{
		Grade:   4,
		Name:    "search_read_probe",
		Status:  "pass",
		OK:      true,
		Message: fmt.Sprintf("Search read probe succeeded (totalCount=%d).", searchPage.TotalCount),
	})

	return &DoctorReportDTO{
		Healthy:  true,
		Region:   string(rc.Region),
		Evidence: string(rc.Evidence),
		Checks:   checks,
	}, nil
}

func doctorHintForError(err error) string {
	var authErr *aphotos.AuthenticationError
	if errors.As(err, &authErr) {
		return "Session cookies were rejected (HTTP 401). Export fresh cookies from an active browser session and import them in Settings."
	}
	var permErr *aphotos.PermissionError
	if errors.As(err, &permErr) {
		return "Access denied (HTTP 403). Verify that your account matches the configured Amazon marketplace region."
	}
	var rlErr *aphotos.RateLimitError
	if errors.As(err, &rlErr) {
		return "Rate limited by Amazon Photos (HTTP 429). Wait for the Retry-After window and reduce concurrency."
	}
	var protoErr *aphotos.RemoteProtocolError
	if errors.As(err, &protoErr) {
		return "Unexpected response shape from the unofficial Amazon Photos Web API; the remote endpoint contract may have changed."
	}
	return ""
}

func toSettingsDTO(cfg store.Settings) *SettingsDTO {
	dto := &SettingsDTO{
		Region:                 cfg.Region,
		MaxMetadataConcurrency: cfg.MaxMetadataConcurrency,
		MaxUploadConcurrency:   cfg.MaxUploadConcurrency,
		DefaultConflictPolicy:  string(cfg.DefaultConflictPolicy),
		LaunchAtLogin:          cfg.LaunchAtLogin,
		CloseToTray:            cfg.CloseToTray,
		NotificationsEnabled:   cfg.NotificationsEnabled,
		BackupsStopped:         cfg.BackupsStopped,
	}
	if cfg.Region != "" {
		if rc, err := aphotos.ResolveRegion(aphotos.Region(strings.ToLower(strings.TrimSpace(cfg.Region)))); err == nil {
			dto.Evidence = string(rc.Evidence)
		}
	}
	return dto
}

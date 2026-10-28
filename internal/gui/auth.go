package gui

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"

	"github.com/thedavidweng/amazon-photos/pkg/aphotos"
	"github.com/thedavidweng/rainforest/internal/credstore"
	"github.com/thedavidweng/rainforest/internal/engine"
)

type Auth struct {
	state       *appState
	mu          sync.Mutex
	loginMu     sync.Mutex
	cancelLogin context.CancelFunc
}

type AuthStatus struct {
	Configured    bool     `json:"configured"`
	Authenticated bool     `json:"authenticated"`
	Region        string   `json:"region"`
	Evidence      string   `json:"evidence,omitempty"`
	RootNodeID    string   `json:"rootNodeId,omitempty"`
	OwnerID       string   `json:"ownerId,omitempty"`
	CookieKeys    []string `json:"cookieKeys,omitempty"`
	VaultPath     string   `json:"vaultPath,omitempty"`
	ErrorCode     string   `json:"errorCode,omitempty"`
	ErrorMessage  string   `json:"errorMessage,omitempty"`
}

type ImportCookiesInput struct {
	RawCookies   string `json:"rawCookies"`
	Region       string `json:"region,omitempty"`
	DriveURL     string `json:"driveUrl,omitempty"`
	UploadURL    string `json:"uploadUrl,omitempty"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
}

type UsageBucketDTO struct {
	Bytes int64 `json:"bytes"`
	Count int64 `json:"count"`
}

type UsageCategoryDTO struct {
	Billable UsageBucketDTO `json:"billable"`
	Total    UsageBucketDTO `json:"total"`
}

type UsageSummary struct {
	LastCalculated     string           `json:"lastCalculated,omitempty"`
	Photo              UsageCategoryDTO `json:"photo"`
	Video              UsageCategoryDTO `json:"video"`
	Other              UsageCategoryDTO `json:"other"`
	Doc                UsageCategoryDTO `json:"doc"`
	TotalBytes         int64            `json:"totalBytes"`
	TotalBillableBytes int64            `json:"totalBillableBytes"`
	TotalItems         int64            `json:"totalItems"`
}

func (a *Auth) Status(ctx context.Context) (*AuthStatus, error) {
	st, eng, err := a.state.deps()
	if err != nil {
		return nil, err
	}
	cs, err := a.state.credVault()
	if err != nil {
		return nil, err
	}

	out := &AuthStatus{
		VaultPath: cs.Path(),
	}
	if cfg, sErr := st.GetSettings(ctx); sErr == nil && strings.TrimSpace(cfg.Region) != "" {
		out.Region = strings.ToLower(strings.TrimSpace(cfg.Region))
		if rc, rErr := aphotos.ResolveRegion(aphotos.Region(out.Region)); rErr == nil {
			out.Evidence = string(rc.Evidence)
		}
	}

	creds, loadErr := cs.Load()
	if loadErr != nil {
		if errors.Is(loadErr, credstore.ErrNotConfigured) {
			return out, nil
		}
		mapped := toError(loadErr)
		if ge, ok := errors.AsType[*Error](mapped); ok {
			out.ErrorCode = ge.Code
			out.ErrorMessage = ge.Message
		} else {
			out.ErrorMessage = aphotos.Redact(loadErr.Error())
		}
		return out, nil
	}

	out.Configured = true
	out.Region = string(creds.Region)
	if rc, rErr := aphotos.ResolveRegion(creds.Region); rErr == nil {
		out.Evidence = string(rc.Evidence)
	}
	keys := make([]string, 0, len(creds.Cookies))
	for k := range creds.Cookies {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out.CookieKeys = keys

	client := eng.Client()
	if client == nil {
		if rErr := a.state.reopen(ctx); rErr != nil {
			if ge, ok := errors.AsType[*Error](rErr); ok {
				out.ErrorCode = ge.Code
				out.ErrorMessage = ge.Message
			}
			return out, nil
		}
		client = eng.Client()
	}
	if client == nil {
		return out, nil
	}

	root, rootErr := client.GetRoot(ctx)
	if rootErr != nil {
		mapped := toError(rootErr)
		if ge, ok := errors.AsType[*Error](mapped); ok {
			out.ErrorCode = ge.Code
			out.ErrorMessage = ge.Message
		} else {
			out.ErrorMessage = aphotos.Redact(rootErr.Error())
		}
		return out, nil
	}

	out.Authenticated = true
	if root != nil {
		out.RootNodeID = root.ID
		out.OwnerID = root.OwnerID
	}
	return out, nil
}

func (a *Auth) ImportCookies(ctx context.Context, in ImportCookiesInput) (*AuthStatus, error) {
	a.CancelLogin()
	a.mu.Lock()
	defer a.mu.Unlock()
	cs, err := a.state.credVault()
	if err != nil {
		return nil, err
	}
	if _, err := cs.ImportCookies(in.RawCookies, in.Region, credstore.HostOverrides{
		DriveURL:     in.DriveURL,
		UploadURL:    in.UploadURL,
		ThumbnailURL: in.ThumbnailURL,
	}); err != nil {
		return nil, toError(err)
	}
	if err := a.state.reopen(ctx); err != nil {
		return nil, err
	}
	status, err := a.Status(ctx)
	if err != nil {
		return nil, err
	}
	a.state.emit(EventAuthChanged, *status)
	return status, nil
}

func (a *Auth) ImportCookieFile(ctx context.Context, path, region string) (*AuthStatus, error) {
	a.CancelLogin()
	a.mu.Lock()
	defer a.mu.Unlock()
	cs, err := a.state.credVault()
	if err != nil {
		return nil, err
	}
	if _, err := cs.ImportCookieFile(path, region, credstore.HostOverrides{}); err != nil {
		return nil, toError(err)
	}
	if err := a.state.reopen(ctx); err != nil {
		return nil, err
	}
	status, err := a.Status(ctx)
	if err != nil {
		return nil, err
	}
	a.state.emit(EventAuthChanged, *status)
	return status, nil
}

func (a *Auth) PickAndImportCookieFile(ctx context.Context, region string) (*AuthStatus, error) {
	picker, err := a.state.filePicker()
	if err != nil {
		return nil, err
	}
	chosen, err := picker.PickCookieFile(ctx)
	if err != nil {
		return nil, toError(err)
	}
	if strings.TrimSpace(chosen) == "" {
		return a.Status(ctx)
	}
	return a.ImportCookieFile(ctx, chosen, region)
}

func (a *Auth) Logout(ctx context.Context) (*AuthStatus, error) {
	a.CancelLogin()
	a.mu.Lock()
	defer a.mu.Unlock()
	cs, err := a.state.credVault()
	if err != nil {
		return nil, err
	}
	if err := cs.Clear(); err != nil {
		return nil, toError(err)
	}
	if err := a.state.reopen(ctx); err != nil {
		return nil, err
	}
	status, err := a.Status(ctx)
	if err != nil {
		return nil, err
	}
	a.state.emit(EventAuthChanged, *status)
	return status, nil
}

func (a *Auth) Usage(ctx context.Context) (*UsageSummary, error) {
	_, eng, err := a.state.deps()
	if err != nil {
		return nil, err
	}
	client := eng.Client()
	if client == nil {
		return nil, toError(engine.ErrOffline)
	}
	usage, err := client.GetUsage(ctx)
	if err != nil {
		return nil, toError(err)
	}
	photo := toUsageCategoryDTO(usage.Photo)
	video := toUsageCategoryDTO(usage.Video)
	other := toUsageCategoryDTO(usage.Other)
	doc := toUsageCategoryDTO(usage.Doc)
	return &UsageSummary{
		LastCalculated:     formatTimeISO(usage.LastCalculated),
		Photo:              photo,
		Video:              video,
		Other:              other,
		Doc:                doc,
		TotalBytes:         photo.Total.Bytes + video.Total.Bytes + other.Total.Bytes + doc.Total.Bytes,
		TotalBillableBytes: photo.Billable.Bytes + video.Billable.Bytes + other.Billable.Bytes + doc.Billable.Bytes,
		TotalItems:         photo.Total.Count + video.Total.Count + other.Total.Count + doc.Total.Count,
	}, nil
}

func toUsageCategoryDTO(c aphotos.UsageCategory) UsageCategoryDTO {
	return UsageCategoryDTO{
		Billable: UsageBucketDTO{
			Bytes: c.Billable.Bytes,
			Count: c.Billable.Count,
		},
		Total: UsageBucketDTO{
			Bytes: c.Total.Bytes,
			Count: c.Total.Count,
		},
	}
}

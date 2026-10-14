package gui

import (
	"context"
	"strings"

	"github.com/thedavidweng/amazon-photos/pkg/aphotos"
	"github.com/thedavidweng/rainforest/internal/engine"
	"github.com/thedavidweng/rainforest/internal/store"
)

type Backups struct {
	state *appState
}

type BackupRuleInput struct {
	LocalPath         string `json:"localPath"`
	IncludeVideos     bool   `json:"includeVideos"`
	ScheduleMode      string `json:"scheduleMode,omitempty"`
	IntervalMinutes   int    `json:"intervalMinutes,omitempty"`
	ConflictPolicy    string `json:"conflictPolicy,omitempty"`
	DestinationNodeID string `json:"destinationNodeId,omitempty"`
	Paused            bool   `json:"paused,omitempty"`
}

type BackupRuleDTO struct {
	ID                string `json:"id"`
	LocalPath         string `json:"localPath"`
	IncludeVideos     bool   `json:"includeVideos"`
	ScheduleMode      string `json:"scheduleMode"`
	IntervalMinutes   int    `json:"intervalMinutes"`
	ConflictPolicy    string `json:"conflictPolicy"`
	DestinationNodeID string `json:"destinationNodeId,omitempty"`
	Paused            bool   `json:"paused"`
	Status            string `json:"status"`
	LastScanAt        string `json:"lastScanAt,omitempty"`
	LastError         string `json:"lastError,omitempty"`
	CreatedAt         string `json:"createdAt"`
	UpdatedAt         string `json:"updatedAt"`
}

func (b *Backups) List(ctx context.Context) ([]BackupRuleDTO, error) {
	_, eng, err := b.state.deps()
	if err != nil {
		return nil, err
	}
	rules, err := eng.ListBackupRules(ctx)
	if err != nil {
		return nil, toError(err)
	}
	out := make([]BackupRuleDTO, 0, len(rules))
	for i := range rules {
		out = append(out, toBackupRuleDTO(rules[i]))
	}
	return out, nil
}

func (b *Backups) Add(ctx context.Context, in BackupRuleInput) (*BackupRuleDTO, error) {
	_, eng, err := b.state.deps()
	if err != nil {
		return nil, err
	}
	rule, err := eng.AddBackupRule(ctx, toEngineBackupInput(in))
	if err != nil {
		return nil, toError(err)
	}
	dto := toBackupRuleDTO(rule)
	return &dto, nil
}

func (b *Backups) Update(ctx context.Context, id string, in BackupRuleInput) (*BackupRuleDTO, error) {
	_, eng, err := b.state.deps()
	if err != nil {
		return nil, err
	}
	rule, err := eng.UpdateBackupRule(ctx, strings.TrimSpace(id), toEngineBackupInput(in))
	if err != nil {
		return nil, toError(err)
	}
	dto := toBackupRuleDTO(rule)
	return &dto, nil
}

func (b *Backups) Pause(ctx context.Context, id string) (*BackupRuleDTO, error) {
	_, eng, err := b.state.deps()
	if err != nil {
		return nil, err
	}
	rule, err := eng.PauseBackupRule(ctx, strings.TrimSpace(id))
	if err != nil {
		return nil, toError(err)
	}
	dto := toBackupRuleDTO(rule)
	return &dto, nil
}

func (b *Backups) Resume(ctx context.Context, id string) (*BackupRuleDTO, error) {
	_, eng, err := b.state.deps()
	if err != nil {
		return nil, err
	}
	rule, err := eng.ResumeBackupRule(ctx, strings.TrimSpace(id))
	if err != nil {
		return nil, toError(err)
	}
	dto := toBackupRuleDTO(rule)
	return &dto, nil
}

func (b *Backups) Remove(ctx context.Context, id string) error {
	_, eng, err := b.state.deps()
	if err != nil {
		return err
	}
	if err := eng.RemoveBackupRule(ctx, strings.TrimSpace(id)); err != nil {
		return toError(err)
	}
	return nil
}

func (b *Backups) ScanNow(ctx context.Context, id string) (*TransferDTO, error) {
	_, eng, err := b.state.deps()
	if err != nil {
		return nil, err
	}
	tr, err := eng.ScanNow(ctx, strings.TrimSpace(id))
	if err != nil {
		return nil, toError(err)
	}
	if tr == nil {
		return nil, nil
	}
	dto := toTransferDTO(*tr)
	return &dto, nil
}

func (b *Backups) SetAllStopped(ctx context.Context, stopped bool) error {
	_, eng, err := b.state.deps()
	if err != nil {
		return err
	}
	if err := eng.SetBackupsStopped(ctx, stopped); err != nil {
		return toError(err)
	}
	return nil
}

func (b *Backups) PickFolder(ctx context.Context) (string, error) {
	picker, err := b.state.filePicker()
	if err != nil {
		return "", err
	}
	path, err := picker.PickDirectory(ctx, "Select Folder to Back Up")
	if err != nil {
		return "", toError(err)
	}
	return path, nil
}

func toEngineBackupInput(in BackupRuleInput) engine.BackupRuleInput {
	return engine.BackupRuleInput{
		LocalPath:         in.LocalPath,
		IncludeVideos:     in.IncludeVideos,
		ScheduleMode:      store.ScheduleMode(strings.TrimSpace(in.ScheduleMode)),
		IntervalMinutes:   in.IntervalMinutes,
		ConflictPolicy:    aphotos.ConflictPolicy(strings.TrimSpace(in.ConflictPolicy)),
		DestinationNodeID: strings.TrimSpace(in.DestinationNodeID),
		Paused:            in.Paused,
	}
}

func toBackupRuleDTO(r store.BackupRule) BackupRuleDTO {
	return BackupRuleDTO{
		ID:                r.ID,
		LocalPath:         r.LocalPath,
		IncludeVideos:     r.IncludeVideos,
		ScheduleMode:      string(r.ScheduleMode),
		IntervalMinutes:   r.IntervalMinutes,
		ConflictPolicy:    string(r.ConflictPolicy),
		DestinationNodeID: r.DestinationNodeID,
		Paused:            r.Paused,
		Status:            string(r.Status),
		LastScanAt:        formatOptionalTimeISO(r.LastScanAt),
		LastError:         aphotos.Redact(r.LastError),
		CreatedAt:         formatTimeISO(r.CreatedAt),
		UpdatedAt:         formatTimeISO(r.UpdatedAt),
	}
}

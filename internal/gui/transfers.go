package gui

import (
	"context"
	"strings"

	"github.com/thedavidweng/amazon-photos/pkg/aphotos"
	"github.com/thedavidweng/rainforest/internal/engine"
	"github.com/thedavidweng/rainforest/internal/store"
)

type Transfers struct {
	state *appState
}

type UploadPlanInput struct {
	Paths               []string `json:"paths"`
	DestinationNodeID   string   `json:"destinationNodeId,omitempty"`
	ConflictPolicy      string   `json:"conflictPolicy,omitempty"`
	IncludeVideos       *bool    `json:"includeVideos,omitempty"`
	IncludePreviews     *bool    `json:"includePreviews,omitempty"`
	KeepFolderStructure *bool    `json:"keepFolderStructure,omitempty"`
}

type DestinationCapabilityDTO struct {
	Region             string `json:"region"`
	Evidence           string `json:"evidence"`
	Name               string `json:"name"`
	Kind               string `json:"kind"`
	Status             string `json:"status"`
	DestinationNodeID  string `json:"destinationNodeId,omitempty"`
	Message            string `json:"message"`
	StructureSupported bool   `json:"structureSupported"`
}

type PlanItemDTO struct {
	LocalPath string `json:"localPath"`
	Name      string `json:"name"`
	MediaType string `json:"mediaType"`
	Size      int64  `json:"size"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Thumbnail []byte `json:"thumbnail,omitempty"`
	RemoteDir string `json:"remoteDir,omitempty"`
}

type UploadPlanDTO struct {
	SourceSummary       string                   `json:"sourceSummary"`
	PhotoCount          int                      `json:"photoCount"`
	VideoCount          int                      `json:"videoCount"`
	TotalCount          int                      `json:"totalCount"`
	TotalBytes          int64                    `json:"totalBytes"`
	SkippedUnsupported  int                      `json:"skippedUnsupported"`
	ConflictPolicy      string                   `json:"conflictPolicy"`
	KeepFolderStructure bool                     `json:"keepFolderStructure"`
	Destination         DestinationCapabilityDTO `json:"destination"`
	Items               []PlanItemDTO            `json:"items"`
}

type StartUploadInput struct {
	Paths               []string `json:"paths"`
	DestinationNodeID   string   `json:"destinationNodeId,omitempty"`
	ConflictPolicy      string   `json:"conflictPolicy,omitempty"`
	IncludeVideos       *bool    `json:"includeVideos,omitempty"`
	KeepFolderStructure *bool    `json:"keepFolderStructure,omitempty"`
}

type TransferListDTO struct {
	Active  []TransferDTO `json:"active"`
	History []TransferDTO `json:"history"`
	Warning string        `json:"warning,omitempty"`
}

func (t *Transfers) ProbeDestination(ctx context.Context, id string) (*DestinationCapabilityDTO, error) {
	_, eng, err := t.state.deps()
	if err != nil {
		return nil, err
	}
	d := toDestinationDTO(eng.ProbeDestination(ctx, id))
	return &d, nil
}

func toDestinationDTO(d engine.DestinationCapability) DestinationCapabilityDTO {
	return DestinationCapabilityDTO{Region: string(d.Region), Evidence: string(d.Evidence), Name: d.Name, Kind: d.Kind, Status: d.Status, DestinationNodeID: d.DestinationNodeID, Message: d.Message, StructureSupported: d.StructureSupported}
}

func (t *Transfers) PlanUpload(ctx context.Context, in UploadPlanInput) (*UploadPlanDTO, error) {
	_, eng, err := t.state.deps()
	if err != nil {
		return nil, err
	}
	includePreviews := true
	if in.IncludePreviews != nil {
		includePreviews = *in.IncludePreviews
	}
	plan, err := eng.PlanUpload(ctx, in.Paths, engine.PlanOptions{
		DestinationNodeID:   in.DestinationNodeID,
		ConflictPolicy:      aphotos.ConflictPolicy(strings.TrimSpace(in.ConflictPolicy)),
		IncludeVideos:       in.IncludeVideos,
		IncludePreviews:     includePreviews,
		KeepFolderStructure: in.KeepFolderStructure,
	})
	if err != nil {
		return nil, toError(err)
	}
	return toUploadPlanDTO(plan), nil
}

func (t *Transfers) StartUpload(ctx context.Context, in StartUploadInput) (*TransferDTO, error) {
	_, eng, err := t.state.deps()
	if err != nil {
		return nil, err
	}
	tr, err := eng.StartUpload(ctx, engine.UploadJobRequest{
		Paths:               in.Paths,
		DestinationNodeID:   in.DestinationNodeID,
		ConflictPolicy:      aphotos.ConflictPolicy(strings.TrimSpace(in.ConflictPolicy)),
		IncludeVideos:       in.IncludeVideos,
		KeepFolderStructure: in.KeepFolderStructure,
	})
	if err != nil {
		return nil, toError(err)
	}
	dto := toTransferDTO(*tr)
	return &dto, nil
}

func (t *Transfers) List(ctx context.Context) (*TransferListDTO, error) {
	_, eng, err := t.state.deps()
	if err != nil {
		return nil, err
	}
	all, err := eng.ListTransfers(ctx, 200)
	if err != nil {
		return nil, toError(err)
	}
	active := make([]TransferDTO, 0)
	history := make([]TransferDTO, 0)
	for i := range all {
		dto := toTransferDTO(all[i])
		if store.IsTerminalStage(all[i].Stage) {
			history = append(history, dto)
		} else {
			active = append(active, dto)
		}
	}
	return &TransferListDTO{
		Active:  active,
		History: history,
		Warning: t.state.reconcileWarning(),
	}, nil
}

func (t *Transfers) GetItems(ctx context.Context, transferID string) ([]TransferItemDTO, error) {
	_, eng, err := t.state.deps()
	if err != nil {
		return nil, err
	}
	items, err := eng.ListTransferItems(ctx, strings.TrimSpace(transferID))
	if err != nil {
		return nil, toError(err)
	}
	out := make([]TransferItemDTO, 0, len(items))
	for i := range items {
		out = append(out, toTransferItemDTO(items[i]))
	}
	return out, nil
}

func (t *Transfers) Pause(ctx context.Context, transferID string) (*TransferDTO, error) {
	_, eng, err := t.state.deps()
	if err != nil {
		return nil, err
	}
	id := strings.TrimSpace(transferID)
	if err := eng.PauseTransfer(ctx, id); err != nil {
		return nil, toError(err)
	}
	tr, err := eng.GetTransfer(ctx, id)
	if err != nil {
		return nil, toError(err)
	}
	dto := toTransferDTO(tr)
	return &dto, nil
}

func (t *Transfers) Resume(ctx context.Context, transferID string) (*TransferDTO, error) {
	_, eng, err := t.state.deps()
	if err != nil {
		return nil, err
	}
	id := strings.TrimSpace(transferID)
	if err := eng.ResumeTransfer(ctx, id); err != nil {
		return nil, toError(err)
	}
	tr, err := eng.GetTransfer(ctx, id)
	if err != nil {
		return nil, toError(err)
	}
	dto := toTransferDTO(tr)
	return &dto, nil
}

func (t *Transfers) Cancel(ctx context.Context, transferID string) (*TransferDTO, error) {
	_, eng, err := t.state.deps()
	if err != nil {
		return nil, err
	}
	id := strings.TrimSpace(transferID)
	if err := eng.CancelTransfer(ctx, id); err != nil {
		return nil, toError(err)
	}
	tr, err := eng.GetTransfer(ctx, id)
	if err != nil {
		return nil, toError(err)
	}
	dto := toTransferDTO(tr)
	return &dto, nil
}

func (t *Transfers) Retry(ctx context.Context, transferID string) (*TransferDTO, error) {
	_, eng, err := t.state.deps()
	if err != nil {
		return nil, err
	}
	tr, err := eng.RetryTransfer(ctx, strings.TrimSpace(transferID))
	if err != nil {
		return nil, toError(err)
	}
	dto := toTransferDTO(*tr)
	return &dto, nil
}

func (t *Transfers) ClearHistory(ctx context.Context) error {
	_, eng, err := t.state.deps()
	if err != nil {
		return err
	}
	if err := eng.ClearFinishedTransfers(ctx); err != nil {
		return toError(err)
	}
	return nil
}

func (t *Transfers) PickFiles(ctx context.Context) ([]string, error) {
	picker, err := t.state.filePicker()
	if err != nil {
		return nil, err
	}
	paths, err := picker.PickFiles(ctx)
	if err != nil {
		return nil, toError(err)
	}
	if paths == nil {
		return []string{}, nil
	}
	return paths, nil
}

func (t *Transfers) PickDirectory(ctx context.Context) (string, error) {
	picker, err := t.state.filePicker()
	if err != nil {
		return "", err
	}
	path, err := picker.PickDirectory(ctx, "Select Folder to Upload")
	if err != nil {
		return "", toError(err)
	}
	return path, nil
}

func toUploadPlanDTO(p *engine.UploadPlan) *UploadPlanDTO {
	if p == nil {
		return nil
	}
	items := make([]PlanItemDTO, 0, len(p.Items))
	for i := range p.Items {
		it := &p.Items[i]
		items = append(items, PlanItemDTO{
			LocalPath: it.LocalPath,
			Name:      it.Name,
			MediaType: string(it.MediaType),
			Size:      it.Size,
			Width:     it.Width,
			Height:    it.Height,
			Thumbnail: it.Thumbnail,
			RemoteDir: it.RemoteDir,
		})
	}
	return &UploadPlanDTO{
		SourceSummary:       p.SourceSummary,
		PhotoCount:          p.PhotoCount,
		VideoCount:          p.VideoCount,
		TotalCount:          p.TotalCount,
		TotalBytes:          p.TotalBytes,
		SkippedUnsupported:  p.SkippedUnsupported,
		ConflictPolicy:      string(p.ConflictPolicy),
		KeepFolderStructure: p.KeepFolderStructure,
		Destination:         toDestinationDTO(p.Destination),
		Items:               items,
	}
}

func toTransferDTO(tr store.Transfer) TransferDTO {
	return TransferDTO{
		ID:               tr.ID,
		Kind:             string(tr.Kind),
		Stage:            string(tr.Stage),
		SourceSummary:    tr.SourceSummary,
		DestNodeID:       tr.DestNodeID,
		DestLocalDir:     tr.DestLocalDir,
		ConflictPolicy:   string(tr.ConflictPolicy),
		BytesDone:        tr.BytesDone,
		BytesTotal:       tr.BytesTotal,
		ItemsDone:        tr.ItemsDone,
		ItemsSkipped:     tr.ItemsSkipped,
		ItemsFailed:      tr.ItemsFailed,
		ItemsUnconfirmed: tr.ItemsUnconfirmed,
		ItemsTotal:       tr.ItemsTotal,
		ErrorCode:        tr.ErrorCode,
		ErrorMessage:     aphotos.Redact(tr.ErrorMessage),
		CreatedAt:        formatTimeISO(tr.CreatedAt),
		UpdatedAt:        formatTimeISO(tr.UpdatedAt),
		FinishedAt:       formatOptionalTimeISO(tr.FinishedAt),
	}
}

func toTransferItemDTO(it store.TransferItem) TransferItemDTO {
	return TransferItemDTO{
		ID:           it.ID,
		TransferID:   it.TransferID,
		LocalPath:    it.LocalPath,
		Name:         it.Name,
		MediaType:    string(it.MediaType),
		Size:         it.Size,
		MD5:          it.MD5,
		RemoteNodeID: it.RemoteNodeID,
		Status:       string(it.Status),
		BytesDone:    it.BytesDone,
		ErrorMessage: aphotos.Redact(it.ErrorMessage),
		UpdatedAt:    formatTimeISO(it.UpdatedAt),
	}
}

type RecentUploadDTO struct {
	Name         string `json:"name"`
	MediaType    string `json:"mediaType"`
	LocalPath    string `json:"localPath"`
	RemoteNodeID string `json:"remoteNodeId"`
	UpdatedAt    string `json:"updatedAt"`
	Thumbnail    []byte `json:"thumbnail,omitempty"`
}

type FolderRefDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type FolderListingDTO struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Path      []FolderRefDTO `json:"path"`
	Folders   []FolderRefDTO `json:"folders"`
	Truncated bool           `json:"truncated"`
}

func (t *Transfers) RecentUploads(ctx context.Context, limit int) ([]RecentUploadDTO, error) {
	_, eng, err := t.state.deps()
	if err != nil {
		return nil, err
	}
	recent, err := eng.RecentUploads(ctx, limit)
	if err != nil {
		return nil, toError(err)
	}
	out := make([]RecentUploadDTO, 0, len(recent))
	for i := range recent {
		it := recent[i].Item
		out = append(out, RecentUploadDTO{
			Name:         it.Name,
			MediaType:    string(it.MediaType),
			LocalPath:    it.LocalPath,
			RemoteNodeID: it.RemoteNodeID,
			UpdatedAt:    formatTimeISO(it.UpdatedAt),
			Thumbnail:    recent[i].Thumbnail,
		})
	}
	return out, nil
}

func (t *Transfers) LastUploadAt(ctx context.Context) (string, error) {
	_, eng, err := t.state.deps()
	if err != nil {
		return "", err
	}
	at, err := eng.LastUploadAt(ctx)
	if err != nil {
		return "", toError(err)
	}
	return formatOptionalTimeISO(at), nil
}

func (t *Transfers) ListFolders(ctx context.Context, parentID string) (*FolderListingDTO, error) {
	_, eng, err := t.state.deps()
	if err != nil {
		return nil, err
	}
	l, err := eng.ListFolders(ctx, parentID)
	if err != nil {
		return nil, toError(err)
	}
	return &FolderListingDTO{
		ID:        l.Folder.ID,
		Name:      l.Folder.Name,
		Path:      toFolderRefDTOs(l.Path),
		Folders:   toFolderRefDTOs(l.Children),
		Truncated: l.Truncated,
	}, nil
}

func toFolderRefDTOs(in []engine.FolderRef) []FolderRefDTO {
	out := make([]FolderRefDTO, 0, len(in))
	for _, f := range in {
		out = append(out, FolderRefDTO{ID: f.ID, Name: f.Name})
	}
	return out
}

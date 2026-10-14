package gui

import (
	"context"
	"strings"

	"github.com/thedavidweng/amazon-photos/pkg/aphotos"
	"github.com/thedavidweng/rainforest/internal/engine"
	"github.com/thedavidweng/rainforest/internal/store"
)

type Downloads struct {
	state *appState
}

type RecoveryContainerDTO struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

type RecoverySourcesDTO struct {
	Region          string                 `json:"region"`
	Evidence        string                 `json:"evidence"`
	RootNodeID      string                 `json:"rootNodeId,omitempty"`
	ConfirmedItems  []TransferItemDTO      `json:"confirmedItems"`
	Folders         []RecoveryContainerDTO `json:"folders"`
	Albums          []RecoveryContainerDTO `json:"albums"`
	CapabilityNotes string                 `json:"capabilityNotes,omitempty"`
	Warning         string                 `json:"warning,omitempty"`
}

type DownloadItemInput struct {
	NodeID       string `json:"nodeId"`
	Name         string `json:"name,omitempty"`
	ExpectedSize int64  `json:"expectedSize,omitempty"`
	ExpectedMD5  string `json:"expectedMd5,omitempty"`
	MediaType    string `json:"mediaType,omitempty"`
}

type DownloadInput struct {
	NodeIDs         []string            `json:"nodeIds,omitempty"`
	Items           []DownloadItemInput `json:"items,omitempty"`
	ContainerNodeID string              `json:"containerNodeId,omitempty"`
	DestDir         string              `json:"destDir"`
	ConflictPolicy  string              `json:"conflictPolicy,omitempty"`
}

func (d *Downloads) ListDestinations(ctx context.Context) (*RecoverySourcesDTO, error) {
	_, eng, err := d.state.deps()
	if err != nil {
		return nil, err
	}
	src, err := eng.ListRecoverySources(ctx)
	if err != nil {
		return nil, toError(err)
	}
	confirmed := make([]TransferItemDTO, 0, len(src.ConfirmedItems))
	for i := range src.ConfirmedItems {
		confirmed = append(confirmed, toTransferItemDTO(src.ConfirmedItems[i]))
	}
	folders := make([]RecoveryContainerDTO, 0, len(src.Folders))
	for i := range src.Folders {
		folders = append(folders, RecoveryContainerDTO{
			ID:   src.Folders[i].ID,
			Name: src.Folders[i].Name,
			Kind: string(src.Folders[i].Kind),
		})
	}
	albums := make([]RecoveryContainerDTO, 0, len(src.Albums))
	for i := range src.Albums {
		albums = append(albums, RecoveryContainerDTO{
			ID:   src.Albums[i].ID,
			Name: src.Albums[i].Name,
			Kind: string(src.Albums[i].Kind),
		})
	}

	notes := "Recovery download supports confirmed transfer history items, explicit remote node IDs, and verified folders or albums."
	if src.Evidence != "verified" {
		notes = "Experimental region: remote folder/album listing is unverified; recovery from confirmed transfer history or explicit node IDs is recommended."
	}

	return &RecoverySourcesDTO{
		Region:          string(src.Region),
		Evidence:        string(src.Evidence),
		RootNodeID:      src.RootNodeID,
		ConfirmedItems:  confirmed,
		Folders:         folders,
		Albums:          albums,
		CapabilityNotes: notes,
		Warning:         src.Warning,
	}, nil
}

func (d *Downloads) StartDownload(ctx context.Context, in DownloadInput) (*TransferDTO, error) {
	_, eng, err := d.state.deps()
	if err != nil {
		return nil, err
	}
	specs := make([]engine.DownloadItemSpec, 0, len(in.Items))
	for i := range in.Items {
		it := &in.Items[i]
		specs = append(specs, engine.DownloadItemSpec{
			NodeID:       strings.TrimSpace(it.NodeID),
			Name:         strings.TrimSpace(it.Name),
			ExpectedSize: it.ExpectedSize,
			ExpectedMD5:  strings.TrimSpace(it.ExpectedMD5),
			MediaType:    store.MediaType(strings.TrimSpace(it.MediaType)),
		})
	}
	tr, err := eng.StartDownload(ctx, engine.DownloadJobRequest{
		NodeIDs:         in.NodeIDs,
		Items:           specs,
		ContainerNodeID: strings.TrimSpace(in.ContainerNodeID),
		DestDir:         strings.TrimSpace(in.DestDir),
		ConflictPolicy:  aphotos.ConflictPolicy(strings.ToLower(strings.TrimSpace(in.ConflictPolicy))),
	})
	if err != nil {
		return nil, toError(err)
	}
	dto := toTransferDTO(*tr)
	return &dto, nil
}

func (d *Downloads) PickTargetDirectory(ctx context.Context) (string, error) {
	picker, err := d.state.filePicker()
	if err != nil {
		return "", err
	}
	path, err := picker.PickDirectory(ctx, "Select Download Destination Directory")
	if err != nil {
		return "", toError(err)
	}
	return path, nil
}

package preview

import (
	"context"
	"fmt"
	"time"

	"github.com/thedavidweng/rainforest/internal/gui"
	"github.com/thedavidweng/rainforest/internal/view"
)

func Services() view.Services {
	empty := gui.NewEmptyServices()
	stamp := time.Now().UTC().Format(time.RFC3339)
	items := []gui.TransferItemDTO{
		{ID: "photo-1", TransferID: "trip", Name: "coast.jpg", LocalPath: "/Pictures/Coast/coast.jpg", MediaType: "photo", Size: 6291456, BytesDone: 6291456, Status: "confirmed", RemoteNodeID: "coast", UpdatedAt: stamp},
		{ID: "photo-2", TransferID: "trip", Name: "sunset.jpg", LocalPath: "/Pictures/Coast/sunset.jpg", MediaType: "photo", Size: 4194304, BytesDone: 4194304, Status: "confirmed", RemoteNodeID: "sunset", UpdatedAt: stamp},
	}
	return view.Services{
		Auth:      auth{Auth: empty.Auth},
		Transfers: transfers{Transfers: empty.Transfers, items: items, stamp: stamp},
		Backups:   backups{Backups: empty.Backups, stamp: stamp},
		Downloads: downloads{Downloads: empty.Downloads, items: items},
		Settings:  settings{Settings: empty.Settings},
	}
}

type auth struct{ *gui.Auth }

func (auth) Status(context.Context) (*gui.AuthStatus, error) {
	return &gui.AuthStatus{Configured: true, Authenticated: true, Region: "us", Evidence: "verified", RootNodeID: "root"}, nil
}

func (auth) Usage(context.Context) (*gui.UsageSummary, error) {
	return &gui.UsageSummary{Photo: gui.UsageCategoryDTO{Total: gui.UsageBucketDTO{Bytes: 12884901888, Count: 2048}}, Video: gui.UsageCategoryDTO{Total: gui.UsageBucketDTO{Bytes: 536870912, Count: 12}}, TotalBytes: (12 << 30) + (512 << 20), TotalItems: 2060}, nil
}

type transfers struct {
	*gui.Transfers
	items []gui.TransferItemDTO
	stamp string
}

func (t transfers) List(context.Context) (*gui.TransferListDTO, error) {
	return &gui.TransferListDTO{
		Active:  []gui.TransferDTO{{ID: "family", Kind: "upload", Stage: "paused", SourceSummary: "Family weekend", BytesDone: 12582912, BytesTotal: 33554432, ItemsDone: 2, ItemsTotal: 6, CreatedAt: t.stamp, UpdatedAt: t.stamp}},
		History: []gui.TransferDTO{{ID: "trip", Kind: "backup", Stage: "completed", SourceSummary: "Coast", BytesDone: 10485760, BytesTotal: 10485760, ItemsDone: 2, ItemsTotal: 2, CreatedAt: t.stamp, UpdatedAt: t.stamp, FinishedAt: t.stamp}},
	}, nil
}

func (t transfers) GetItems(_ context.Context, id string) ([]gui.TransferItemDTO, error) {
	switch id {
	case "trip":
		return t.items, nil
	case "family":
		return []gui.TransferItemDTO{{ID: "family-1", TransferID: id, Name: "weekend.jpg", MediaType: "photo", Size: 6291456, BytesDone: 6291456, Status: "confirmed", UpdatedAt: t.stamp}}, nil
	default:
		return nil, fmt.Errorf("unknown preview transfer: %s", id)
	}
}

func (t transfers) RecentUploads(_ context.Context, limit int) ([]gui.RecentUploadDTO, error) {
	recent := make([]gui.RecentUploadDTO, 0, len(t.items))
	for i := range t.items {
		item := &t.items[i]
		recent = append(recent, gui.RecentUploadDTO{Name: item.Name, MediaType: item.MediaType, LocalPath: item.LocalPath, RemoteNodeID: item.RemoteNodeID, UpdatedAt: t.stamp})
	}
	return recent[:min(limit, len(recent))], nil
}
func (t transfers) LastUploadAt(context.Context) (string, error) { return t.stamp, nil }
func (transfers) ListFolders(_ context.Context, id string) (*gui.FolderListingDTO, error) {
	switch id {
	case "", "root":
		return &gui.FolderListingDTO{ID: "root", Folders: []gui.FolderRefDTO{{ID: "trips", Name: "Trips"}, {ID: "family", Name: "Family"}}}, nil
	case "trips", "family":
		name := "Trips"
		if id == "family" {
			name = "Family"
		}
		return &gui.FolderListingDTO{ID: id, Name: name, Path: []gui.FolderRefDTO{{ID: id, Name: name}}}, nil
	default:
		return nil, fmt.Errorf("unknown preview folder: %s", id)
	}
}

func (transfers) ProbeDestination(_ context.Context, id string) (*gui.DestinationCapabilityDTO, error) {
	name := map[string]string{"trips": "Trips", "family": "Family"}[id]
	if name == "" {
		return nil, fmt.Errorf("unknown preview destination: %s", id)
	}
	return &gui.DestinationCapabilityDTO{Region: "us", Evidence: "verified", Name: name, Kind: "FOLDER", Status: "readable", DestinationNodeID: id, StructureSupported: true}, nil
}

type backups struct {
	*gui.Backups
	stamp string
}

func (b backups) List(context.Context) ([]gui.BackupRuleDTO, error) {
	return []gui.BackupRuleDTO{
		{ID: "coast", LocalPath: "/Pictures/Coast", IncludeVideos: true, ScheduleMode: "immediate", ConflictPolicy: "skip", DestinationNodeID: "trips", KeepFolderStructure: true, Status: "active", LastScanAt: b.stamp, CreatedAt: b.stamp, UpdatedAt: b.stamp},
		{ID: "family", LocalPath: "/Pictures/Family", IncludeVideos: true, ScheduleMode: "interval", IntervalMinutes: 30, ConflictPolicy: "skip", DestinationNodeID: "family", KeepFolderStructure: true, Status: "active", LastScanAt: b.stamp, CreatedAt: b.stamp, UpdatedAt: b.stamp},
	}, nil
}

func (b backups) Files(_ context.Context, id string) ([]gui.TrackedFileDTO, error) {
	name := map[string]string{"coast": "2026/coast.jpg", "family": "weekend.jpg"}[id]
	if name == "" {
		return nil, fmt.Errorf("unknown preview backup: %s", id)
	}
	return []gui.TrackedFileDTO{{RelPath: name, Size: 6291456, Status: "confirmed", RemoteNodeID: id, LastSyncedAt: b.stamp}}, nil
}

type downloads struct {
	*gui.Downloads
	items []gui.TransferItemDTO
}

func (d downloads) ListDestinations(context.Context) (*gui.RecoverySourcesDTO, error) {
	return &gui.RecoverySourcesDTO{Region: "us", Evidence: "verified", RootNodeID: "root", ConfirmedItems: d.items, Folders: []gui.RecoveryContainerDTO{{ID: "trips", Name: "Trips", Kind: "FOLDER"}, {ID: "family", Name: "Family", Kind: "FOLDER"}}}, nil
}

type settings struct{ *gui.Settings }

func (settings) Get(context.Context) (*gui.SettingsDTO, error) {
	return &gui.SettingsDTO{Region: "us", Evidence: "verified", MaxUploadConcurrency: 2, MaxMetadataConcurrency: 4, DefaultConflictPolicy: "skip", CloseToTray: true, NotificationsEnabled: true}, nil
}

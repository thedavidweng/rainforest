package preview

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
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
		{ID: "photo-3", TransferID: "trip", Name: "waterfall.jpg", LocalPath: "/Pictures/Coast/waterfall.jpg", MediaType: "photo", Size: 5242880, BytesDone: 5242880, Status: "confirmed", RemoteNodeID: "waterfall", UpdatedAt: stamp},
		{ID: "photo-4", TransferID: "trip", Name: "travel-desk.jpg", LocalPath: "/Pictures/Coast/travel-desk.jpg", MediaType: "photo", Size: 3932160, BytesDone: 3932160, Status: "confirmed", RemoteNodeID: "desk", UpdatedAt: stamp},
		{ID: "photo-5", TransferID: "trip", Name: "forest-trail.jpg", LocalPath: "/Pictures/Coast/forest-trail.jpg", MediaType: "photo", Size: 4718592, BytesDone: 4718592, Status: "confirmed", RemoteNodeID: "trail", UpdatedAt: stamp},
		{ID: "photo-6", TransferID: "trip", Name: "mountain-peak.jpg", LocalPath: "/Pictures/Coast/mountain-peak.jpg", MediaType: "photo", Size: 7077888, BytesDone: 7077888, Status: "confirmed", RemoteNodeID: "mountain", UpdatedAt: stamp},
		{ID: "photo-7", TransferID: "trip", Name: "alpine-lake.jpg", LocalPath: "/Pictures/Coast/alpine-lake.jpg", MediaType: "photo", Size: 3670016, BytesDone: 3670016, Status: "confirmed", RemoteNodeID: "lake", UpdatedAt: stamp},
		{ID: "photo-8", TransferID: "trip", Name: "coastal-cliffs.jpg", LocalPath: "/Pictures/Coast/coastal-cliffs.jpg", MediaType: "photo", Size: 5505024, BytesDone: 5505024, Status: "confirmed", RemoteNodeID: "cliffs", UpdatedAt: stamp},
	}
	return view.Services{
		Auth:      auth{Auth: empty.Auth},
		Transfers: &transfers{Transfers: empty.Transfers, items: items, stamp: stamp},
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
	items  []gui.TransferItemDTO
	stamp  string
	thumbs sync.Map
}

func (t *transfers) List(context.Context) (*gui.TransferListDTO, error) {
	var totalBytes int64
	for i := range t.items {
		totalBytes += t.items[i].Size
	}
	return &gui.TransferListDTO{
		Active:  []gui.TransferDTO{{ID: "family", Kind: "upload", Stage: "paused", SourceSummary: "Family weekend", BytesDone: 12582912, BytesTotal: 33554432, ItemsDone: 2, ItemsTotal: 6, CreatedAt: t.stamp, UpdatedAt: t.stamp}},
		History: []gui.TransferDTO{{ID: "trip", Kind: "backup", Stage: "completed", SourceSummary: "Coast", BytesDone: totalBytes, BytesTotal: totalBytes, ItemsDone: len(t.items), ItemsTotal: len(t.items), CreatedAt: t.stamp, UpdatedAt: t.stamp, FinishedAt: t.stamp}},
	}, nil
}

func (t *transfers) GetItems(_ context.Context, id string) ([]gui.TransferItemDTO, error) {
	switch id {
	case "trip":
		return t.items, nil
	case "family":
		return []gui.TransferItemDTO{
			{ID: "family-1", TransferID: id, Name: "weekend.jpg", MediaType: "photo", Size: 6291456, BytesDone: 6291456, Status: "confirmed", UpdatedAt: t.stamp},
			{ID: "family-2", TransferID: id, Name: "park.jpg", MediaType: "photo", Size: 6291456, BytesDone: 6291456, Status: "confirmed", UpdatedAt: t.stamp},
			{ID: "family-3", TransferID: id, Name: "picnic.jpg", MediaType: "photo", Size: 5242880, BytesDone: 0, Status: "pending", UpdatedAt: t.stamp},
			{ID: "family-4", TransferID: id, Name: "kids.jpg", MediaType: "photo", Size: 4194304, BytesDone: 0, Status: "pending", UpdatedAt: t.stamp},
			{ID: "family-5", TransferID: id, Name: "garden.jpg", MediaType: "photo", Size: 5767168, BytesDone: 0, Status: "pending", UpdatedAt: t.stamp},
			{ID: "family-6", TransferID: id, Name: "dinner.jpg", MediaType: "photo", Size: 5767168, BytesDone: 0, Status: "pending", UpdatedAt: t.stamp},
		}, nil
	default:
		return nil, fmt.Errorf("unknown preview transfer: %s", id)
	}
}

func (t *transfers) RecentUploads(ctx context.Context, limit int) ([]gui.RecentUploadDTO, error) {
	n := min(limit, len(t.items))
	recent := make([]gui.RecentUploadDTO, n)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		item := &t.items[i]
		recent[i] = gui.RecentUploadDTO{
			Name:         item.Name,
			MediaType:    item.MediaType,
			LocalPath:    item.LocalPath,
			RemoteNodeID: item.RemoteNodeID,
			UpdatedAt:    t.stamp,
		}
		if cached, ok := t.thumbs.Load(item.ID); ok {
			if b, ok := cached.([]byte); ok {
				recent[i].Thumbnail = b
			}
			continue
		}
		wg.Add(1)
		go func(idx int, id string) {
			defer wg.Done()
			data := fetchPicsumThumb(ctx, idx)
			if len(data) > 0 {
				t.thumbs.Store(id, data)
				recent[idx].Thumbnail = data
			}
		}(i, item.ID)
	}
	wg.Wait()
	return recent, nil
}

func fetchPicsumThumb(ctx context.Context, idx int) []byte {
	client := &http.Client{Timeout: 2 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://picsum.photos/128?random=%d", idx+1), http.NoBody)
	if err != nil {
		return nil
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil
	}
	data, _ := io.ReadAll(resp.Body)
	return data
}

func (*transfers) LastUploadAt(context.Context) (string, error) {
	return time.Now().UTC().Format(time.RFC3339), nil
}

func (*transfers) ListFolders(_ context.Context, id string) (*gui.FolderListingDTO, error) {
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

func (*transfers) ProbeDestination(_ context.Context, id string) (*gui.DestinationCapabilityDTO, error) {
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
	switch id {
	case "coast":
		return []gui.TrackedFileDTO{
			{RelPath: "2026/coast.jpg", Size: 6291456, Status: "confirmed", RemoteNodeID: "coast", LastSyncedAt: b.stamp},
			{RelPath: "2026/sunset.jpg", Size: 4194304, Status: "confirmed", RemoteNodeID: "sunset", LastSyncedAt: b.stamp},
			{RelPath: "2026/waterfall.jpg", Size: 5242880, Status: "confirmed", RemoteNodeID: "waterfall", LastSyncedAt: b.stamp},
			{RelPath: "2026/travel-desk.jpg", Size: 3932160, Status: "confirmed", RemoteNodeID: "desk", LastSyncedAt: b.stamp},
			{RelPath: "2026/forest-trail.jpg", Size: 4718592, Status: "confirmed", RemoteNodeID: "trail", LastSyncedAt: b.stamp},
			{RelPath: "2026/mountain-peak.jpg", Size: 7077888, Status: "confirmed", RemoteNodeID: "mountain", LastSyncedAt: b.stamp},
			{RelPath: "2026/alpine-lake.jpg", Size: 3670016, Status: "confirmed", RemoteNodeID: "lake", LastSyncedAt: b.stamp},
			{RelPath: "2026/coastal-cliffs.jpg", Size: 5505024, Status: "confirmed", RemoteNodeID: "cliffs", LastSyncedAt: b.stamp},
		}, nil
	case "family":
		return []gui.TrackedFileDTO{
			{RelPath: "weekend.jpg", Size: 6291456, Status: "confirmed", RemoteNodeID: "family-1", LastSyncedAt: b.stamp},
			{RelPath: "park.jpg", Size: 3932160, Status: "confirmed", RemoteNodeID: "family-2", LastSyncedAt: b.stamp},
		}, nil
	default:
		return nil, fmt.Errorf("unknown preview backup: %s", id)
	}
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

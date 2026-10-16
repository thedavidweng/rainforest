package view

import (
	"context"

	"github.com/thedavidweng/rainforest/internal/gui"
)

type Services struct {
	Auth interface {
		Login(ctx context.Context, region string) (*gui.AuthStatus, error)
		CancelLogin()
		Status(ctx context.Context) (*gui.AuthStatus, error)
		ImportCookies(ctx context.Context, in gui.ImportCookiesInput) (*gui.AuthStatus, error)
		PickAndImportCookieFile(ctx context.Context, region string) (*gui.AuthStatus, error)
		Logout(ctx context.Context) (*gui.AuthStatus, error)
		Usage(ctx context.Context) (*gui.UsageSummary, error)
	}
	Transfers interface {
		ProbeDestination(ctx context.Context, id string) (*gui.DestinationCapabilityDTO, error)
		PlanUpload(ctx context.Context, in gui.UploadPlanInput) (*gui.UploadPlanDTO, error)
		StartUpload(ctx context.Context, in gui.StartUploadInput) (*gui.TransferDTO, error)
		List(ctx context.Context) (*gui.TransferListDTO, error)
		GetItems(ctx context.Context, transferID string) ([]gui.TransferItemDTO, error)
		Pause(ctx context.Context, transferID string) (*gui.TransferDTO, error)
		Resume(ctx context.Context, transferID string) (*gui.TransferDTO, error)
		Cancel(ctx context.Context, transferID string) (*gui.TransferDTO, error)
		Retry(ctx context.Context, transferID string) (*gui.TransferDTO, error)
		ClearHistory(ctx context.Context) error
		PickFiles(ctx context.Context) ([]string, error)
		PickDirectory(ctx context.Context) (string, error)
		RecentUploads(ctx context.Context, limit int) ([]gui.RecentUploadDTO, error)
		LastUploadAt(ctx context.Context) (string, error)
		ListFolders(ctx context.Context, parentID string) (*gui.FolderListingDTO, error)
	}
	Backups interface {
		List(ctx context.Context) ([]gui.BackupRuleDTO, error)
		Add(ctx context.Context, in gui.BackupRuleInput) (*gui.BackupRuleDTO, error)
		Update(ctx context.Context, id string, in gui.BackupRuleInput) (*gui.BackupRuleDTO, error)
		Pause(ctx context.Context, id string) (*gui.BackupRuleDTO, error)
		Resume(ctx context.Context, id string) (*gui.BackupRuleDTO, error)
		Remove(ctx context.Context, id string) error
		ScanNow(ctx context.Context, id string) (*gui.TransferDTO, error)
		SetAllStopped(ctx context.Context, stopped bool) error
		PickFolder(ctx context.Context) (string, error)
		Files(ctx context.Context, ruleID string) ([]gui.TrackedFileDTO, error)
	}
	Downloads interface {
		ListDestinations(ctx context.Context) (*gui.RecoverySourcesDTO, error)
		StartDownload(ctx context.Context, in gui.DownloadInput) (*gui.TransferDTO, error)
		PickTargetDirectory(ctx context.Context) (string, error)
	}
	Settings interface {
		Get(ctx context.Context) (*gui.SettingsDTO, error)
		Update(ctx context.Context, in gui.SettingsInput) (*gui.SettingsDTO, error)
		Doctor(ctx context.Context) (*gui.DoctorReportDTO, error)
		DoctorWithWriteProbe(ctx context.Context, confirm bool) (*gui.DoctorReportDTO, error)
	}
}

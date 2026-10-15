package view

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/thedavidweng/rainforest/internal/gui"
	"github.com/thedavidweng/rainforest/locales"
)

var regionCodes = []string{"us", "ca", "uk", "jp", "de", "fr", "it", "es"}

func tr(c *ui.Context) locales.Messages {
	return locales.In(c.Locale())
}

func isTerminalStage(stage string) bool {
	switch stage {
	case "completed", "partial", "failed", "cancelled", "unconfirmed":
		return true
	}
	return false
}

func isRunningStage(stage string) bool {
	return stage == "running" || stage == "queued"
}

func transferStatus(m locales.Messages, kind, stage string) (string, tone) {
	switch stage {
	case "queued":
		return m.StageQueued(), toneNeutral
	case "running":
		if kind == "download" {
			return m.StageDownloading(), toneAccent
		}
		return m.StageUploading(), toneAccent
	case "paused":
		return m.StagePaused(), toneWarning
	case "completed":
		return m.StageCompleted(), toneSuccess
	case "partial":
		return m.StagePartial(), toneWarning
	case "failed":
		return m.StageFailed(), toneDanger
	case "cancelled":
		return m.StageCancelled(), toneNeutral
	case "unconfirmed":
		return m.StageUnconfirmed(), toneWarning
	}
	return stage, toneNeutral
}

func itemStatus(m locales.Messages, status string) (string, tone) {
	switch status {
	case "queued":
		return m.ItemQueued(), toneNeutral
	case "running":
		return m.ItemRunning(), toneAccent
	case "confirmed":
		return m.ItemConfirmed(), toneSuccess
	case "skipped":
		return m.ItemSkipped(), toneNeutral
	case "album_pending":
		return m.ItemAlbumPending(), toneWarning
	case "unconfirmed":
		return m.ItemUnconfirmed(), toneWarning
	case "failed":
		return m.ItemFailed(), toneDanger
	case "cancelled":
		return m.ItemCancelled(), toneNeutral
	}
	return status, toneNeutral
}

func ruleStatus(m locales.Messages, r gui.BackupRuleDTO, allStopped bool) (string, tone) {
	if r.Paused || (allStopped && r.Status != "paused") {
		return m.RulePaused(), toneNeutral
	}
	switch r.Status {
	case "active":
		return m.RuleActive(), toneSuccess
	case "scanning":
		return m.RuleScanning(), toneAccent
	case "paused":
		return m.RulePaused(), toneNeutral
	case "source_unavailable":
		return m.RuleUnavailable(), toneWarning
	case "error":
		return m.RuleError(), toneDanger
	}
	return r.Status, toneNeutral
}

func doctorStatus(m locales.Messages, status string) (string, *ui.SVG, tone) {
	switch status {
	case "pass":
		return m.DoctorPass(), iconCheckCircle, toneSuccess
	case "warn":
		return m.DoctorWarn(), iconWarning, toneWarning
	case "fail":
		return m.DoctorFail(), iconXCircle, toneDanger
	case "skipped":
		return m.DoctorSkipped(), iconCircleDashed, toneNeutral
	}
	return status, iconCircleDashed, toneNeutral
}

func destinationReadable(status string) bool {
	return status == "readable"
}

func capabilityStatus(m locales.Messages, status string) (string, tone) {
	if destinationReadable(status) {
		return m.DestReady(), toneSuccess
	}
	return m.DestUnavailable(), toneDanger
}

func probeStatus(m locales.Messages, st *probeState) (string, tone) {
	if st.state == loadIdle || st.state == loadRunning {
		return m.DestChecking(), toneAccent
	}
	if st.state == loadFailed {
		return m.DestUnavailable(), toneDanger
	}
	return capabilityStatus(m, st.capab.Status)
}

func kindLabel(m locales.Messages, kind string) string {
	switch kind {
	case "upload":
		return m.KindUpload()
	case "backup":
		return m.KindBackup()
	case "download":
		return m.KindDownload()
	}
	return kind
}

func kindIcon(kind string) *ui.SVG {
	switch kind {
	case "backup":
		return iconFolder
	case "download":
		return iconCloudArrowDown
	}
	return iconCloudArrowUp
}

func conflictLabel(m locales.Messages, policy string, forDownload bool) string {
	switch policy {
	case "skip":
		return m.ConflictSkip()
	case "rename":
		return m.ConflictRename()
	case "fail":
		if forDownload {
			return m.ConflictFailDownload()
		}
		return m.ConflictFailUpload()
	}
	return policy
}

func regionHost(region string) string {
	switch region {
	case "uk", "jp":
		return "amazon.co." + region
	case "":
		return "amazon.com"
	}
	if region == "us" {
		return "amazon.com"
	}
	return "amazon." + region
}

func regionName(m locales.Messages, region string) string {
	switch region {
	case "us":
		return m.RegionNameUs()
	case "ca":
		return m.RegionNameCa()
	case "uk":
		return m.RegionNameUk()
	case "jp":
		return m.RegionNameJp()
	case "de":
		return m.RegionNameDe()
	case "fr":
		return m.RegionNameFr()
	case "it":
		return m.RegionNameIt()
	case "es":
		return m.RegionNameEs()
	}
	return region
}

func regionLabel(m locales.Messages, region string) string {
	return regionName(m, region) + " · " + regionHost(region)
}

func regionCode(region string) string {
	if region == "" {
		return "US"
	}
	return strings.ToUpper(region)
}

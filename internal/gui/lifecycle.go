package gui

import (
	"fmt"
	"strings"
	"sync"

	"github.com/thedavidweng/rainforest/internal/store"
)

type TrayAction int

const (
	TrayActionNone TrayAction = iota
	TrayActionSeparator
	TrayActionShow
	TrayActionQuit
)

type TrayRow struct {
	Label  string     `json:"label"`
	Action TrayAction `json:"action"`
}

type TrayTracker struct {
	mu    sync.Mutex
	order []string
	byID  map[string]TransferDTO
}

func NewTrayTracker() *TrayTracker {
	return &TrayTracker{
		byID: make(map[string]TransferDTO),
	}
}

func (t *TrayTracker) Apply(tr TransferDTO) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if store.IsTerminalStage(store.TransferStage(tr.Stage)) {
		t.removeLocked(tr.ID)
		return
	}
	if _, known := t.byID[tr.ID]; !known {
		t.order = append(t.order, tr.ID)
	}
	t.byID[tr.ID] = tr
}

func (t *TrayTracker) Remove(id string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.removeLocked(id)
}

func (t *TrayTracker) removeLocked(id string) {
	if _, known := t.byID[id]; !known {
		return
	}
	delete(t.byID, id)
	for i, other := range t.order {
		if other == id {
			t.order = append(t.order[:i], t.order[i+1:]...)
			return
		}
	}
}

func (t *TrayTracker) ActiveCount() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.order)
}

func (t *TrayTracker) Menu() []TrayRow {
	t.mu.Lock()
	defer t.mu.Unlock()
	rows := make([]TrayRow, 0, len(t.order)+4)
	if len(t.order) == 0 {
		rows = append(rows, TrayRow{Label: "No active transfers", Action: TrayActionNone})
	}
	for _, id := range t.order {
		rows = append(rows, TrayRow{Label: trayTransferLabel(t.byID[id]), Action: TrayActionNone})
	}
	rows = append(rows,
		TrayRow{Action: TrayActionSeparator},
		TrayRow{Label: "Show Rainforest", Action: TrayActionShow},
		TrayRow{Label: "Quit Rainforest", Action: TrayActionQuit},
	)
	return rows
}

func trayTransferLabel(tr TransferDTO) string {
	arrow := "↑"
	if tr.Kind == string(store.TransferKindDownload) {
		arrow = "↓"
	}
	summary := strings.TrimSpace(tr.SourceSummary)
	if summary == "" {
		summary = tr.ID
	}
	label := fmt.Sprintf("%s %s · %s", arrow, summary, tr.Stage)
	if tr.BytesTotal > 0 {
		pct := (tr.BytesDone*100 + tr.BytesTotal/2) / tr.BytesTotal
		label += fmt.Sprintf(" · %d%%", pct)
	} else if tr.ItemsTotal > 0 {
		label += fmt.Sprintf(" · %d/%d", tr.ItemsDone, tr.ItemsTotal)
	}
	return label
}

type QuitDecision int

const (
	QuitImmediately QuitDecision = iota
	QuitAsk
)

func DecideQuit(activeTransfers, activeBackups int, closeToTray bool) QuitDecision {
	if activeTransfers > 0 {
		return QuitAsk
	}
	return QuitImmediately
}

func QuitMessage(activeTransfers, activeBackups int, closeToTray bool) string {
	var head string
	switch {
	case activeTransfers == 1 && activeBackups > 0:
		head = fmt.Sprintf("1 transfer is currently running and %d backup folder rule(s) are enabled.", activeBackups)
	case activeTransfers > 1 && activeBackups > 0:
		head = fmt.Sprintf("%d transfers are currently running and %d backup folder rule(s) are enabled.", activeTransfers, activeBackups)
	case activeTransfers == 1:
		head = "1 transfer is currently running."
	case activeTransfers > 1:
		head = fmt.Sprintf("%d transfers are currently running.", activeTransfers)
	case activeBackups > 0:
		head = fmt.Sprintf("%d backup folder rule(s) are enabled.", activeBackups)
	default:
		head = "No transfers are currently running."
	}

	trayNote := "Close Window to Tray keeps Rainforest running in the system tray."
	if !closeToTray {
		trayNote = "Close Window to Tray can be enabled in Settings to keep Rainforest running in the background."
	}

	return fmt.Sprintf(
		"%s Choose Pause Task to pause a single transfer, Stop Backups to suspend background folder scans, %s, or Quit Rainforest to exit the application and interrupt active transfers.",
		head,
		trayNote,
	)
}

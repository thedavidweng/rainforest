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
	QuitPromptConfirm = QuitAsk
)

func DecideQuit(activeTransfers, activeBackups int, closeToTray bool) QuitDecision {
	if activeTransfers > 0 || (activeBackups > 0 && !closeToTray) {
		return QuitPromptConfirm
	}
	return QuitImmediately
}

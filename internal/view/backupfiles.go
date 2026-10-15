package view

import (
	"context"
	"path"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/thedavidweng/rainforest/internal/gui"
	"github.com/thedavidweng/rainforest/locales"
)

const (
	sizeColumn   = 58
	syncedColumn = 74
)

type fileNode struct {
	name     string
	dir      bool
	size     int64
	count    int
	status   string
	synced   string
	children []string
}

type fileTree struct {
	roots []string
	nodes map[string]*fileNode
}

type filesState struct {
	state   loadState
	files   []gui.TrackedFileDTO
	tree    fileTree
	err     string
	outline ui.OutlineState[string]
	opened  bool
}

func buildFileTree(files []gui.TrackedFileDTO) fileTree {
	t := fileTree{nodes: map[string]*fileNode{}}
	link := func(parent, key string) {
		if parent == "" {
			t.roots = append(t.roots, key)
			return
		}
		p := t.nodes[parent]
		p.children = append(p.children, key)
	}
	for _, f := range files {
		rel := path.Clean(strings.ReplaceAll(f.RelPath, "\\", "/"))
		parts := strings.Split(rel, "/")
		parent := ""
		for i, part := range parts {
			key := strings.Join(parts[:i+1], "/")
			n := t.nodes[key]
			if n == nil {
				n = &fileNode{name: part, dir: i < len(parts)-1}
				if n.dir {
					n.children = []string{}
				}
				t.nodes[key] = n
				link(parent, key)
			}
			if n.dir {
				n.size += f.Size
				n.count++
			}
			parent = key
		}
		leaf := t.nodes[rel]
		leaf.size, leaf.status, leaf.synced = f.Size, f.Status, f.LastSyncedAt
	}
	t.sortKeys(t.roots)
	for _, n := range t.nodes {
		t.sortKeys(n.children)
	}
	return t
}

func (t fileTree) sortKeys(keys []string) {
	sort.SliceStable(keys, func(i, j int) bool {
		a, b := t.nodes[keys[i]], t.nodes[keys[j]]
		if a.dir != b.dir {
			return a.dir
		}
		return strings.ToLower(a.name) < strings.ToLower(b.name)
	})
}

func (t fileTree) children(key string) []string {
	n := t.nodes[key]
	if n == nil || !n.dir {
		return nil
	}
	return n.children
}

func (a *App) filesFor(ruleID string) *filesState {
	st := a.files[ruleID]
	if st == nil {
		st = &filesState{}
		a.files[ruleID] = st
	}
	if st.state == loadIdle {
		a.loadFiles(ruleID)
	}
	return st
}

func (a *App) loadFiles(ruleID string) {
	st := a.files[ruleID]
	if st == nil {
		st = &filesState{}
		a.files[ruleID] = st
	}
	st.state = loadRunning
	a.do(func(ctx context.Context) func() {
		files, err := a.svc.Backups.Files(ctx, ruleID)
		return func() {
			if err != nil {
				st.state, st.err = loadFailed, errMessage(err)
				return
			}
			st.state, st.files, st.err = loadDone, files, ""
			st.tree = buildFileTree(files)
			if !st.opened {
				st.opened = true
				for _, key := range st.tree.roots {
					if st.tree.nodes[key].dir {
						st.outline.Open.Add(key)
					}
				}
			}
		}
	})
}

func (a *App) staleFiles(ruleID string) {
	if st := a.files[ruleID]; st != nil && st.state != loadRunning {
		st.state = loadIdle
	}
}

func fileStatusIcon(status string) (*ui.SVG, tone) {
	switch status {
	case "confirmed":
		return iconCheckCircle, toneSuccess
	case "failed":
		return iconXCircle, toneDanger
	case "unconfirmed", "album_pending":
		return iconWarning, toneWarning
	case "running":
		return iconCircleNotch, toneAccent
	case "queued":
		return iconClock, toneNeutral
	}
	return iconCircleDashed, toneNeutral
}

func (a *App) filesPanel(c *ui.Context, m locales.Messages, rule gui.BackupRuleDTO) {
	t := c.Theme()
	st := a.filesFor(rule.ID)
	sectionTitle(c, m.BackupFilesTitle(), func() {
		if st.state == loadDone && len(st.files) > 0 {
			ui.Text(c, m.BackupFileCount(len(st.files))).FontSize(12).TextColor(t.TextMuted).Margin(4, 0, 0)
		}
	})
	switch {
	case st.state == loadFailed:
		banner(c, toneDanger, iconWarning, "", st.err, "", nil)
	case st.state != loadDone && st.files == nil:
		ui.Row(c).Justify(ui.Center).Padding(32, 0).Children(func() {
			ui.Spinner(c).Label(m.CommonLoading())
		})
	case len(st.files) == 0:
		emptyState(c, iconFolderOpen, m.BackupFilesEmptyTitle(), m.BackupFilesEmptyBody(), "", nil)
	default:
		a.fileTreeView(c, m, st)
	}
}

func (a *App) fileTreeView(c *ui.Context, m locales.Messages, st *filesState) {
	t := c.Theme()
	tree := st.tree
	ui.Outline(c, &st.outline, tree.roots, tree.children, func(key string) {
		n := tree.nodes[key]
		if n.dir {
			ui.Icon(c, iconFolder).FontSize(18).TextColor(t.TextMuted)
			ui.Text(c, n.name).FontWeight(600).Grow(1).MinWidth(0).SingleLine().Tooltip(n.name + " · " + m.BackupFileCount(n.count))
			ui.Text(c, formatBytes(n.size)).FontSize(12).TextColor(t.TextMuted).SingleLine().Width(sizeColumn).TextAlign(ui.End)
			ui.Box(c).Width(syncedColumn).Shrink(0)
			return
		}
		icon, tn := fileStatusIcon(n.status)
		label, _ := itemStatus(m, n.status)
		ui.Icon(c, icon).FontSize(16).TextColor(toneColor(c, tn)).Tooltip(label)
		ui.Text(c, n.name).Grow(1).MinWidth(0).SingleLine().Tooltip(key)
		ui.Text(c, formatBytes(n.size)).FontSize(12).TextColor(t.TextMuted).SingleLine().Width(sizeColumn).TextAlign(ui.End)
		when := ""
		if n.synced != "" {
			when = relativeTime(c, n.synced)
		}
		ui.Text(c, when).FontSize(12).TextColor(t.TextMuted).SingleLine().Width(syncedColumn).TextAlign(ui.End)
	}).Grow(1).MinHeight(160).Border(1, t.Border).Radius(radiusField)
}

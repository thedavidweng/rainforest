package engine

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/thedavidweng/amazon-photos/pkg/aphotos"
	"github.com/thedavidweng/rainforest/internal/store"
)

const (
	folderListingCap  = 1000
	folderListingPage = 200
	maxBreadcrumbs    = 32
)

type FolderRef struct {
	ID   string
	Name string
}

type FolderListing struct {
	Folder    FolderRef
	Path      []FolderRef
	Children  []FolderRef
	Truncated bool
}

func (e *Engine) ListFolders(ctx context.Context, parentID string) (*FolderListing, error) {
	client := e.Client()
	if client == nil {
		return nil, ErrOffline
	}
	var node *aphotos.Node
	var err error
	if id := strings.TrimSpace(parentID); id == "" {
		node, err = client.GetRoot(ctx)
	} else {
		node, err = client.GetNode(ctx, id)
	}
	if err != nil {
		return nil, err
	}
	if node.Kind != aphotos.NodeKindFolder || node.Status != aphotos.NodeStatusAvailable {
		return nil, fmt.Errorf("node %s is not an available folder", node.ID)
	}
	path, err := folderBreadcrumbs(ctx, client, node)
	if err != nil {
		return nil, err
	}
	children, truncated, err := listChildFolders(ctx, client, node.ID)
	if err != nil {
		return nil, err
	}
	return &FolderListing{
		Folder:    path[len(path)-1],
		Path:      path,
		Children:  children,
		Truncated: truncated,
	}, nil
}

func folderBreadcrumbs(ctx context.Context, client *aphotos.Client, node *aphotos.Node) ([]FolderRef, error) {
	var reversed []FolderRef
	seen := map[string]bool{}
	for cur := node; cur != nil; {
		seen[cur.ID] = true
		name := cur.Name
		if cur.IsRoot {
			name = ""
		}
		reversed = append(reversed, FolderRef{ID: cur.ID, Name: name})
		if cur.IsRoot || len(cur.Parents) == 0 || len(reversed) >= maxBreadcrumbs || seen[cur.Parents[0]] {
			break
		}
		parent, err := client.GetNode(ctx, cur.Parents[0])
		if err != nil {
			return nil, err
		}
		cur = parent
	}
	path := make([]FolderRef, len(reversed))
	for i, ref := range reversed {
		path[len(reversed)-1-i] = ref
	}
	return path, nil
}

func listChildFolders(ctx context.Context, client *aphotos.Client, parentID string) ([]FolderRef, bool, error) {
	var out []FolderRef
	lastPageFull := false
	for offset := 0; offset < folderListingCap; offset += folderListingPage {
		page, err := client.ListChildren(ctx, parentID, aphotos.ChildrenOptions{FoldersOnly: true, Limit: folderListingPage, Offset: offset})
		if err != nil {
			return nil, false, err
		}
		for i := range page.Items {
			n := &page.Items[i]
			if n.Kind == aphotos.NodeKindFolder && n.Status == aphotos.NodeStatusAvailable && n.ID != "" {
				out = append(out, FolderRef{ID: n.ID, Name: n.Name})
			}
		}
		lastPageFull = len(page.Items) >= folderListingPage
		if !lastPageFull && !page.HasMore {
			break
		}
	}
	truncated := false
	if lastPageFull {
		extra, err := client.ListChildren(ctx, parentID, aphotos.ChildrenOptions{FoldersOnly: true, Limit: 1, Offset: folderListingCap})
		if err != nil {
			return nil, false, err
		}
		truncated = len(extra.Items) > 0
	}
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, truncated, nil
}

type RecentUpload struct {
	Item      store.TransferItem
	Thumbnail []byte
}

const maxRecentUploads = 24

func (e *Engine) RecentUploads(ctx context.Context, limit int) ([]RecentUpload, error) {
	st := e.Store()
	if st == nil {
		return nil, nil
	}
	limit = min(max(limit, 1), maxRecentUploads)
	items, err := st.ListConfirmedItems(ctx, limit*4)
	if err != nil {
		return nil, err
	}
	out := make([]RecentUpload, 0, min(limit, len(items)))
	for i := range items {
		if len(out) == limit {
			break
		}
		r := RecentUpload{Item: items[i]}
		if items[i].MediaType == store.MediaTypePhoto && items[i].LocalPath != "" {
			if t, tErr := e.LocalThumbnail(items[i].LocalPath); tErr == nil {
				r.Thumbnail = t.Data
			}
		}
		out = append(out, r)
	}
	return out, nil
}

func (e *Engine) LastUploadAt(ctx context.Context) (*time.Time, error) {
	st := e.Store()
	if st == nil {
		return nil, nil
	}
	items, err := st.ListConfirmedItems(ctx, 1)
	if err != nil || len(items) == 0 {
		return nil, err
	}
	t := items[0].UpdatedAt
	return &t, nil
}

func (e *Engine) TrackedFiles(ctx context.Context, ruleID string) ([]store.TrackedFile, error) {
	st := e.Store()
	if st == nil {
		return nil, nil
	}
	if _, err := st.GetBackupRule(ctx, ruleID); err != nil {
		return nil, err
	}
	return st.ListTrackedFiles(ctx, ruleID)
}

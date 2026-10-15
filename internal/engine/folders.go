package engine

import (
	"context"
	"fmt"
	"path"
	"strings"
	"sync"

	"github.com/thedavidweng/amazon-photos/pkg/aphotos"
	"github.com/thedavidweng/rainforest/internal/store"
)

type folderResolver struct {
	client *aphotos.Client
	base   string
	mu     sync.Mutex
	cache  map[string]string
}

func newFolderResolver(client *aphotos.Client, base string) *folderResolver {
	return &folderResolver{client: client, base: base, cache: map[string]string{"": base}}
}

func splitRemoteDir(dir string) []string {
	var parts []string
	for _, p := range strings.Split(dir, "/") {
		if p != "" && p != "." && p != ".." {
			parts = append(parts, p)
		}
	}
	return parts
}

func (r *folderResolver) resolve(ctx context.Context, dir string, create bool) (id string, found bool, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	parentID := r.base
	built := ""
	for _, name := range splitRemoteDir(dir) {
		built = path.Join(built, name)
		if id, ok := r.cache[built]; ok {
			parentID = id
			continue
		}
		child, fErr := r.findOrCreate(ctx, name, parentID, create)
		if fErr != nil || child == "" {
			return "", false, fErr
		}
		r.cache[built] = child
		parentID = child
	}
	return parentID, true, nil
}

func (r *folderResolver) findOrCreate(ctx context.Context, name, parentID string, create bool) (string, error) {
	id, err := r.findFolder(ctx, name, parentID)
	if err != nil || id != "" || !create {
		return id, err
	}
	node, cErr := r.client.CreateFolder(ctx, name, parentID)
	if cErr == nil && node != nil && strings.TrimSpace(node.ID) != "" {
		return node.ID, nil
	}
	if id, err = r.findFolder(ctx, name, parentID); err == nil && id != "" {
		return id, nil
	}
	if cErr == nil {
		cErr = fmt.Errorf("create folder %q returned no node", name)
	}
	return "", cErr
}

func (r *folderResolver) findFolder(ctx context.Context, name, parentID string) (string, error) {
	page, err := r.client.QueryNodes(ctx, aphotos.NodeQueryOptions{
		Filter: aphotos.NewFilter().Kind(aphotos.NodeKindFolder).Status(aphotos.NodeStatusAvailable).NameExact(name).ParentID(parentID),
		Limit:  10,
	})
	if err != nil {
		return "", err
	}
	for i := range page.Items {
		n := &page.Items[i]
		if n.Name == name && n.Kind == aphotos.NodeKindFolder && n.Status == aphotos.NodeStatusAvailable && strings.TrimSpace(n.ID) != "" && hasParent(n, parentID) {
			return n.ID, nil
		}
	}
	return "", nil
}

func hasParent(n *aphotos.Node, parentID string) bool {
	if len(n.Parents) == 0 {
		return true
	}
	for _, p := range n.Parents {
		if p == parentID {
			return true
		}
	}
	return false
}

func (d DestinationCapability) uploadParent(ctx context.Context, remoteDir string, create bool) (id string, found bool, err error) {
	if remoteDir == "" || !d.StructureSupported || d.folders == nil {
		return d.parentID, true, nil
	}
	return d.folders.resolve(ctx, remoteDir, create)
}

func remoteDirFromLocalRel(rel string) string {
	dir := path.Dir(rel)
	if dir == "." || dir == "/" {
		return ""
	}
	return dir
}

func findExistingInDestination(ctx context.Context, client *aphotos.Client, d DestinationCapability, item store.TransferItem) (*aphotos.Node, error) {
	parentID, found, err := d.uploadParent(ctx, item.RemoteDir, false)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("cannot reconcile upload: destination folder %q is missing", item.RemoteDir)
	}
	return findRemoteNodeForItem(ctx, client, item, parentID)
}

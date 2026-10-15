package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/thedavidweng/rainforest/internal/store"

	"github.com/thedavidweng/amazon-photos/pkg/aphotos"
)

type DestinationCapability struct {
	Region            aphotos.Region         `json:"region"`
	Evidence          aphotos.EvidenceStatus `json:"evidence"`
	DestinationNodeID string                 `json:"destinationNodeId"`
	Name              string                 `json:"name"`
	Kind              string                 `json:"kind"`
	Status            string                 `json:"status"`
	Message           string                 `json:"message"`
	rootID            string
	parentID          string
	err               error
}

func (e *Engine) ProbeDestination(ctx context.Context, id string) DestinationCapability {
	return e.probeDestination(ctx, e.Client(), id)
}

func (e *Engine) probeDestination(ctx context.Context, client *aphotos.Client, id string) DestinationCapability {
	d := DestinationCapability{DestinationNodeID: strings.TrimSpace(id), Status: "pending", Message: "Credentials unavailable; authenticate before uploading."}
	if client == nil {
		if st := e.Store(); st != nil {
			if cfg, err := st.GetSettings(ctx); err == nil {
				if rc, err := aphotos.ResolveRegion(aphotos.Region(cfg.Region)); err == nil {
					d.Region = rc.Region
					d.Evidence = rc.Evidence
				}
			}
		}
		d.err = ErrOffline
		return d
	}
	rc := client.RegionConfig()
	d.Region = rc.Region
	d.Evidence = rc.Evidence
	var node *aphotos.Node
	var err error
	if d.DestinationNodeID != "" {
		node, err = client.GetNode(ctx, d.DestinationNodeID)
	} else {
		node, err = client.GetRoot(ctx)
	}
	if node != nil {
		d.Name = node.Name
		d.Kind = string(node.Kind)
	}
	if err == nil {
		switch {
		case node == nil:
			err = errors.New("destination response is empty")
		case d.DestinationNodeID != "" && node.ID != d.DestinationNodeID:
			err = errors.New("destination Node ID does not match the requested ID")
		case node.Status != aphotos.NodeStatusAvailable:
			err = errors.New("destination must be AVAILABLE")
		case node.Kind != aphotos.NodeKindFolder && node.Kind != aphotos.NodeKindVisualCollection:
			err = fmt.Errorf("unsupported destination type %q; use FOLDER or VISUAL_COLLECTION", node.Kind)
		}
	}
	if err == nil {
		root := node
		if d.DestinationNodeID != "" {
			root, err = client.GetRoot(ctx)
		}
		if err == nil && (root.ID == "" || root.Kind != aphotos.NodeKindFolder || root.Status != aphotos.NodeStatusAvailable) {
			err = errors.New("account root must be an AVAILABLE FOLDER")
		}
		if err == nil {
			d.rootID = root.ID
			d.parentID = root.ID
			if node.Kind == aphotos.NodeKindFolder {
				d.parentID = node.ID
			}
			d.Status = "readable"
			d.Message = "Target is readable. Upload and album membership are confirmed separately."
			if d.Name == "" && d.DestinationNodeID == "" {
				d.Name = "Account root"
			}
			return d
		}
	}
	d.Status = "failed"
	d.err = err
	d.Message = formatErrMsg(err)
	var auth *aphotos.AuthenticationError
	var permission *aphotos.PermissionError
	var missing *aphotos.NotFoundError
	var rate *aphotos.RateLimitError
	var network *aphotos.TransientNetworkError
	switch {
	case errors.As(err, &auth):
		d.Message = "Re-authenticate: account credentials expired or invalid (401)."
	case errors.As(err, &permission):
		d.Message = "No permission to access this destination (403)."
	case errors.As(err, &missing):
		d.Message = "Destination does not exist (404)."
	case errors.As(err, &rate), errors.As(err, &network), errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		d.Status = "pending"
		d.Message = "Temporary destination probe failure; retry when connected. " + formatErrMsg(err)
	}
	return d
}

func (d DestinationCapability) requireReadable() error {
	if d.Status != "readable" {
		return fmt.Errorf("destination %s: %s: %w", d.Status, d.Message, d.err)
	}
	return nil
}

func (e *Engine) bindDestination(ctx context.Context, key string, d DestinationCapability) error {
	return e.Store().BindDestination(ctx, key, string(d.Region)+":"+d.rootID+":"+d.DestinationNodeID)
}

func (e *Engine) associateAlbum(ctx context.Context, st *store.Store, client *aphotos.Client, d DestinationCapability, item *store.TransferItem) {
	item.Status = store.ItemStatusAlbumPending
	item.ErrorMessage = "Media uploaded; album membership pending. Retry only associates the saved media Node ID."
	item.UpdatedAt = time.Now().UTC()
	if err := st.UpdateTransferItem(context.Background(), *item); err != nil {
		item.ErrorMessage = formatErrMsg(err)
		return
	}
	if client != e.Client() {
		return
	}
	if err := client.AddAlbumMembers(ctx, d.DestinationNodeID, []string{item.RemoteNodeID}); err != nil {
		item.ErrorMessage = "Media uploaded; album association failed: " + formatErrMsg(err)
	} else {
		item.Status = store.ItemStatusConfirmed
		item.ErrorMessage = ""
	}
	item.UpdatedAt = time.Now().UTC()
	if err := st.UpdateTransferItem(context.Background(), *item); err != nil {
		return
	}
	if tr, err := recomputeAndPersistTransfer(context.Background(), st, item.TransferID, ""); err == nil {
		e.emitTransferProgress(tr, *item)
	}
}

func reconciledStatus(d DestinationCapability) store.ItemStatus {
	if d.Kind == string(aphotos.NodeKindVisualCollection) {
		return store.ItemStatusAlbumPending
	}
	return store.ItemStatusConfirmed
}

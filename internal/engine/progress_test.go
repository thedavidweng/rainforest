package engine

import (
	"sync"
	"testing"
	"time"

	"github.com/thedavidweng/rainforest/internal/store"
)

func TestLiveByteProgressKeepsCompletedTotals(t *testing.T) {
	eng := New(Config{})
	var mu sync.Mutex
	var got []store.Transfer
	eng.SetObserver(Observer{
		OnTransferProgress: func(tr store.Transfer, _ store.TransferItem) {
			mu.Lock()
			got = append(got, tr)
			mu.Unlock()
		},
	})

	stale := store.Transfer{
		ID: "batch", Stage: store.StageRunning,
		BytesTotal: 300, ItemsTotal: 3,
		UpdatedAt: time.Now().UTC(),
	}
	eng.emitLiveByteProgress(stale, store.TransferItem{ID: "b", TransferID: "batch", BytesDone: 40, Status: store.ItemStatusRunning})
	eng.emitLiveByteProgress(stale, store.TransferItem{ID: "c", TransferID: "batch", BytesDone: 25, Status: store.ItemStatusRunning})

	done := stale
	done.ItemsDone = 1
	done.BytesDone = 100
	done.UpdatedAt = done.UpdatedAt.Add(time.Second)
	eng.emitTransferProgress(done, store.TransferItem{ID: "b", TransferID: "batch", BytesDone: 100, Status: store.ItemStatusConfirmed})

	eng.emitLiveByteProgress(stale, store.TransferItem{ID: "c", TransferID: "batch", BytesDone: 30, Status: store.ItemStatusRunning})

	mu.Lock()
	defer mu.Unlock()
	if len(got) != 4 {
		t.Fatalf("progress events = %d, want 4", len(got))
	}
	if got[0].BytesDone != 40 || got[0].ItemsDone != 0 {
		t.Fatalf("first tick = %+v, want 40 bytes and 0 items", got[0])
	}
	if got[1].BytesDone != 65 || got[1].ItemsDone != 0 {
		t.Fatalf("second tick = %+v, want 65 bytes from both in-flight files", got[1])
	}
	if got[2].BytesDone != 125 || got[2].ItemsDone != 1 {
		t.Fatalf("after confirm = %+v, want 125 bytes and 1 item", got[2])
	}
	if got[3].BytesDone != 130 || got[3].ItemsDone != 1 || got[3].BytesTotal != 300 {
		t.Fatalf("later tick = %+v, want completed totals kept and in-flight bytes added", got[3])
	}
}

func TestLiveByteProgressStopsAfterTerminalStage(t *testing.T) {
	eng := New(Config{})
	var n int
	eng.SetObserver(Observer{
		OnTransferProgress: func(store.Transfer, store.TransferItem) { n++ },
	})
	final := store.Transfer{
		ID: "batch", Stage: store.StageCompleted,
		BytesDone: 300, BytesTotal: 300, ItemsDone: 3, ItemsTotal: 3,
		UpdatedAt: time.Now().UTC(),
	}
	eng.emitTransferStage(final)
	eng.emitLiveByteProgress(store.Transfer{
		ID: "batch", Stage: store.StageRunning, BytesTotal: 300, ItemsTotal: 3,
	}, store.TransferItem{ID: "c", BytesDone: 10, Status: store.ItemStatusRunning})
	if n != 0 {
		t.Fatalf("progress events after completion = %d, want 0", n)
	}
}

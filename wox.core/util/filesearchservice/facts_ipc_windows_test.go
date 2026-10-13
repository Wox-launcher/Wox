//go:build windows

package filesearchservice

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"wox/util/filesearchservice/protocol"
)

// runFactTestServer uses the actual ACL-protected pipe without privileged volume access.
func runFactTestServer(t *testing.T, handler factRequestHandler) (string, context.CancelFunc) {
	t.Helper()
	name := fmt.Sprintf(`\\.\pipe\WoxFactsTest-%d-%s`, os.Getpid(), t.Name())
	sid, err := currentUserSID()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serveFactsPipe(ctx, name, sid, handler) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil && !errors.Is(err, context.Canceled) {
				t.Error(err)
			}
		case <-time.After(3 * time.Second):
			t.Error("fact pipe server did not stop")
		}
	})
	return name, cancel
}

func TestFactsPipeStreamsBatches(t *testing.T) {
	boundary := protocol.Boundary{Volume: `C:\`, RootID: 5, JournalID: 9, NextUSN: 300}
	name, _ := runFactTestServer(t, func(ctx context.Context, kind protocol.Kind, got protocol.Boundary, emit FactSink) error {
		if kind != protocol.Enumerate || got != boundary {
			return fmt.Errorf("wrong request: %v %v", kind, got)
		}
		payload, _ := protocol.EncodeBoundary(boundary)
		if err := emit(protocol.Begin, payload); err != nil {
			return err
		}
		var batch []byte
		for i := range 12000 {
			var err error
			batch, err = protocol.AppendEntry(batch, protocol.Entry{ID: uint64(i + 10), ParentID: 5, Name: "项目报告.txt"})
			if err != nil {
				return err
			}
			if len(batch) >= protocol.TargetBatchSize {
				if err := emit(protocol.Records, batch); err != nil {
					return err
				}
				batch = batch[:0]
			}
		}
		if err := emit(protocol.Records, batch); err != nil {
			return err
		}
		return emit(protocol.End, payload)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	count := 0
	err := streamFactsPipe(ctx, name, protocol.Enumerate, boundary, func(kind protocol.Kind, payload []byte) error {
		if kind == protocol.Records {
			return protocol.VisitEntries(payload, func(entry protocol.Entry) error {
				if entry.Name != "项目报告.txt" || entry.ID != uint64(count+10) {
					return fmt.Errorf("wrong entry: %+v", entry)
				}
				count++
				return nil
			})
		}
		got, err := protocol.DecodeBoundary(payload)
		if err == nil && got != boundary {
			return errors.New("wrong boundary")
		}
		return err
	})
	if err != nil || count != 12000 {
		t.Fatalf("count=%d: %v", count, err)
	}
}

func TestFactsPipeCancelsIdleSubscriptionOnDisconnect(t *testing.T) {
	started := make(chan struct{})
	stopped := make(chan struct{})
	name, _ := runFactTestServer(t, func(ctx context.Context, _ protocol.Kind, _ protocol.Boundary, _ FactSink) error {
		close(started)
		<-ctx.Done()
		close(stopped)
		return ctx.Err()
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- streamFactsPipe(ctx, name, protocol.Subscribe, protocol.Boundary{Volume: `C:\`}, nil) }()
	select {
	case <-started:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled client: %v", err)
	}
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("disconnected client left a journal reader running")
	}
}

func TestFactsPipeCheckpointAndReset(t *testing.T) {
	name, _ := runFactTestServer(t, func(ctx context.Context, _ protocol.Kind, boundary protocol.Boundary, emit FactSink) error {
		if boundary.JournalID != 10 {
			return emit(protocol.ResetRequired, nil)
		}
		boundary.NextUSN++
		payload, _ := protocol.EncodeBoundary(boundary)
		return emit(protocol.Checkpoint, payload)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	boundary := protocol.Boundary{Volume: `C:\`, JournalID: 10, NextUSN: 20}
	var got protocol.Boundary
	if err := streamFactsPipe(ctx, name, protocol.Replay, boundary, func(kind protocol.Kind, payload []byte) error {
		var err error
		got, err = protocol.DecodeBoundary(payload)
		return err
	}); err != nil || got.NextUSN != 21 {
		t.Fatalf("checkpoint=%+v: %v", got, err)
	}
	boundary.JournalID = 9
	if err := streamFactsPipe(ctx, name, protocol.Replay, boundary, nil); !errors.Is(err, ErrFactsReset) {
		t.Fatalf("reset=%v", err)
	}
}

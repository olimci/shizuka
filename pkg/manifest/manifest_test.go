package manifest

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/options"
)

func TestBuildSkipsOutputCleanupWhenRequested(t *testing.T) {
	t.Parallel()

	out := t.TempDir()
	stale := filepath.Join(out, "stale.txt")
	if err := os.WriteFile(stale, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale file: %v", err)
	}

	man := New()
	man.Emit(TextArtefact(Claim{Owner: "test", Target: "index.html"}, "hello"))

	opts := options.DefaultOptions()
	opts.SkipOutputCleanup = true

	if err := man.Begin(context.Background(), config.DefaultConfig(), opts, nil, out); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}
	if err := man.Complete(true); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if _, err := os.Stat(stale); err != nil {
		t.Fatalf("expected stale file to remain, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "index.html")); err != nil {
		t.Fatalf("expected output file: %v", err)
	}
}

func TestBuildCleansOutputWhenEnabled(t *testing.T) {
	t.Parallel()

	out := t.TempDir()
	stale := filepath.Join(out, "stale.txt")
	if err := os.WriteFile(stale, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale file: %v", err)
	}

	man := New()
	man.Emit(TextArtefact(Claim{Owner: "test", Target: "index.html"}, "hello"))

	opts := options.DefaultOptions()

	if err := man.Begin(context.Background(), config.DefaultConfig(), opts, nil, out); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}
	if err := man.Complete(true); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Fatalf("expected stale file removed, got %v", err)
	}
}

func TestBeginWritesArtefactsEagerly(t *testing.T) {
	t.Parallel()

	out := t.TempDir()
	man := New()

	opts := options.DefaultOptions()

	if err := man.Begin(context.Background(), config.DefaultConfig(), opts, nil, out); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	man.Emit(Artefact{
		Claim: Claim{Owner: "test", Target: "index.html"},
		Builder: func(w io.Writer) error {
			_, err := w.Write([]byte("hello"))
			return err
		},
	})

	target := filepath.Join(out, "index.html")
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(target); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected eager output %q to be written before Complete", target)
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := man.Complete(true); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
}

func TestEmitDoesNotBlockWhenArtefactWorkersAreCongested(t *testing.T) {
	t.Parallel()

	out := t.TempDir()
	man := New()

	opts := options.DefaultOptions()
	opts.MaxWorkers = 1
	opts.ArtefactWorkers = 1

	if err := man.Begin(context.Background(), config.DefaultConfig(), opts, nil, out); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	writerStarted := make(chan struct{})
	releaseWriter := make(chan struct{})
	man.Emit(Artefact{
		Claim: Claim{Owner: "test", Target: "blocked.txt"},
		Builder: func(w io.Writer) error {
			close(writerStarted)
			<-releaseWriter
			_, err := w.Write([]byte("blocked"))
			return err
		},
	})

	select {
	case <-writerStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("writer did not start")
	}

	emitted := make(chan struct{})
	go func() {
		defer close(emitted)
		for i := range 16 {
			man.Emit(TextArtefact(
				Claim{Owner: "test", Target: "queued-" + strconv.Itoa(i) + ".txt"},
				"queued",
			))
		}
	}()

	select {
	case <-emitted:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Emit blocked behind congested artefact workers")
	}

	close(releaseWriter)
	if err := man.Complete(true); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
}

func TestZeroArtefactWorkersDrainOnComplete(t *testing.T) {
	t.Parallel()

	out := t.TempDir()
	man := New()

	opts := options.DefaultOptions()
	opts.MaxWorkers = 2
	opts.ArtefactWorkers = 0

	if err := man.Begin(context.Background(), config.DefaultConfig(), opts, nil, out); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	man.Emit(TextArtefact(Claim{Owner: "test", Target: "index.html"}, "hello"))

	if _, err := os.Stat(filepath.Join(out, "index.html")); !os.IsNotExist(err) {
		t.Fatalf("expected no eager output before Complete, got %v", err)
	}

	if err := man.Complete(true); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "index.html")); err != nil {
		t.Fatalf("expected output after Complete: %v", err)
	}
}

func TestPositiveArtefactWorkersWriteEagerly(t *testing.T) {
	t.Parallel()

	out := t.TempDir()
	man := New()

	opts := options.DefaultOptions()
	opts.ArtefactWorkers = 1

	if err := man.Begin(context.Background(), config.DefaultConfig(), opts, nil, out); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	man.Emit(TextArtefact(Claim{Owner: "test", Target: "index.html"}, "hello"))

	target := filepath.Join(out, "index.html")
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(target); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("expected eager output %q to be written before Complete", target)
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := man.Complete(true); err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
}

func TestDefaultArtefactWorkers(t *testing.T) {
	t.Parallel()

	tests := map[int]int{
		0:  1,
		1:  1,
		2:  1,
		4:  1,
		8:  2,
		16: 2,
	}

	for workers, want := range tests {
		if got := defaultArtefactWorkers(workers); got != want {
			t.Fatalf("defaultArtefactWorkers(%d) = %d, want %d", workers, got, want)
		}
	}
}

func TestCompleteAddsStepWorkersToManifestDrain(t *testing.T) {
	t.Parallel()

	out := t.TempDir()
	man := New()

	opts := options.DefaultOptions()
	opts.MaxWorkers = 2
	opts.ArtefactWorkers = 1

	if err := man.Begin(context.Background(), config.DefaultConfig(), opts, nil, out); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	writerStarted := make(chan struct{})
	releaseWriters := make(chan struct{})
	started := make(chan string, 4)

	man.Emit(Artefact{
		Claim: Claim{Owner: "test", Target: "blocked.txt"},
		Builder: func(w io.Writer) error {
			close(writerStarted)
			started <- "blocked"
			<-releaseWriters
			_, err := w.Write([]byte("blocked"))
			return err
		},
	})

	select {
	case <-writerStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("eager writer did not start")
	}

	for i := range 3 {
		target := "queued-" + strconv.Itoa(i) + ".txt"
		man.Emit(Artefact{
			Claim: Claim{Owner: "test", Target: target},
			Builder: func(w io.Writer) error {
				started <- target
				<-releaseWriters
				_, err := w.Write([]byte("queued"))
				return err
			},
		})
	}

	done := make(chan error, 1)
	go func() {
		done <- man.Complete(true)
	}()

	got := map[string]struct{}{}
	deadline := time.After(2 * time.Second)
	for len(got) < 3 {
		select {
		case target := <-started:
			got[target] = struct{}{}
		case <-deadline:
			t.Fatalf("expected Complete to start drain workers, got starts for %v", got)
		}
	}

	close(releaseWriters)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Complete failed: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Complete did not finish")
	}
}

func TestBeginReportsDuplicateClaims(t *testing.T) {
	t.Parallel()

	out := t.TempDir()
	man := New()

	opts := options.DefaultOptions()

	if err := man.Begin(context.Background(), config.DefaultConfig(), opts, nil, out); err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	first := TextArtefact(Claim{Owner: "one", Target: "index.html"}, "one")
	second := TextArtefact(Claim{Owner: "two", Target: "index.html"}, "two")
	man.Emit(first)
	man.Emit(second)

	if err := man.Complete(true); err == nil {
		t.Fatalf("expected duplicate claim failure")
	}
}

package build

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/options"
)

func TestBuildWaitsForRunningStepsBeforeClosingManifest(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Root = t.TempDir()

	opts := options.DefaultOptions()
	opts.Context = context.Background()
	opts.OutputPath = t.TempDir()
	opts.MaxWorkers = 2

	emitReady := make(chan struct{})
	allowEmit := make(chan struct{})
	done := make(chan error, 1)

	steps := []Step{
		StepFunc("slow-emit", func(_ context.Context, sc *StepContext) error {
			close(emitReady)
			<-allowEmit
			sc.Manifest.Emit(manifest.TextArtefact(
				manifest.Claim{Owner: "slow-emit", Target: "index.html"},
				"ok",
			))
			return nil
		}),
		StepFunc("fail-fast", func(_ context.Context, _ *StepContext) error {
			return errors.New("boom")
		}),
	}

	go func() {
		done <- build(steps, cfg, opts, cfg.Root, "shizuka.toml")
	}()

	select {
	case <-emitReady:
	case <-time.After(2 * time.Second):
		t.Fatal("slow step did not start")
	}

	close(allowEmit)

	select {
	case err := <-done:
		if !errors.Is(err, ErrTaskError) {
			t.Fatalf("build() error = %v, want ErrTaskError", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("build() did not finish")
	}
}

func TestBuildFailsOnDuplicateOutputClaims(t *testing.T) {
	t.Parallel()

	cfg := config.DefaultConfig()
	cfg.Root = t.TempDir()

	opts := options.DefaultOptions()
	opts.Context = context.Background()
	opts.OutputPath = t.TempDir()
	opts.MaxWorkers = 2
	opts.ArtefactWorkers = 0

	steps := []Step{
		StepFunc("one", func(_ context.Context, sc *StepContext) error {
			sc.Manifest.Emit(manifest.TextArtefact(
				manifest.Claim{Owner: "one", Target: "index.html"},
				"one",
			))
			return nil
		}),
		StepFunc("two", func(_ context.Context, sc *StepContext) error {
			sc.Manifest.Emit(manifest.TextArtefact(
				manifest.Claim{Owner: "two", Target: "index.html"},
				"two",
			))
			return nil
		}),
	}

	err := build(steps, cfg, opts, cfg.Root, "shizuka.toml")
	if !errors.Is(err, manifest.ErrConflicts) {
		t.Fatalf("build() error = %v, want ErrConflicts", err)
	}
}

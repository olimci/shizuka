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
		done <- build(steps, cfg, opts, cfg.Root, "shizuka.toml", nil)
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

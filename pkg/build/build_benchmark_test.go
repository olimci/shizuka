package build

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/options"
)

type workerRatioWorkload struct {
	name          string
	steps         int
	artefacts     int
	bytes         int
	stepDelay     time.Duration
	artefactDelay time.Duration
}

func BenchmarkWorkerRatios(b *testing.B) {
	stepWorkers := []int{1, 2, 4, 8, runtime.NumCPU()}
	slices.Sort(stepWorkers)
	stepWorkers = slices.Compact(stepWorkers)

	workloads := []workerRatioWorkload{
		{name: "small-fast", steps: 8, artefacts: 256, bytes: 128},
		{name: "large-copy", steps: 8, artefacts: 64, bytes: 64 * 1024},
		{name: "slow-artefact", steps: 8, artefacts: 128, bytes: 512, artefactDelay: 250 * time.Microsecond},
		{name: "mixed-step-and-artefact", steps: 8, artefacts: 128, bytes: 512, stepDelay: 250 * time.Microsecond, artefactDelay: 100 * time.Microsecond},
	}

	for _, workload := range workloads {
		b.Run(workload.name, func(b *testing.B) {
			for _, workers := range stepWorkers {
				for _, artefactWorkers := range artefactWorkerSweep(workers) {
					name := fmt.Sprintf("steps=%d/artefacts=%d", workers, artefactWorkers)
					b.Run(name, func(b *testing.B) {
						benchmarkWorkerRatio(b, workload, workers, artefactWorkers)
					})
				}
			}
		})
	}
}

func artefactWorkerSweep(stepWorkers int) []int {
	values := []int{
		0,
		1,
		max(1, stepWorkers/4),
		max(1, stepWorkers/2),
		stepWorkers,
		stepWorkers * 2,
	}
	slices.Sort(values)
	return slices.Compact(values)
}

func benchmarkWorkerRatio(b *testing.B, workload workerRatioWorkload, stepWorkers, artefactWorkers int) {
	b.Helper()
	b.ReportAllocs()

	payload := strings.Repeat("x", workload.bytes)
	baseOut := b.TempDir()
	cfg := config.DefaultConfig()
	cfg.Root = b.TempDir()

	steps := makeWorkerRatioSteps(workload, payload)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		opts := options.DefaultOptions()
		opts.Context = context.Background()
		opts.OutputPath = filepath.Join(baseOut, fmt.Sprintf("run-%d", i))
		opts.MaxWorkers = stepWorkers
		opts.ArtefactWorkers = artefactWorkers
		opts.SkipOutputCleanup = true

		if err := build(steps, cfg, opts, cfg.Root, "shizuka.toml"); err != nil {
			b.Fatal(err)
		}
	}
}

func makeWorkerRatioSteps(workload workerRatioWorkload, payload string) []Step {
	steps := make([]Step, 0, workload.steps)
	for stepIndex := range workload.steps {
		stepIndex := stepIndex
		steps = append(steps, StepFunc(fmt.Sprintf("step-%d", stepIndex), func(ctx context.Context, sc *StepContext) error {
			if workload.stepDelay > 0 {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(workload.stepDelay):
				}
			}

			for artefactIndex := stepIndex; artefactIndex < workload.artefacts; artefactIndex += workload.steps {
				artefactIndex := artefactIndex
				sc.Manifest.Emit(manifest.Artefact{
					Claim: manifest.Claim{
						Owner:  fmt.Sprintf("step-%d", stepIndex),
						Target: fmt.Sprintf("step-%d/artefact-%d.txt", stepIndex, artefactIndex),
					},
					Builder: func(w io.Writer) error {
						if workload.artefactDelay > 0 {
							time.Sleep(workload.artefactDelay)
						}
						_, err := io.WriteString(w, payload)
						return err
					},
				})
			}
			return nil
		}))
	}
	return steps
}

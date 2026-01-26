package build

import (
	"context"
	"sync"

	"github.com/olimci/shizuka/pkg/steps"
)

func newResourceManager() *resourceManager {
	rm := &resourceManager{
		readers: make(map[string]int),
		writers: make(map[string]bool),
	}
	rm.cond = sync.NewCond(&rm.mu)
	return rm
}

type resourceManager struct {
	mu      sync.Mutex
	cond    *sync.Cond
	readers map[string]int
	writers map[string]bool
}

func (rm *resourceManager) Broadcast() {
	rm.mu.Lock()
	rm.cond.Broadcast()
	rm.mu.Unlock()
}

func (rm *resourceManager) Acquire(ctx context.Context, step steps.Step) error {
	reads, writes := normalizeResources(step.Reads, step.Writes)
	if len(reads) == 0 && len(writes) == 0 {
		return nil
	}

	rm.mu.Lock()
	defer rm.mu.Unlock()

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if rm.available(reads, writes) {
			rm.lock(reads, writes)
			return nil
		}
		rm.cond.Wait()
	}
}

func (rm *resourceManager) Release(step steps.Step) {
	reads, writes := normalizeResources(step.Reads, step.Writes)
	if len(reads) == 0 && len(writes) == 0 {
		return
	}

	rm.mu.Lock()
	rm.unlock(reads, writes)
	rm.cond.Broadcast()
	rm.mu.Unlock()
}

func (rm *resourceManager) available(reads, writes []string) bool {
	for _, w := range writes {
		if rm.writers[w] || rm.readers[w] > 0 {
			return false
		}
	}
	for _, r := range reads {
		if rm.writers[r] {
			return false
		}
	}
	return true
}

func (rm *resourceManager) lock(reads, writes []string) {
	for _, w := range writes {
		rm.writers[w] = true
	}
	for _, r := range reads {
		rm.readers[r]++
	}
}

func (rm *resourceManager) unlock(reads, writes []string) {
	for _, w := range writes {
		delete(rm.writers, w)
	}
	for _, r := range reads {
		if count := rm.readers[r] - 1; count > 0 {
			rm.readers[r] = count
		} else {
			delete(rm.readers, r)
		}
	}
}

func normalizeResources(reads, writes []string) ([]string, []string) {
	if len(reads) == 0 && len(writes) == 0 {
		return nil, nil
	}

	writeSet := make(map[string]struct{}, len(writes))
	for _, w := range writes {
		if w == "" {
			continue
		}
		writeSet[w] = struct{}{}
	}

	readSet := make(map[string]struct{}, len(reads))
	for _, r := range reads {
		if r == "" {
			continue
		}
		if _, ok := writeSet[r]; ok {
			continue
		}
		readSet[r] = struct{}{}
	}

	normWrites := make([]string, 0, len(writeSet))
	for w := range writeSet {
		normWrites = append(normWrites, w)
	}

	normReads := make([]string, 0, len(readSet))
	for r := range readSet {
		normReads = append(normReads, r)
	}

	return normReads, normWrites
}

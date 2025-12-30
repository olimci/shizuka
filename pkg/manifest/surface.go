package manifest

import (
	"sync"
)

type Surface struct {
	parent *Manifest

	artefacts []Artefact

	registry map[string]any
}

type SurfaceCache struct {
	artefacts []Artefact
	registry  map[string]any
}

func (s *Surface) Get(k string) (any, bool) {
	s.parent.registryMu.RLock()
	unlock := sync.OnceFunc(func() {
		s.parent.registryMu.RUnlock()
	})
	defer unlock()

	v, ok := s.parent.registry[k]
	if ok {
		return v, true
	}

	unlock()

	v, ok = s.registry[k]
	return v, ok
}

func (s *Surface) Set(k string, v any) {
	s.registry[k] = v
}

func (s *Surface) Emit(a Artefact) {
	s.artefacts = append(s.artefacts, a)
}

func (s *Surface) AsCache() *SurfaceCache {
	return &SurfaceCache{
		artefacts: s.artefacts,
		registry:  s.registry,
	}
}

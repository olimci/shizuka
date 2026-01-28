package steps

import (
	"fmt"
	"io/fs"

	"github.com/olimci/shizuka/pkg/events"
	"github.com/olimci/shizuka/pkg/manifest"
	"github.com/olimci/shizuka/pkg/utils/set"
)

func NewStepContext(man *manifest.Manifest, sourceFS fs.FS, sourceRoot string, handler events.Handler, step Step) StepContext {
	return &stepContext{
		manifest:     man,
		sourceFS:     sourceFS,
		sourceRoot:   sourceRoot,
		reads:        set.FromSlice(step.Reads),
		writes:       set.FromSlice(step.Writes),
		eventHandler: handler,
	}
}

type stepContext struct {
	id StepID

	manifest   *manifest.Manifest
	sourceFS   fs.FS
	sourceRoot string
	reads      *set.Set[string]
	writes     *set.Set[string]

	eventHandler events.Handler
}

func (sc *stepContext) Get(key string) (any, bool) {
	if !sc.reads.Has(key) && !sc.writes.Has(key) {
		panic(fmt.Sprintf("step %s read registry key %q without declaring it", sc.id, key))
	}
	return sc.manifest.Get(key)
}

func (sc *stepContext) Set(key string, value any) {
	if !sc.writes.Has(key) {
		panic(fmt.Sprintf("step %s wrote registry key %q without declaring it", sc.id, key))
	}
	sc.manifest.Set(key, value)
}

func (sc *stepContext) Emit(artefact manifest.Artefact) {
	sc.manifest.Emit(artefact)
}

func (sc *stepContext) Source() (fs.FS, string) {
	return sc.sourceFS, sc.sourceRoot
}

func (sc *stepContext) event(level events.Level, message string, err error) {
	sc.eventHandler.Handle(events.Event{
		Level:   level,
		Message: message,
		Error:   err,
	})
}

func (sc *stepContext) Debug(message string) {
	sc.event(events.Debug, message, nil)
}

func (sc *stepContext) Debugf(format string, args ...any) {
	sc.event(events.Debug, fmt.Sprintf(format, args...), nil)
}

func (sc *stepContext) Info(message string) {
	sc.event(events.Info, message, nil)
}

func (sc *stepContext) Infof(format string, args ...any) {
	sc.event(events.Info, fmt.Sprintf(format, args...), nil)
}

func (sc *stepContext) Error(err error, message string) {
	sc.event(events.Error, message, err)
}

func (sc *stepContext) Errorf(err error, format string, args ...any) {
	sc.event(events.Error, fmt.Sprintf(format, args...), err)
}

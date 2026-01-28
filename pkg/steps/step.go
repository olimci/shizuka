package steps

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/olimci/shizuka/pkg/manifest"
)

type StepID struct {
	Owner string
	Name  string
	Sub   string
}

func (s StepID) String() string {
	if s.Sub == "" {
		return fmt.Sprintf("%s:%s", s.Owner, s.Name)
	}
	return fmt.Sprintf("%s:%s:%s", s.Owner, s.Name, s.Sub)
}

// StepContext is the interface for the build step to interact with the build process.
type StepContext interface {
	Get(key string) (any, bool)
	Set(key string, value any)
	Emit(artefact manifest.Artefact)
	Source() (fs.FS, string)
	Debug(message string)
	Debugf(format string, args ...any)
	Info(message string)
	Infof(format string, args ...any)
	Error(err error, message string)
	Errorf(err error, format string, args ...any)
}

func StepFunc(id StepID, fn func(context.Context, StepContext) error) Step {
	return Step{
		ID: id,
		Fn: fn,
	}
}

type Step struct {
	ID     StepID
	Deps   []StepID
	Reads  []string
	Writes []string
	Fn     func(context.Context, StepContext) error
}

func (s Step) WithReads(reads ...string) Step {
	s.Reads = append([]string(nil), reads...)
	return s
}

func (s Step) WithWrites(writes ...string) Step {
	s.Writes = append([]string(nil), writes...)
	return s
}

func (s Step) WithDeps(deps ...StepID) Step {
	s.Deps = append(s.Deps, deps...)
	return s
}

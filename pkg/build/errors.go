package build

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/olimci/shizuka/pkg/manifest"
)

type BuildError struct {
	Claim manifest.Claim
	Err   error
}

func WrapError(claim manifest.Claim, err error) *BuildError {
	if err == nil {
		return nil
	}

	if buildErr, ok := errors.AsType[*BuildError](err); ok {
		if isZeroClaim(buildErr.Claim) && !isZeroClaim(claim) {
			out := *buildErr
			out.Claim = claim
			return &out
		}
		return buildErr
	}

	return &BuildError{
		Claim: claim,
		Err:   err,
	}
}

func (e *BuildError) Error() string {
	if e == nil {
		return ""
	}
	if loc := e.Location(); loc != "" {
		return fmt.Sprintf("%s: %v", loc, e.Err)
	}
	if e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *BuildError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *BuildError) Source() string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e.Claim.Source)
}

func (e *BuildError) Target() string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e.Claim.Target)
}

func (e *BuildError) Owner() string {
	if e == nil {
		return ""
	}
	return strings.TrimSpace(e.Claim.Owner)
}

func (e *BuildError) Location() string {
	if src := e.Source(); src != "" {
		return src
	}
	if target := e.Target(); target != "" {
		return target
	}
	return e.Owner()
}

func (e *BuildError) Description() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

type errorState struct {
	mu     sync.Mutex
	errors []*BuildError
}

func (s *errorState) Add(claim manifest.Claim, err error) {
	buildErr := WrapError(claim, err)
	if buildErr == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.errors = append(s.errors, buildErr)
}

func (s *errorState) HasErrors() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.errors) > 0
}

func (s *errorState) Slice() []*BuildError {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]*BuildError, len(s.errors))
	copy(out, s.errors)
	return out
}

type Failure struct {
	Errors []*BuildError
}

func (f *Failure) Error() string {
	if f == nil || len(f.Errors) == 0 {
		return ErrBuildFailed.Error()
	}

	if len(f.Errors) == 1 && f.Errors[0] != nil {
		return fmt.Sprintf("%s: %s", ErrBuildFailed, f.Errors[0])
	}

	return fmt.Sprintf("%s: %s", ErrBuildFailed, f.Summary())
}

func (f *Failure) Unwrap() error {
	if f == nil || len(f.Errors) == 0 {
		return nil
	}
	return f.Errors[0]
}

func (f *Failure) Count() int {
	if f == nil {
		return 0
	}
	return len(f.Errors)
}

func (f *Failure) HasErrors() bool {
	return f != nil && len(f.Errors) > 0
}

func (f *Failure) Summary() string {
	switch n := f.Count(); n {
	case 0:
		return "no errors"
	case 1:
		return "1 error"
	default:
		return fmt.Sprintf("%d errors", n)
	}
}

func AsFailure(err error) (*Failure, bool) {
	var failure *Failure
	if errors.As(err, &failure) {
		return failure, true
	}
	return nil, false
}

func isZeroClaim(claim manifest.Claim) bool {
	return strings.TrimSpace(claim.Owner) == "" &&
		strings.TrimSpace(claim.Source) == "" &&
		strings.TrimSpace(claim.Target) == "" &&
		strings.TrimSpace(claim.Canon) == ""
}

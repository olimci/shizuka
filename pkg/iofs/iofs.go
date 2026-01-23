package iofs

import (
	"context"
	"io"
	"io/fs"
)

// Readable represents an arbitrary readable source.
type Readable interface {
	FS(context.Context) (fs.FS, error)
	Root() string
	Close() error
}

// WriterFunc writes a destination file to the provided writer.
type WriterFunc func(w io.Writer) error

// Writable abstracts the output filesystem for builds.
// Implementations must be safe for concurrent Write calls.
type Writable interface {
	Readable
	EnsureRoot() error
	MkdirAll(rel string, perm fs.FileMode) error
	Remove(rel string) error
	RemoveAll(rel string) error
	Write(rel string, gen WriterFunc, exists bool) error
}

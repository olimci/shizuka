package fileutil

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
)

type AtomicOptions struct {
	Sync            bool
	CompareExisting bool
}

func AtomicWrite(root *os.Root, path string, gen func(w io.Writer) error, opts AtomicOptions) (bool, error) {
	dir, base := filepath.Split(path)
	tmpRel, tmp, err := temp(root, dir, "."+base+".tmp-")
	if err != nil {
		return false, err
	}
	defer func() {
		_ = tmp.Close()
		_ = root.Remove(tmpRel)
	}()

	if err := gen(tmp); err != nil {
		return false, err
	}
	if opts.Sync {
		if err := tmp.Sync(); err != nil {
			return false, err
		}
	}
	if err := tmp.Close(); err != nil {
		return false, err
	}

	if opts.CompareExisting {
		if eq, err := cmp(root, tmpRel, path); err != nil {
			return false, err
		} else if eq {
			return false, nil
		}
	}

	if err := root.Rename(tmpRel, path); err != nil {
		return false, err
	}
	if opts.Sync {
		syncDir := dir
		if syncDir == "" {
			syncDir = "."
		}
		if df, err := root.Open(syncDir); err == nil {
			_ = df.Sync()
			_ = df.Close()
		}
	}
	return true, nil
}

func temp(root *os.Root, dir, prefix string) (string, *os.File, error) {
	for range 100 {
		var b [16]byte
		if _, err := rand.Read(b[:]); err != nil {
			return "", nil, err
		}
		name := filepath.Join(dir, prefix+hex.EncodeToString(b[:]))
		file, err := root.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			return name, file, nil
		}
		if errors.Is(err, os.ErrExist) {
			continue
		}
		return "", nil, err
	}
	return "", nil, fmt.Errorf("create temporary file in %q: too many collisions", dir)
}

func cmp(root *os.Root, a, b string) (bool, error) {
	aFi, err := root.Stat(a)
	if err != nil {
		return false, err
	} else if aFi.IsDir() {
		return false, fmt.Errorf("a is a directory")
	}

	bFi, err := root.Stat(b)
	if err != nil {
		return false, err
	} else if bFi.IsDir() {
		return false, fmt.Errorf("b is a directory")
	}

	if aFi.Size() != bFi.Size() {
		return false, nil
	}

	aF, err := root.Open(a)
	if err != nil {
		return false, err
	}
	defer aF.Close()

	bF, err := root.Open(b)
	if err != nil {
		return false, err
	}
	defer bF.Close()

	return cmpReader(aF, bF)
}

func cmpReader(aF, bF io.Reader) (bool, error) {
	const bufSize = 128 * 1024
	aBuf := make([]byte, bufSize)
	bBuf := make([]byte, bufSize)

	for {
		aN, aErr := io.ReadFull(aF, aBuf)
		bN, bErr := io.ReadFull(bF, bBuf)

		if aErr != nil && !errors.Is(aErr, io.EOF) && !errors.Is(aErr, io.ErrUnexpectedEOF) {
			return false, aErr
		}

		if bErr != nil && !errors.Is(bErr, io.EOF) && !errors.Is(bErr, io.ErrUnexpectedEOF) {
			return false, bErr
		}

		if aN == 0 && bN == 0 {
			return true, nil
		}

		if aN != bN || !slices.Equal(aBuf[:aN], bBuf[:bN]) {
			return false, nil
		}
	}
}

package fileutil

import (
	"os"
	"time"
)

type Stat struct {
	Created time.Time
	Updated time.Time
	Size    int64
}

func Info(path string) (Stat, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Stat{}, err
	}

	meta := Stat{
		Updated: info.ModTime(),
		Size:    info.Size(),
	}

	// TODO: can we make this work on more platforms?
	if created, ok := createdAt(info); ok {
		meta.Created = created
	} else {
		meta.Created = meta.Updated
	}
	return meta, nil
}

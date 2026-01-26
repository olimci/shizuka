package transforms

import "time"

type BuildMeta struct {
	StartTime       time.Time
	StartTimestring string

	ConfigPath string
	Dev        bool
}

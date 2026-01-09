package watcher

type Event struct {
	Reason string
	Paths  []string
}

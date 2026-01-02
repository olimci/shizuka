package scaffold

type Collection struct {
	Config CollectionCfg
	source Source
	Base   string

	Templates []*Template
}

func (c *Collection) Close() error {
	return c.source.Close()
}

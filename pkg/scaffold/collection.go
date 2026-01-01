package scaffold

type Collection struct {
	Config CollectionConfig
	source source
	Base   string

	Scaffolds []*Scaffold
}

func (c *Collection) Close() error {
	return c.source.Close()
}

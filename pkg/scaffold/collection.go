package scaffold

type Collection struct {
	Config CollectionCfg
	source Source
	Base   string

	Templates []*Template
}

func (c *Collection) Get(slug string) *Template {
	for _, t := range c.Templates {
		if t.Config.Metadata.Slug == slug {
			return t
		}
	}

	return nil
}

func (c *Collection) Close() error {
	return c.source.Close()
}

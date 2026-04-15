package transforms

type QueryResult struct {
	Rows  []map[string]any
	Pages []*Page
}

type PaginationState struct {
	QueryKey       string
	PageNumber     int
	PageSize       int
	TotalItems     int
	TotalPages     int
	HasPrev        bool
	HasNext        bool
	PrevPageNumber int
	NextPageNumber int
	PrevURL        string
	NextURL        string
}

type PageQueryDef struct {
	Query    string `toml:"query" yaml:"query" json:"query"`
	Paginate bool   `toml:"paginate" yaml:"paginate" json:"paginate"`
	PageSize int    `toml:"page_size" yaml:"page_size" json:"page_size"`
	Template string `toml:"template" yaml:"template" json:"template"`
}

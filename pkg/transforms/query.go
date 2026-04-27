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

type QueryGroupState struct {
	QueryKey string
	Column   string
	Value    any
	URLPath  string
}

type PageQueryDef struct {
	Query           string `toml:"query" yaml:"query" json:"query"`
	Paginate        bool   `toml:"paginate" yaml:"paginate" json:"paginate"`
	PageSize        int    `toml:"page_size" yaml:"page_size" json:"page_size"`
	Template        string `toml:"template" yaml:"template" json:"template"`
	GroupBy         string `toml:"group_by" yaml:"group_by" json:"group_by"`
	GroupFormat     string `toml:"group_format" yaml:"group_format" json:"group_format"`
	PageFormat      string `toml:"page_format" yaml:"page_format" json:"page_format"`
	RedirectPageOne bool   `toml:"redirect_page_1" yaml:"redirect_page_1" json:"redirect_page_1"`
}

# Templates

Shizuka templates are Go `html/template` files. Each content page selects a template by name using frontmatter `template = "..."`.

## Where templates live

By default, Shizuka loads templates via:

- `paths.templates` (default: `templates/*.tmpl`)

## How template names are chosen

Each template file should define one named template, and frontmatter should reference that name:

- `templates/page.tmpl` should contain `{{ define "page" }}...{{ end }}`
- `templates/post.tmpl` should contain `{{ define "post" }}...{{ end }}`

Defined template names must be unique across all files matched by `paths.templates`.

## Template input data

Each page is rendered with this root object:

- `.Page`: the current page
- `.Site`: global site data

### `.Page` fields

Key fields you can use in templates:

- `.Page.Title`, `.Page.Description`, `.Page.Tags`, `.Page.Featured`, `.Page.Draft`
- `.Page.Source.Kind` (`"markdown"`, `"html"`, `"structured"`)
- `.Page.SourcePath` (source file path)
- `.Page.URLPath` (site-relative URL path without a leading slash)
- `.Page.OutputPath` (output path, usually `{url_path}/index.html`)
- `.Page.Template` (requested template name)
- `.Page.Slug` (clean slug without leading slash)
- `.Page.Aliases` (`[]string`, site-relative alias paths)
- `.Page.Weight` (`int`)
- `.Page.Canon` (absolute canonical URL)
- `.Page.Section`
- `.Page.Date`, `.Page.Updated`, `.Page.PubDate` (`time.Time`)
- `.Page.Git.Tracked`, `.Page.Git.Created`, `.Page.Git.Updated`
- `.Page.Git.CommitHash`, `.Page.Git.ShortHash`, `.Page.Git.AuthorName`
- `.Page.Body` (`template.HTML`)
- `.Page.Links` (`[]PageLink`; resolved internal page links discovered from Markdown and wikilinks)
- `.Page.Parent`, `.Page.Children` (hierarchical navigation inferred from `index.*` pages in content directories)
- `.Page.Params` (arbitrary map)
- `.Page.Queries` (`map[string]*QueryResult`) for frontmatter-defined computed queries
- `.Page.Assets` (`map[string]*PageAsset`) for owned bundle assets
- `.Page.Headers` (used by the headers build step; see `_docs/config.md`)
- `.Page.Pagination` (`*PaginationState`) for paginated page renders only

`PageLink` contains: `RawTarget`, `Fragment`, `Label`, `Embed`, `Target` (`*Page`).

`QueryResult` contains:

- `.Rows` (`[]map[string]any`)
- `.Pages` (`[]*Page`) when the query includes `_page`

### `.Site` fields

- `.Site.Title`, `.Site.Description`, `.Site.URL`
- `.Site.Params`
- `.Site.Queries` (`map[string]*QueryResult`) for config-defined computed queries
- `.Site.Meta.IsDev`, `.Site.Meta.ConfigPath`, `.Site.Meta.BuildTime`
- `.Site.Meta.Git.Available`, `.Site.Meta.Git.RepoRoot`, `.Site.Meta.Git.GitDir`
- `.Site.Meta.Git.Branch`, `.Site.Meta.Git.CommitHash`, `.Site.Meta.Git.ShortHash`, `.Site.Meta.Git.Dirty`

## Built-in template functions

Shizuka registers a small set of helpers:

- `datefmt layout t` -> formatted time string (returns `""` for zero time)
- `default fallback value` -> `fallback` when `value` is empty / zero
- `uniq values` -> deduplicated `[]string` preserving original order
- `slugify s` -> normalized URL-safe slug string
- `asset key page` -> public URL string for a bundle asset, or `""`
- `assetMeta key page` -> `*PageAsset` for a bundle asset, or `nil`
- `dict key value ...` -> `map[string]any`
- `merge map ...` -> merged `map[string]any` (later maps win)
- `discard` -> aborts the current template artefact without failing the build
- `query sql args...` -> `*QueryResult`
- `asPages` / `AsPages` -> `[]*Page` from a page-backed `QueryResult`
- `asPage` / `AsPage` -> first `*Page` from a page-backed `QueryResult`
- `first` / `First` -> first row or item from a `QueryResult`, `[]map[string]any`, or `[]*Page`
  - built-in tables: `pages`, `site`, `page_links`, `page_assets`
  - page conversion expects the query result to include `_page`; `select * from pages ...` works, while projected queries like `select Title from pages` do not
  - positional placeholders use `?`, for example: `{{ query "select * from pages where Section = ? order by Date desc limit ?" "posts" 5 | asPages }}`

Everything else is standard Go template behavior (`range`, `if`, `len`, `index`, etc).

## Recommended template shape

```gotemplate
{{ define "page" }}
<!DOCTYPE html>
<html lang="en">
<body>
    <main>{{ .Page.Body }}</main>
</body>
</html>
{{ end }}
```

Then reference that name from frontmatter with `template = "page"`.

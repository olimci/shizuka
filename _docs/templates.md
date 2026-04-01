# Templates

Shizuka templates are Go `html/template` files. Each content page selects a template by name using frontmatter `template: "…"`.

## Where templates live

By default, Shizuka loads templates via:

- `build.steps.content.template_glob` (default: `templates/*.tmpl`)

## How template names are chosen

Each template file should define one named template, and your frontmatter should reference that name:

- `templates/page.tmpl` should contain `{{ define "page" }}...{{ end }}`
- `templates/post.tmpl` should contain `{{ define "post" }}...{{ end }}`

Using the file basename as the template name is the recommended convention, but the runtime ultimately looks up the name you define in the template set. Those defined names must be unique across all files matched by `template_glob`.

## Template input data

Each page is rendered with this root object:

- `.Page`: the current page
- `.Site`: global site data and collections

### `.Page` fields

Key fields you can use in templates:

- `.Page.Title`, `.Page.Description`, `.Page.Tags`, `.Page.Featured`, `.Page.Draft`
- `.Page.Slug` (clean slug without leading slash)
- `.Page.Aliases` (`[]string`, site-relative alias paths)
- `.Page.Weight` (`int`)
- `.Page.Canon` (absolute canonical URL)
- `.Page.Section` (populated from frontmatter key `sections`)
- `.Page.Date`, `.Page.Updated`, `.Page.PubDate` (`time.Time`; use `.IsZero`, `.Format`, etc)
- `.Page.Git.Tracked`, `.Page.Git.Created`, `.Page.Git.Updated`
- `.Page.Git.CommitHash`, `.Page.Git.ShortHash`, `.Page.Git.AuthorName`
- `.Page.Body` (`template.HTML`; rendered Markdown output or `body` from structured pages)
- `.Page.Params` (arbitrary map for full page templates)
- `.Page.Headers` (used by the headers build step; see `_docs/config.md`)
- `.Page.Meta.Source` (source file path)
- `.Page.Meta.URLPath` (site-relative URL path without a leading slash)
- `.Page.Meta.Target` (output path, usually `{url_path}/index.html`)
- `.Page.Meta.Template` (the template name requested by frontmatter)

### `.Site` fields

- `.Site.Title`, `.Site.Description`, `.Site.URL`
- `.Site.Meta.IsDev`, `.Site.Meta.ConfigPath`, `.Site.Meta.BuildTime`, `.Site.Meta.BuildTimeString`
- `.Site.Meta.Git.Available`, `.Site.Meta.Git.RepoRoot`, `.Site.Meta.Git.GitDir`
- `.Site.Meta.Git.Branch`, `.Site.Meta.Git.CommitHash`, `.Site.Meta.Git.ShortHash`, `.Site.Meta.Git.Dirty`
- `.Site.Tree` (page tree for hierarchical navigation)
- `.Site.Collections`:
  - `.All` (`[]*PageLite`)
  - `.Published`, `.Drafts`, `.Featured`
  - `.Latest`, `.RecentlyUpdated`, `.Undated`
- `.Site.Groups`:
  - `.BySlug` (`map[string]*PageLite`)
  - `.BySection`, `.ByTag` (`map[string][]*PageLite`)
  - `.ByYear` (`map[int][]*PageLite`)
  - `.ByYearMonth` (`map[string][]*PageLite`, key format `YYYY-MM`)

`PageLite` contains: `Git`, `Slug`, `Canon`, `Aliases`, `Weight`, `Title`, `Description`, `Section`, `Tags`, `Date`, `Updated`, `PubDate`, `Params`, `Featured`, `Draft`. `PageLite.Params` is copied from `.Page.Params` with `_`-prefixed keys stripped out.

## Built-in template functions

Shizuka registers a small set of helpers:

- `where field value pages` -> filtered `[]*PageLite`
  - supported `field` values: `"Title"`, `"Description"`, `"Section"`, `"Slug"`, `"Weight"`, `"Featured"`, `"Draft"`, `"Date:before"`, `"Date:after"`, `"Updated:before"`, `"Updated:after"`, `"Tags"`, `"Tags:not"`
- `whereEq field value pages` -> pages where `field == value`
- `whereNe field value pages` -> pages where `field != value`
- `whereHas field value pages` -> pages where a list-like field contains `value`
- `whereIn field pages value...` -> pages where `field` matches any of the provided values
  - `whereEq` / `whereNe` / `whereHas` / `whereIn` support: `"Title"`, `"Description"`, `"Section"`, `"Slug"`, `"Weight"`, `"Featured"`, `"Draft"`, `"Date"`, `"Updated"`, `"PubDate"`, `"Tags"`
  - `whereEq` / `whereNe` / `whereHas` also support page params via `"Params.some_key"`
- `sort field order pages` -> sorted `[]*PageLite`
  - `order` must be `"asc"` or `"desc"`
  - `field` values: `"Title"`, `"Description"`, `"Section"`, `"Slug"`, `"Weight"`, `"Date"`, `"Updated"`, `"PubDate"`
- `limit n pages` -> first `n` pages
- `offset n pages` -> pages after skipping the first `n`
- `first pages` / `last pages` -> a single `*PageLite` (or `nil` if empty)
- `groupBy field pages` -> `map[string][]*PageLite`
  - supported `field` values: `"Section"`, `"Tags"`, `"Year"`, `"YearMonth"`, `"Featured"`, `"Draft"`
- `datefmt layout t` -> formatted time string (returns `""` for zero time)
- `default fallback value` -> `fallback` when `value` is empty / zero
- `uniq values` -> deduplicated `[]string` preserving original order
- `slugify s` -> normalized URL-safe slug string
- `dict key value ...` -> `map[string]any`
- `merge map ...` -> merged `map[string]any` (later maps win)

Everything else is standard Go template behavior (`range`, `if`, `len`, `index`, etc).

## Recommended template shape

Shizuka executes the template by name. The recommended pattern is to make each `.tmpl` file a named partial:

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

Then reference that name from frontmatter with `template: "page"`.

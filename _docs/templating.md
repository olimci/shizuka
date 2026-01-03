# Templating and frontmatter

This doc covers how to write content frontmatter and Go templates for Shizuka.

## Content files and frontmatter

Shizuka reads files from the content directory and turns them into pages. Supported
content types:

- Markdown: `.md` with fenced frontmatter
- Structured: `.toml`, `.yaml`, `.yml`, `.json` (entire file is frontmatter)
- Raw HTML: `.html` is copied as-is (no template rendering)

Markdown frontmatter must be fenced with `---` (YAML) or `+++` (TOML). Example:

```yaml
---
title: "Welcome"
template: "page"
---

# Hello
```

```toml
+++
title = "Welcome"
template = "page"
+++

# Hello
```

Structured files (TOML/YAML/JSON) represent the frontmatter directly. For those
formats, the `body` field becomes the HTML body that is passed to templates.

## Frontmatter fields

All formats map to the same fields:

- `slug` string
- `title` string
- `description` string
- `sections` string (mapped to `Page.Section` in templates)
- `tags` array of strings
- `date` time
- `updated` time
- `params` map (arbitrary data for the full page)
- `lite_params` map (arbitrary data for page collections)
- `template` string (template name to render)
- `body` string (only used for TOML/YAML/JSON content)
- `featured` bool
- `draft` bool

Notes:
- For Markdown, the body is the Markdown content after frontmatter and is rendered
  to HTML automatically.
- For TOML/YAML/JSON, the `body` field is taken verbatim as HTML.
- Dates are parsed by the respective decoder (TOML datetime, YAML timestamp, JSON
  time string). If a date fails to parse, the build fails for that page.

## Templates

Templates use Go's `html/template` engine. Files are loaded from the `templates`
folder using the `build.templates_glob` config value. By default this is:

```
templates/*.tmpl
```

Template names are derived from the file base name. Example:

- `templates/page.tmpl` -> template name `page`
- `templates/post.tmpl` -> template name `post`

Your frontmatter `template` value must match this name. If a template is missing,
the build fails unless a fallback template is configured.

## Template data

Each template receives a `PageTemplate` value with these fields:

- `.Page` (full page data)
- `.Site` (site metadata and collections)
- `.Meta` (source/target/template metadata)
- `.SiteMeta` (build metadata)

### `.Page`

```text
Slug, Title, Description, Section, Tags,
Date, Updated, Params, LiteParams,
Body (HTML), Featured, Draft
```

### `.Site`

```text
Title, Description, URL
Collections (All, Drafts, Featured, Latest, RecentlyUpdated)
```

Each collection is a list of `PageLite` values. `PageLite` contains:

```text
Slug, Title, Description, Section, Tags,
Date, Updated, LiteParams, Featured, Draft
```

### `.Meta`

```text
Source, Target, Template
```

### `.SiteMeta`

```text
BuildTime (RFC3339 string), Dev (bool)
```

## Template helpers

Shizuka registers a few template helpers for working with collections:

- `where FIELD VALUE PAGES` filters pages by field
- `sort FIELD ORDER PAGES` sorts pages by field
- `limit N PAGES` limits to the first N pages

Supported `FIELD` values for `where`:

- `Section`
- `Featured`
- `Draft`
- `Date:before`
- `Date:after`
- `Updated:before`
- `Updated:after`
- `Tags`
- `Tags:not`

Supported `FIELD` values for `sort`:

- `Title`
- `Description`
- `Section`
- `Slug`
- `Date`
- `Updated`

`ORDER` must be `asc` or `desc`. Any other value returns an empty list.

Example:

```gotemplate
{{ $posts := where "Section" "posts" .Site.Collections.All }}
{{ $recent := limit 5 (sort "Date" "desc" $posts) }}
```

## Output paths

Pages are routed by filename, not by frontmatter `slug`:

- `content/index.md` -> `dist/index.html`
- `content/about.md` -> `dist/about/index.html`
- `content/blog/post.md` -> `dist/blog/post/index.html`

If you need a custom path, name the file accordingly.

# Config (`shizuka.*`)

Shizuka loads configuration from TOML, YAML, or JSON.

If you do not pass `--config`, Shizuka looks for `shizuka.toml` first, then `shizuka.yaml`, `shizuka.yml`, and `shizuka.json`.

Unknown keys are treated as errors.

## Minimal config

The built-in scaffolds generate a minimal config that relies on Shizuka defaults for most behavior:

```toml
version = "0.1.0"

[site]
title = "My site"
description = "A small static site."
url = "https://example.com/"
```

Equivalent YAML:

```yaml
version: "0.1.0"
site:
  title: "My site"
  description: "A small static site."
  url: "https://example.com/"
```

Equivalent JSON:

```json
{
  "version": "0.1.0",
  "site": {
    "title": "My site",
    "description": "A small static site.",
    "url": "https://example.com/"
  }
}
```

`site.url` is required and must start with `http://` or `https://`.

## Full layout

Shizuka groups config by domain instead of build pipeline steps:

```toml
version = "0.1.0"

[site]
title = "My site"
description = "A small static site."
url = "https://example.com/"

[paths]
output = "dist"
content = "content"
static = "static"
templates = "templates/*.tmpl"

[build]
minify = true

[content.defaults]
template = "page"
section = ""

[content.defaults.params]
author = "Your Name"

[content.markdown]
wikilinks = false

[content.markdown.goldmark]
extensions = ["gfm", "table", "strikethrough", "tasklist", "deflist", "footnotes", "typographer"]

[content.markdown.goldmark.parser]
auto_heading_id = false
attribute = false

[content.markdown.goldmark.renderer]
hardbreaks = false
XHTML = false

[content.bundles]
enabled = false
output = "_assets"
mode = "fingerprinted"

[content.raw]
markdown = false

[content.git]
enabled = false
backfill = true

[headers]
output = "_headers"

[headers.values]
"/" = { "X-Frame-Options" = "DENY" }

[redirects]
output = "_redirects"
shorten = "/s"

[[redirects.entries]]
from = "/old"
to = "/new"
status = 301

[rss]
output = "rss.xml"
sections = ["posts"]
limit = 50
include_drafts = false

[sitemap]
output = "sitemap.xml"
include_drafts = false
```

## Sections

### `paths`

Filesystem locations for the site.

- `output`: build destination root, default `dist`
- `content`: content source root, default `content`
- `static`: static asset source root, default `static`
- `templates`: template glob, default `templates/*.tmpl`

### `build`

Build-wide toggles.

- `minify`: minify rendered HTML and copied assets where supported; default `true`

### `content.defaults`

Defaults applied while indexing pages.

- `template`: fallback template name; default `page`
- `section`: fallback section name
- `params`: merged into page `params`, with frontmatter winning on conflicts

### `content.markdown`

Markdown-specific behavior.

- `wikilinks`: when true, Shizuka rewrites wikilinks such as `[[about]]`, `[[about|About]]`, and `![[hero.png]]`
- `goldmark`: Goldmark parser/renderer configuration

### `content.bundles`

Implicit page bundle asset discovery under `content/`.

- `enabled`: turns bundle asset handling on or off
- `output`: root output path for fingerprinted assets
- `mode`: `"fingerprinted"` or `"adjacent"`

When fingerprinting is enabled, relative Markdown and HTML body links that point at owned bundle assets are rewritten to the emitted asset URL.

### `content.raw`

Controls extra sidecar outputs for content sources.

- `markdown`: when true, Markdown pages also emit a raw `.md` variant such as `posts/hello.md`

### `content.git`

Enables git-backed metadata for content pages and for the site build.

- `enabled`: turns git integration on or off
- `backfill`: when true, missing page `created` and `updated` values are filled from git history

When enabled, Shizuka exposes git metadata on `.Page.Git` and `.Site.Meta.Git`.

### `headers`

Writes a Netlify-style `_headers` file.

- `output`: output filename, default `_headers`
- `values`: base header map keyed by URL path

Per-page `headers` from frontmatter are merged under that page’s URL path, for example `posts/hello`.

### `redirects`

Writes a Netlify-style `_redirects` file.

- `output`: output filename, default `_redirects`
- `shorten`: base path for generated short redirects, default `"/s"`
- `entries`: explicit redirects

Generated redirects also include:

- alias redirects from frontmatter `aliases`
- short redirects for pages in section `"posts"` when `slug` is set

URL paths are site-relative and do not include a leading `/` (for example `posts/hello`).

### `rss`

Builds an RSS feed at `output` (default `rss.xml`).

Only pages matching all of the following are included:

- `rss.include = true` in frontmatter
- `sections` matches one of `rss.sections`
- not a draft unless `include_drafts = true`

### `sitemap`

Builds `sitemap.xml` by default.

Only pages with `sitemap.include = true` are included, and drafts are excluded unless configured otherwise.

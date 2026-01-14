# Config (`shizuka.*`)

Shizuka loads configuration from a config file (default path: `shizuka.toml`).

Supported formats: TOML (`.toml`), YAML (`.yaml` / `.yml`), JSON (`.json`).

Unknown keys are treated as errors.

## Minimal config

The built-in scaffolds generate a minimal config that relies on Shizuka defaults for most build behavior:

```toml
shizuka.version = "0.1.0"

[site]
title = "My site"
description = "A small static site."
url = "https://example.com/"
```

`site.url` is required and must start with `http://` or `https://`.

## Build config

All build options live under `[build]`.

```toml
[build]
output = "dist"
minify = true

[build.steps.static]
source = "static"
destination = "."

[build.steps.content]
source = "content"
destination = "."
template_glob = "templates/*.tmpl"

[build.steps.content.default_params]
author = "Your Name"

[build.steps.content.cascade]
section = "notes"

[build.steps.content.goldmark_config]
extensions = ["gfm", "table", "strikethrough", "tasklist", "deflist", "footnotes", "typographer"]

[build.steps.content.goldmark_config.parser]
auto_heading_id = false
attribute = false

[build.steps.content.goldmark_config.renderer]
hardbreaks = false
XHTML = false
```

`default_params` are merged into each page’s frontmatter `params` (frontmatter wins). `cascade` is merged into each page’s `cascade` field, and then inherited down the content tree (page values win). Params with a leading `_` are treated as unexported and are excluded from `PageLite`.

### `build.steps.headers`

Writes a Netlify-style `_headers` file. The final headers are:

- whatever you put in config, plus
- per-page `headers` from frontmatter (merged under that page’s URL path, e.g. `posts/hello`)

```toml
[build.steps.headers]
output = "_headers"

[build.steps.headers.headers]
"/" = { "X-Frame-Options" = "DENY" }
```

### `build.steps.redirects`

Writes a Netlify-style `_redirects` file.

- `shorten` defaults to `"/s"` (normalized to a leading `/` with no trailing `/`)
- for pages in section `"posts"`, a non-empty `slug` generates a short redirect: `{site.url}{shorten}/{last-slug-segment} -> {page.url_path}`
- you can also specify explicit redirects in config

URL paths are site-relative and do not include a leading `/` (for example `posts/hello`).

```toml
[build.steps.redirects]
output = "_redirects"
shorten = "/s"

[[build.steps.redirects.redirects]]
from = "/old"
to = "/new"
status = 301
```

### `build.steps.rss`

Builds an RSS feed at `output` (default `rss.xml`).

Only pages matching all of the following are included:

- `rss.include = true` in frontmatter
- `sections` matches one of `build.steps.rss.sections`
- not a draft (unless `include_drafts = true`)

```toml
[build.steps.rss]
output = "rss.xml"
sections = ["posts"]
limit = 50
include_drafts = false
```

### `build.steps.sitemap`

Builds `sitemap.xml` by default.

Only pages with `sitemap.include = true` are included, and drafts are excluded unless configured otherwise.

```toml
[build.steps.sitemap]
output = "sitemap.xml"
include_drafts = false
```

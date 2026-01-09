# `shizuka.toml` config

Shizuka loads configuration from a TOML file (default path: `shizuka.toml`).

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

[build.steps.content.default_lite_params]
card_variant = "default"

[build.steps.content.goldmark_config]
extensions = ["gfm", "table", "strikethrough", "tasklist", "deflist", "footnotes", "typographer"]

[build.steps.content.goldmark_config.parser]
auto_heading_id = false
attribute = false

[build.steps.content.goldmark_config.renderer]
hardbreaks = false
XHTML = false
```

### `build.steps.headers`

Writes a Netlify-style `_headers` file. The final headers are:

- whatever you put in config, plus
- per-page `headers` from frontmatter (merged under that page’s canonical path)

```toml
[build.steps.headers]
output = "_headers"

[build.steps.headers.headers]
"/" = { "X-Frame-Options" = "DENY" }
```

### `build.steps.redirects`

Writes a Netlify-style `_redirects` file.

- `shorten` defaults to `"/s"` (normalized to a leading `/` with no trailing `/`)
- every page with a non-empty `slug` gets a generated short redirect: `{site.url}{shorten}/{slug} -> {page.canon}`
- you can also specify explicit redirects in config

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


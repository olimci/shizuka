# Config (`shizuka.*`)

Shizuka loads configuration from a TOML config file (default path: `shizuka.toml`).

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
default_template = "page"

[build.steps.content.git]
enabled = false
backfill = true

[build.steps.content.default_params]
author = "Your Name"

[build.steps.content.goldmark_config]
extensions = ["gfm", "table", "strikethrough", "tasklist", "deflist", "footnotes", "typographer"]

[build.steps.content.goldmark_config.parser]
auto_heading_id = false
attribute = false

[build.steps.content.goldmark_config.renderer]
hardbreaks = false
XHTML = false

[build.steps.content.markdown]
wikilinks = false

[build.steps.content.bundle_assets]
enabled = false
output = "_assets"
mode = "fingerprinted"

[build.steps.content.raw]
markdown = false
```

`default_params` are merged into each page’s frontmatter `params` (frontmatter wins).

`default_template` is used by `shizuka new` when creating generic pages that do not explicitly pick a template. New files created by `shizuka new` use TOML frontmatter by default.

### `build.steps.content.bundle_assets`

Enables implicit page bundle asset discovery under `content/`.

- same-stem sibling files such as `posts/hello.png`
- same-stem directories such as `posts/hello/hero.png`

Supported settings:

- `enabled`: turns bundle asset handling on or off
- `output`: root output path for fingerprinted assets
- `mode`: `"fingerprinted"` or `"adjacent"`

When fingerprinting is enabled, relative Markdown and HTML body links that point at owned bundle assets are rewritten to the emitted asset URL.

### `build.steps.content.markdown`

Markdown-specific authoring options.

- `wikilinks`: when true, Shizuka rewrites wikilinks such as `[[about]]`, `[[about|About]]`, and `![[hero.png]]`

### `build.steps.content.raw`

Controls extra sidecar outputs for content sources.

- `markdown`: when true, Markdown pages also emit a raw `.md` variant such as `posts/hello.md`

### `build.steps.content.git`

Enables git-backed metadata for content pages and for the site build.

- `enabled`: turns git integration on or off
- `backfill`: when true, missing page `date` and `updated` values are filled from git history

When enabled, Shizuka exposes git metadata on `.Page.Git` and `.Site.Meta.Git`.

```toml
[build.steps.content.git]
enabled = true
backfill = true
```

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

- `shorten` defaults to `"/s"`; when set explicitly, the value is used as written except that Shizuka still requires a leading `/` and removes a trailing `/`
- for pages in section `"posts"`, a non-empty `slug` generates a short redirect: `{site.url}{shorten}/{last-slug-segment} -> {page.url_path}`
- per-page `aliases` from frontmatter also generate redirects to that page’s primary `url_path`
- you can also specify explicit redirects in config
- generated short redirects do not change `.Page.Canon`; canonical URLs still point at the page’s actual URL path

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

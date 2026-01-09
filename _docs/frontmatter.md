# Frontmatter + page data

Shizuka uses frontmatter to build a `Page`, then renders it through a template.

## Markdown pages (`content/**/*.md`)

Markdown files must start with a fenced frontmatter block:

- YAML frontmatter: fence with `---`
- TOML frontmatter: fence with `+++`

The fence line must be the very first line of the file (no leading whitespace, no content before it).

### Example (YAML)

```yaml
---
title: "Hello"
description: "A short post."
template: "post"
sections: "posts"
slug: "posts/hello"
date: 2024-01-15
updated: 2024-02-01
tags: ["intro", "shizuka"]
featured: true
draft: false

params:
  hero_image: "/img/hello.jpg"
lite_params:
  card_style: "compact"

headers:
  Cache-Control: "public, max-age=3600"

rss:
  include: true
  guid: "posts/hello"

sitemap:
  include: true
  changefreq: "monthly"
  priority: 0.7
---

Markdown content here.
```

## Structured pages (`*.toml`, `*.yaml`/`*.yml`, `*.json`)

Non-Markdown content files are parsed as a single object and must include:

- `template`: the template name to render
- `body`: a string containing HTML to inject as `.Page.Body`

Example (`content/about.toml`):

```toml
title = "About"
template = "page"
slug = "about"

[sitemap]
include = true

body = """
<p>This is an about page.</p>
"""
```

## Supported frontmatter keys

Top-level:

- `slug` (string): used for redirects and for linking if your templates use it
- `title` (string)
- `description` (string)
- `sections` (string): becomes `.Page.Section` (note the key name is plural)
- `tags` ([]string)
- `date` / `updated` (timestamps)
- `template` (string): required for rendering
- `featured` (bool)
- `draft` (bool)

Nested / maps:

- `params` (map): merged into `.Page.Params` (overrides config defaults)
- `lite_params` (map): merged into `.Page.LiteParams` (overrides config defaults)
- `headers` (map string->string): per-page headers for the headers build step
- `rss`:
  - `include` (bool), `title`, `description`, `guid`
- `sitemap`:
  - `include` (bool), `changefreq`, `priority`


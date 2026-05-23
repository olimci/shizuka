---
title: "Frontmatter"
description: "Page metadata, page data, and structured content."
weight: 40
---

Shizuka uses frontmatter to build a `Page`, then renders it through a template.

Frontmatter values can also be supplied as defaults in
`content.defaults.global` and `content.defaults.sections.<section>` in
`shizuka.jsonc`. Page frontmatter is decoded into the selected defaults, so
explicit page values win.

Page routes are derived from file paths under the content directory. For
example, `content/index.md` renders at `/`, `content/about.md` renders at
`/about/`, and `content/posts/hello.md` renders at `/posts/hello/`.

## Markdown and HTML pages

Markdown and HTML files may start with a frontmatter block:

| Format | Syntax |
| --- | --- |
| YAML | Fence with `---`. |
| TOML | Fence with `+++`. |
| JSON/JSONC | Put a JSON object at the very start of the file, with no fence. Comments are allowed. |

If present, the fence line must be the very first line of the file, with no
leading whitespace and no content before it.

## YAML example

```yaml
---
title: "Hello"
description: "A short post."
template: "post"
section: "posts"
slug: "hello"
created: 2024-01-15
updated: 2024-02-01
weight: 10
tags: ["intro", "shizuka"]
featured: true
draft: false

params:
  hero_image: "/img/hello.jpg"
  _card_style: "compact"

headers:
  Cache-Control: "public, max-age=3600"

rss:
  include: true
  guid: "posts/hello"

sitemap:
  include: true
  changefreq: "monthly"
  priority: 0.7

robots:
  disallow: false
---

Body content here. For `.md` files it is rendered as Markdown. For `.html`
files it is treated as raw HTML and injected into `.Page.Body`.
```

## JSON/JSONC example

```json
{
  // Comments are allowed in object frontmatter.
  "title": "Hello",
  "description": "A short post.",
  "template": "post",
  "section": "posts",
  "slug": "hello",
  "weight": 10,
  "tags": ["intro", "shizuka"],
  "featured": true,
  "draft": false,
  "rss": { "include": true },
  "sitemap": { "include": true, "changefreq": "monthly", "priority": 0.7 },
  "robots": { "disallow": false }
}

Markdown content here.
```

## Structured pages

Non-Markdown content files are parsed as a single object and must include:

| Key | Meaning |
| --- | --- |
| `template` | The template name to render. |
| `body` | A string containing HTML to inject as `.Page.Body`. |
| `body_markdown` | Optional boolean. When true, `body` is rendered as Markdown before it is injected as `.Page.Body`. |

Example `content/about.toml`:

```toml
title = "About"
template = "page"
weight = 20

[sitemap]
include = true

body = """
This is an **about** page.
"""
body_markdown = true
```

## Supported keys

| Key | Type |
| --- | --- |
| `title` | string |
| `description` | string |
| `section` | string, becomes `.Page.Section` |
| `slug` | optional identifier. Explicit slugs must contain only letters, numbers, `_`, and `-`; missing slugs are generated for the build. |
| `tags` | list of strings |
| `created`, `updated` | timestamps |
| `weight` | integer ordering hint; lower values sort earlier |
| `template` | string, required for rendering unless a default supplies it |
| `body` | string, only for structured pages |
| `body_markdown` | boolean, only for structured pages |
| `featured` | boolean |
| `draft` | boolean |
| `params` | arbitrary map merged into `.Page.Params` |
| `headers` | map of per-page headers for the headers build step |
| `rss` | `include`, `title`, `description`, `guid` |
| `sitemap` | `include`, `changefreq`, `priority` |
| `robots` | `disallow` |

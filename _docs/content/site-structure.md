---
title: "Site structure"
description: "What a Shizuka site looks like on disk."
weight: 20
---

A Shizuka site is a directory with a config file and some content:

```text
.
├─ shizuka.jsonc
├─ content/
│  ├─ index.md
│  └─ posts/
│     └─ hello.md
├─ templates/
│  ├─ index.tmpl
│  └─ post.tmpl
├─ data/
│  └─ catalog.jsonc
└─ static/
   └─ style.css
```

## The important folders

| Path | Purpose |
| --- | --- |
| `content/` | Source pages. Markdown pages are converted to HTML and then rendered through a template. |
| `data/` | Optional structured data manifests registered as query tables. |
| `templates/` | Go `html/template` files used to render pages. |
| `static/` | Copied to the output as-is: CSS, images, JavaScript, fonts, and other static files. |
| `dist/` | The default build output folder. |

## How content maps to output URLs

Shizuka outputs pretty URLs by writing `index.html` files into directories.

Given the default config:

| Source | Output |
| --- | --- |
| `content/index.md` | `dist/index.html` |
| `content/about.md` | `dist/about/index.html` |
| `content/posts/hello.md` | `dist/posts/hello/index.html` |
| `content/posts/index.md` | `dist/posts/index.html` |

This mapping is based on the file path and name, not frontmatter.

## Supported content file types

| Extension | Behavior |
| --- | --- |
| `*.md` | Markdown body, optional frontmatter. |
| `*.html` | Raw HTML body, optional frontmatter. |
| `*.toml`, `*.yaml`, `*.yml`, `*.json`, `*.jsonc` | Structured page objects. They must contain `template` and `body`; set `body_markdown = true` to render `body` as Markdown. |

Non-page files under `content/` are ignored. Put CSS, images, JavaScript, fonts,
and other static files under `static/`.

## Data files

If a `data/` directory exists, Shizuka loads `*.toml`, `*.yaml`, `*.yml`,
`*.json`, and `*.jsonc` files as data manifests. Each manifest can define one
or more query tables:

```jsonc
{
  "tables": {
    "products": {
      "name": "shop_products",
      "rows": [
        { "id": "a", "title": "Alpha" },
        { "id": "b", "title": "Beta" }
      ]
    }
  }
}
```

`name` is the public table identifier used in template queries. `rows` may be
an array of objects or a keyed object whose keys are added as a `key` column.

## What gets built

The build is a set of steps configured in the config file:

| Step | Output |
| --- | --- |
| `static` | Copies `static/` into the output. |
| `content` | Builds pages from `content/` and renders them with `templates/`. |
| Optional artefacts | `_headers`, `_redirects`, `rss.xml`, `sitemap.xml`, `robots.txt`, and `404.html`. |

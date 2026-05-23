---
title: "shizuka docs"
description: "A Shizuka-built reference site for Shizuka."
weight: 10
sitemap:
  priority: 1
---

This documentation is itself a Shizuka site. The source lives in `_docs/`, the
content lives in `_docs/content/`, and the shared page template lives in
`_docs/templates/page.tmpl`.

## Build the docs

From the repository root:

```sh
go run . build -c _docs/shizuka.jsonc
```

The built site is written to `_docs/dist/`.

For local development:

```sh
go run . dev -c _docs/shizuka.jsonc
```

## Map

| Page | Purpose |
| --- | --- |
| [Site structure](/site-structure/) | The expected folders and URL mapping for a Shizuka site. |
| [Config](/config/) | The `shizuka.jsonc` shape and supported build artefacts. |
| [Frontmatter](/frontmatter/) | Page metadata, defaults, and structured content formats. |
| [Templates](/templates/) | Template names, page data, site data, and helper functions. |
| [Scaffolds](/scaffolds/) | `shizuka init` template and collection layouts. |
| [Build DAG](/build-step-dag/) | The current high-level build step graph. |

## Design notes

The docs template intentionally follows the embedded debug page: raw HTML,
dark mode, monospace text, white borders, plain tables, and no client-side
framework.

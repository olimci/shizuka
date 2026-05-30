---
title: "Build step DAG"
description: "The current high-level build dependency graph."
weight: 70
---

The build pipeline is modeled as a directed acyclic graph of steps. Page
indexing feeds the page render path and the optional output artefacts.

```mermaid
flowchart TD
  static["static"]

  pages_index["pages:index"]
  git{{"git"}}
  pages_resolve["pages:resolve"]
  pages_render["pages:render"]
  pages_query["pages:query"]
  pages_templates["pages:templates"]
  pages_build["pages:build"]

  headers{{"headers"}}
  redirects{{"redirects"}}
  rss{{"rss"}}
  sitemap{{"sitemap"}}
  robots{{"robots"}}
  not_found{{"not_found"}}

  pages_index --patch--> git
  git --> pages_resolve

  pages_index --> pages_resolve
  pages_resolve --> pages_render
  pages_render --> pages_query
  pages_query --> pages_templates
  pages_templates --> pages_build

  pages_index --> headers

  pages_resolve --> redirects
  pages_resolve --> rss
  pages_resolve --> sitemap
  pages_resolve --> robots

  pages_templates --> not_found
```

The Mermaid source is also available as [`/build-step-dag.mmd`](/build-step-dag.mmd).

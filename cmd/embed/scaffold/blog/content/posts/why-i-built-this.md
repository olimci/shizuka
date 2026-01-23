---
title: "Why I Built This"
description: "The story behind creating a new static site generator and what makes it different."
template: "post"
sections: "posts"
slug: "posts/why-i-built-this"
date: 2024-01-15
tags:
  - meta
  - shizuka
---

I've used many static site generators over the years. Jekyll, Hugo, Eleventy, Next.js, Astro - each has its strengths.

But I wanted something different:

## Fast Builds

Built in Go with concurrent execution. Build times scale linearly with content, not exponentially.

## Simple Configuration

One TOML file. That's it. No plugins to install, no config sprawl.

```toml
[site]
title = "My Site"
url = "https://example.com"

[build]
output = "dist"
```

## Clean Templates

Go templates with helpful functions. No framework-specific syntax to learn.

```html
<h1>{{.Page.Title}}</h1>
<div>{{.Page.Body}}</div>
```

## Live Reload

Save a file, see it update instantly. No full rebuilds, no waiting.

## Developer Experience

- Type-safe configuration
- Helpful error messages
- Fast iteration cycles
- Deploy anywhere

That's why I built Shizuka. Maybe you'll find it useful too.

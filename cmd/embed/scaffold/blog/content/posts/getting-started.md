---
title: "Getting Started with Shizuka"
description: "Learn how to build fast static sites with clean templates and great developer experience."
template: "post"
sections: "posts"
slug: "posts/getting-started"
date: 2024-01-20
tags:
  - shizuka
  - tutorial
  - getting-started
---

Shizuka is a static site generator built for speed and simplicity. It takes your Markdown files and turns them into a blazing-fast website.

## Why Static Sites?

Static sites are:

- **Fast**: No database queries, no server-side rendering
- **Secure**: No backend to hack
- **Scalable**: Serve from a CDN anywhere in the world
- **Simple**: Just HTML, CSS, and JavaScript

## Quick Start

Initialize a new site:

```bash
shizuka init
```

Start the dev server:

```bash
shizuka dev
```

Build for production:

```bash
shizuka build
```

## Writing Content

Create a new post in `content/posts/`:

```markdown
---
title: "My First Post"
date: 2024-01-20
tags: ["hello", "world"]
---

Write your content here using **Markdown**.
```

## What's Next?

- Customize your templates in `templates/`
- Update styles in `static/style.css`
- Add more content in `content/`

Check out the [Shizuka docs](https://github.com/olimci/shizuka) for more.

# Site structure (what a Shizuka site looks like)

A Shizuka site is a directory with (at minimum) a config file and some content:

```
.
├─ shizuka.toml (or `.yaml` / `.yml` / `.json`)
├─ content/
│  ├─ index.md
│  └─ posts/
│     └─ hello.md
├─ templates/
│  ├─ index.tmpl
│  └─ post.tmpl
└─ static/
   └─ style.css
```

## The important folders

- `content/`: source pages. Markdown pages are converted to HTML and then rendered through a template.
- `templates/`: Go `html/template` files used to render pages.
- `static/`: copied to the output as-is (CSS, images, JS, fonts, etc).
- `dist/` (default): the build output folder.

## How content maps to output URLs

Shizuka outputs “pretty URLs” by writing `index.html` files into directories.

Given the default config:

- `content/index.md` -> `dist/index.html`
- `content/about.md` -> `dist/about/index.html`
- `content/posts/hello.md` -> `dist/posts/hello/index.html`
- `content/posts/index.md` -> `dist/posts/index.html`

This mapping is based on the file path and name, not frontmatter.

## Supported content file types

- Markdown pages: `*.md` (requires fenced frontmatter; see `_docs/frontmatter.md`)
- HTML pages: `*.html` (same frontmatter rules as Markdown; body is treated as raw HTML and then rendered through the page template)
- Structured pages: `*.toml`, `*.yaml`/`*.yml`, `*.json` (must contain `template` and `body`)

Non-page files under `content/` are ignored unless they are claimed as bundle assets by a page with the same stem, for example `content/posts/hello.md` owning `content/posts/hello/hero.png`.

## What gets built (high level)

The build is a set of steps configured in your config file (see `_docs/config.md`):

- `static`: copy `static/` into the output
- `content`: build pages from `content/` and render with `templates/`
- optional: `_headers`, `_redirects`, `rss.xml`, `sitemap.xml`

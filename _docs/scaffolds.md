# Scaffolds (`shizuka init`)

Shizuka can scaffold a new site from a “template” directory, or from a “collection” of templates.

## Template scaffold layout

A scaffold template is a directory containing `shizuka.template.*` (`.toml`, `.yaml`/`.yml`, or `.json`) plus the files you want copied into the new project:

```
my-template/
├─ shizuka.template.toml
├─ _gitignore
├─ _shizuka.toml.tmpl
├─ content/
│  └─ index.md
├─ templates/
│  └─ page.tmpl
└─ static/
   └─ style.css
```

## `shizuka.template.*`

```toml
[metadata]
name = "Default"
slug = "default"
description = "Starter theme"
version = "1.0.0"
shizuka_version = "0.1.0"

[files]
strip_suffixes = [".tmpl"]
templates = ["_shizuka.toml.tmpl", "content/**/*.md"]
files = ["**/*"]

[files.renames]
"_shizuka.toml.tmpl" = "shizuka.toml"
"_gitignore" = ".gitignore"

[variables.SiteName]
name = "Site name"
description = "Used for the site title."
default = "My site"
```

### How files are written

When scaffolding:

- everything under the template directory is copied (except the `shizuka.template.*` config)
- any path matching `files.templates` is processed as a Go `text/template` with your variables (e.g. `{{.SiteName}}`)
- filenames are transformed:
  - `files.renames` matches by basename (e.g. `_gitignore` -> `.gitignore`)
  - otherwise, a leading `_` becomes a leading `.` (e.g. `_gitignore` -> `.gitignore`)
  - `files.strip_suffixes` removes the first matching suffix from the basename

Note: `files.files` exists in the config schema, but is not currently used to filter which files are copied.

## Collection scaffold layout

A scaffold collection is a directory containing `shizuka.collection.*` (`.toml`, `.yaml`/`.yml`, or `.json`) and one subdirectory per template:

```
my-collection/
├─ shizuka.collection.toml
├─ default/
│  ├─ shizuka.template.toml
│  └─ ...
└─ minimal/
   ├─ shizuka.template.toml
   └─ ...
```

In a collection, each template’s `metadata.slug` must match its directory name.

## `shizuka.collection.*`

```toml
[metadata]
name = "My templates"
description = "Starter templates"
version = "1.0.0"
shizuka_version = "0.1.0"

[templates]
items = ["default", "minimal"]
default = "default"
```

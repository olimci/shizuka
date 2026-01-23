# shizuka

My static site generator. I made this because I found other static site generators like Hugo and Jekyll annoying to use, and figured I could do a better job lol

## Installation

```sh
go install github.com/olimci/shizuka
```

## Usage

Create a new static site in the current dir using the `init` command

```sh
shizuka init
```

Then start a dev server with `dev`

```sh
shizuka dev
```

When you have made your site, build it with `build`

```sh
shizuka build
```

## Deployment

Deploying is super easy. If you are using cloudflare pages or similar, just select "Custom Framework" and set the build command to:

```sh
go run github.com/olimci/shizuka x build
```

# shizuka

My static site generator. I made this because I found other static site generators like Hugo and Jekyll annoying to use, and figured I could do a better job lol

Currently in alpha, so expect API breakages. Stable release should come soon.

## Installation

```sh
go install github.com/olimci/shizuka@latest
```

## Usage

Clone the example site:

```sh
git clone https://github.com/olimci/shizuka-example-site my-site
cd my-site
```

Then start a dev server with `dev`:

```sh
shizuka dev
```

You can now edit the site and see changes live in the dev server. Press r+enter to rebuild, or q+enter to quit.

When you have made your site, build it with `build`:

```sh
shizuka build
```

## Deployment

Deploying is super easy. If you are using cloudflare pages or similar, just select "Custom Framework" and set the build command to:

```sh
go run github.com/olimci/shizuka@latest build
```

# TODO:

- create `Canon` field of the page, that infers from claim path, slug, and other sources (base URL) to give a good canonical URL for the page. Slugs are used to generate the canonical URL only when shorten is enabled in config, (look at how redirect handles this)
- use slugs more consistently, ie check that slugs are unique, and restrict format to being url-safe. prefill empty slugs with something meaningful and unique
- prefill base URL with "localhost:{port}" when running in development mode (strat is probably to make it an option that overrides config, like withOutput)
- investigate usage of filepath.join where that isn't actually valid (ie should use path.Join)
- investigate if there are any remaining places that utils/lazy would improve performance

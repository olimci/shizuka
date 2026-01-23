# TODO:

- **Cache-aware artefacts**: extend `manifest.Artefact` in `pkg/manifest/artefact.go` with a stable cache key (hash of template name + data fingerprint + source mtime); then skip `AtomicEdit` when the cache says unchanged. This keeps the flat map and avoids tree complexity.
- **Incremental build via file change set**: reuse watcher event paths to filter artefacts by `Claim.Source`/`Claim.Owner` and avoid emitting/rewriting unrelated outputs; you can add optional build options in `pkg/config/options.go`.
- **Build pipeline observability**: add per-step artefact counts and durations in `pkg/build/build.go` or `pkg/events/summary.go` to pinpoint slow steps and cache wins.
- **Template/content dependency graph**: track which pages/templates depend on which sources in `pkg/transforms` and `pkg/build/steps.go` to drive smart rebuilds.


My Recommended Roadmap

Week 1:
1. Add tests for frontmatter parsing and path validation
2. Remove duplicate build functions in transforms/page.go
3. Fix nil pointer check in manifest

Week 2:
4. Finish filesystem abstraction for watcher
5. Add godoc for public APIs
6. Extract common dev server logic

Week 3:
7. Add 10-15 essential template functions
8. Implement template/config caching for dev mode
9. Add build profiling

Month 2:
10. Asset pipeline
11. Partial rebuilds
12. Plugin system

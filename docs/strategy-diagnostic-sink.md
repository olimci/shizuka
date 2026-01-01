# Strategy: Diagnostic Sink for Logging & Non-Fatal Errors

> **Status: Implemented** ✅


## Problem Statement

In `pkg/build/steps.go`, there are several places where we currently return fatal errors but would prefer non-fatal behavior:

```go
// Line 85-86: Template not specified
return fmt.Errorf("%w for page %s", ErrNoTemplate, page.Source)
// TODO: should be non-fatal except on final build

// Line 88-89: Template not found  
return fmt.Errorf("%w (%s) for page %s", ErrTemplateNotFound, page.Template, page.Source)
// TODO: same here

// Line 159: Invalid target path
return err
// TODO: should be nonfatal (warn) really

// Line 165: Page build failure
return err
// TODO: should likely be non-fatal
```

The core issue is that our current step architecture only supports two outcomes:
1. **Success** - step completes, build continues
2. **Fatal error** - step fails, entire build aborts

What we actually need is a richer set of categories:
3. **Debug** - verbose information for troubleshooting
4. **Info** - general build progress information
5. **Warning** - something went wrong, but build should continue
6. **Error** - serious issue, may or may not be fatal depending on mode

Additionally, we want flexibility to:
- Standard logging at various levels (debug, info, warn, error)
- Configure which level causes build failure (`--fail-on-level`)
- Treat certain errors as warnings in dev mode (more lenient rebuilds)
- Aggregate and display all issues at the end of a build
- Stream diagnostics to the UI in real-time during dev server operation
- Filter output by level (e.g., hide debug in prod, show in dev)

## Proposed Solution: Diagnostic Sink

### Core Types

```go
// pkg/build/diagnostic.go

type DiagnosticLevel int

const (
    LevelDebug DiagnosticLevel = iota
    LevelInfo
    LevelWarning
    LevelError
)

func (l DiagnosticLevel) String() string {
    switch l {
    case LevelDebug:
        return "debug"
    case LevelInfo:
        return "info"
    case LevelWarning:
        return "warning"
    case LevelError:
        return "error"
    default:
        return "unknown"
    }
}

// ParseLevel converts a string to a DiagnosticLevel
func ParseLevel(s string) (DiagnosticLevel, error) {
    switch strings.ToLower(s) {
    case "debug":
        return LevelDebug, nil
    case "info":
        return LevelInfo, nil
    case "warning", "warn":
        return LevelWarning, nil
    case "error", "err":
        return LevelError, nil
    default:
        return LevelDebug, fmt.Errorf("unknown level: %s", s)
    }
}

// Diagnostic represents a single build issue
type Diagnostic struct {
    Level   DiagnosticLevel
    StepID  string // Which step reported this
    Source  string // File path or other context
    Message string
    Err     error  // Original error, if any
}

func (d Diagnostic) Error() string {
    if d.Source != "" {
        return fmt.Sprintf("[%s] %s: %s", d.Level, d.Source, d.Message)
    }
    return fmt.Sprintf("[%s] %s", d.Level, d.Message)
}

// DiagnosticSink collects diagnostics during a build
type DiagnosticSink interface {
    // Report adds a diagnostic to the sink
    Report(d Diagnostic)
    
    // Diagnostics returns all collected diagnostics
    Diagnostics() []Diagnostic
    
    // DiagnosticsAtLevel returns diagnostics at or above the given level
    DiagnosticsAtLevel(level DiagnosticLevel) []Diagnostic
    
    // HasLevel returns true if any diagnostics at or above level were reported
    HasLevel(level DiagnosticLevel) bool
    
    // MaxLevel returns the highest severity level reported (or -1 if empty)
    MaxLevel() DiagnosticLevel
    
    // Clear removes all diagnostics (useful between rebuilds)
    Clear()
}
```

### Default Implementation

```go
// pkg/build/diagnostic.go

// DiagnosticCollector is the default thread-safe implementation of DiagnosticSink
type DiagnosticCollector struct {
    mu          sync.RWMutex
    diagnostics []Diagnostic
    minLevel    DiagnosticLevel // Only store diagnostics at or above this level
    
    // Optional callback for real-time streaming
    OnReport func(Diagnostic)
}

type CollectorOption func(*DiagnosticCollector)

func WithMinLevel(level DiagnosticLevel) CollectorOption {
    return func(c *DiagnosticCollector) {
        c.minLevel = level
    }
}

func WithOnReport(fn func(Diagnostic)) CollectorOption {
    return func(c *DiagnosticCollector) {
        c.OnReport = fn
    }
}

func NewDiagnosticCollector(opts ...CollectorOption) *DiagnosticCollector {
    c := &DiagnosticCollector{
        minLevel: LevelDebug, // Collect everything by default
    }
    for _, opt := range opts {
        opt(c)
    }
    return c
}

func (c *DiagnosticCollector) Report(d Diagnostic) {
    // Filter by minimum level
    if d.Level < c.minLevel {
        return
    }
    
    c.mu.Lock()
    c.diagnostics = append(c.diagnostics, d)
    callback := c.OnReport
    c.mu.Unlock()
    
    // Call outside lock to avoid deadlocks
    if callback != nil {
        callback(d)
    }
}

func (c *DiagnosticCollector) Diagnostics() []Diagnostic {
    c.mu.RLock()
    defer c.mu.RUnlock()
    return slices.Clone(c.diagnostics)
}

func (c *DiagnosticCollector) DiagnosticsAtLevel(level DiagnosticLevel) []Diagnostic {
    c.mu.RLock()
    defer c.mu.RUnlock()
    var result []Diagnostic
    for _, d := range c.diagnostics {
        if d.Level >= level {
            result = append(result, d)
        }
    }
    return result
}

func (c *DiagnosticCollector) HasLevel(level DiagnosticLevel) bool {
    c.mu.RLock()
    defer c.mu.RUnlock()
    for _, d := range c.diagnostics {
        if d.Level >= level {
            return true
        }
    }
    return false
}

func (c *DiagnosticCollector) MaxLevel() DiagnosticLevel {
    c.mu.RLock()
    defer c.mu.RUnlock()
    if len(c.diagnostics) == 0 {
        return DiagnosticLevel(-1)
    }
    max := LevelDebug
    for _, d := range c.diagnostics {
        if d.Level > max {
            max = d.Level
        }
    }
    return max
}

func (c *DiagnosticCollector) Clear() {
    c.mu.Lock()
    c.diagnostics = nil
    c.mu.Unlock()
}

// CountByLevel returns a map of level -> count
func (c *DiagnosticCollector) CountByLevel() map[DiagnosticLevel]int {
    c.mu.RLock()
    defer c.mu.RUnlock()
    counts := make(map[DiagnosticLevel]int)
    for _, d := range c.diagnostics {
        counts[d.Level]++
    }
    return counts
}

// Summary returns a human-readable summary of diagnostics
func (c *DiagnosticCollector) Summary() string {
    counts := c.CountByLevel()
    if len(counts) == 0 {
        return "no diagnostics"
    }
    
    var parts []string
    for _, level := range []DiagnosticLevel{LevelError, LevelWarning, LevelInfo, LevelDebug} {
        if count := counts[level]; count > 0 {
            parts = append(parts, fmt.Sprintf("%d %s(s)", count, level))
        }
    }
    return strings.Join(parts, ", ")
}
```

### Integration with Options

```go
// pkg/build/options.go

type Options struct {
    context        context.Context
    configPath     string
    maxWorkers     int
    Dev            bool
    
    // Diagnostic fields
    DiagnosticSink DiagnosticSink
    FailOnLevel    DiagnosticLevel // Build fails if any diagnostic at or above this level
    LenientErrors  bool            // Treat certain errors as warnings (typically enabled in dev)
}

func defaultOptions() *Options {
    return &Options{
        context:     context.Background(),
        configPath:  "shizuka.toml",
        maxWorkers:  runtime.NumCPU(),
        Dev:         false,
        FailOnLevel: LevelError, // By default, only errors cause failure
    }
}

func WithDiagnosticSink(sink DiagnosticSink) Option {
    return func(o *Options) {
        o.DiagnosticSink = sink
    }
}

// WithFailOnLevel sets the minimum diagnostic level that causes build failure.
// Common values:
//   - LevelError (default): only errors cause failure
//   - LevelWarning: warnings and errors cause failure (strict mode)
//   - LevelInfo: even info messages cause failure (very strict, rarely used)
func WithFailOnLevel(level DiagnosticLevel) Option {
    return func(o *Options) {
        o.FailOnLevel = level
    }
}

// WithFailOnWarnings is a convenience for WithFailOnLevel(LevelWarning)
func WithFailOnWarnings() Option {
    return WithFailOnLevel(LevelWarning)
}

func WithLenientErrors() Option {
    return func(o *Options) {
        o.LenientErrors = true
    }
}
```

### StepContext Helpers

```go
// pkg/build/step.go

type StepContext struct {
    Ctx      context.Context
    Manifest *manifest.Manifest
    Options  *Options
    StepID   string // Add this for diagnostic attribution
    defers   []Step
}

// report is the internal method for reporting diagnostics
func (sc *StepContext) report(level DiagnosticLevel, source, message string, err error) {
    if sc.Options.DiagnosticSink == nil {
        return // No sink configured, silently ignore
    }
    sc.Options.DiagnosticSink.Report(Diagnostic{
        Level:   level,
        StepID:  sc.StepID,
        Source:  source,
        Message: message,
        Err:     err,
    })
}

// Debug reports a debug-level diagnostic. For verbose troubleshooting info.
func (sc *StepContext) Debug(source, message string) {
    sc.report(LevelDebug, source, message, nil)
}

// Debugf is a formatted version of Debug
func (sc *StepContext) Debugf(source, format string, args ...any) {
    sc.Debug(source, fmt.Sprintf(format, args...))
}

// Info reports an info-level diagnostic. For general progress information.
func (sc *StepContext) Info(source, message string) {
    sc.report(LevelInfo, source, message, nil)
}

// Infof is a formatted version of Info
func (sc *StepContext) Infof(source, format string, args ...any) {
    sc.Info(source, fmt.Sprintf(format, args...))
}

// Warn reports a warning diagnostic. Something went wrong but build continues.
func (sc *StepContext) Warn(source, message string, err error) {
    sc.report(LevelWarning, source, message, err)
}

// Warnf is a formatted version of Warn
func (sc *StepContext) Warnf(source, format string, args ...any) {
    sc.Warn(source, fmt.Sprintf(format, args...), nil)
}

// Error reports an error diagnostic. Serious issue.
// Returns nil if LenientErrors is enabled (error demoted to warning).
// Returns an error if the diagnostic should be fatal.
func (sc *StepContext) Error(source, message string, err error) error {
    if sc.Options.DiagnosticSink == nil {
        // No sink, fall back to returning the error
        if err != nil {
            return fmt.Errorf("%s: %s: %w", source, message, err)
        }
        return fmt.Errorf("%s: %s", source, message)
    }
    
    level := LevelError
    if sc.Options.LenientErrors {
        level = LevelWarning
    }
    
    sc.report(level, source, message, err)
    
    if sc.Options.LenientErrors {
        return nil // Demoted to warning, continue build
    }
    if err != nil {
        return fmt.Errorf("%s: %s: %w", source, message, err)
    }
    return fmt.Errorf("%s: %s", source, message)
}

// Errorf is a formatted version of Error
func (sc *StepContext) Errorf(source, format string, args ...any) error {
    return sc.Error(source, fmt.Sprintf(format, args...), nil)
}
```

### Updated Steps Usage

```go
// pkg/build/steps.go

func StepContent() Step {
    build := StepFunc("pages:build", func(sc *StepContext) error {
        // ... existing code ...
        
        for _, page := range pages {
            claim := manifest.Claim{
                Source: page.Source,
                Target: page.Target,
                Owner:  "pages:build",
            }

            if tmpl.Lookup(page.Template) == nil {
                if page.Template == "" {
                    // Report error - may be demoted to warning in dev mode
                    if err := sc.ReportError(page.Source, "no template specified", ErrNoTemplate); err != nil {
                        continue // Skip this page, but don't abort build
                    }
                    continue
                } else {
                    if err := sc.ReportError(page.Source, 
                        fmt.Sprintf("template %q not found", page.Template), 
                        ErrTemplateNotFound); err != nil {
                        continue
                    }
                    continue
                }
            }

            artefact := makeArtefact(page, claim)
            sc.Manifest.Emit(artefact)
        }

        return nil
    }, "pages:resolve")
    
    // ... rest of function ...
    
    return StepFunc("pages:index", func(sc *StepContext) error {
        // ... existing code ...

        for _, rel := range files.Values() {
            source, target, err := makeTarget(config.Build.ContentDir, rel)
            if err != nil {
                // This is just a warning - skip the file
                sc.Warn(rel, "invalid target path", err)
                continue
            }

            if filepath.Ext(source) == ".html" {
                sc.Manifest.Emit(makeStatic("pages:index", source, target))
                continue
            }

            page, err := transforms.BuildPage(source, md)
            if err != nil {
                // Report but potentially continue in dev mode
                if err := sc.ReportError(source, "failed to build page", err); err != nil {
                    continue // Skip this page
                }
                continue
            }

            page.Target = target
            pages[rel] = page
        }

        // ... rest of function ...
    })
}
```

### Build Function Updates

```go
// pkg/build/build.go

func Build(steps []Step, config *Config, opts ...Option) error {
    o := defaultOptions().Apply(opts...)
    
    // Ensure we have a diagnostic sink (use no-op if not provided)
    if o.DiagnosticSink == nil {
        o.DiagnosticSink = &noopSink{}
    }
    
    // ... existing build logic ...
    
    // After all steps complete, check if any diagnostic at or above FailOnLevel was reported
    if o.DiagnosticSink.HasLevel(o.FailOnLevel) {
        maxLevel := o.DiagnosticSink.MaxLevel()
        return fmt.Errorf("%w: %s(s) reported during build (fail-on-level: %s)", 
            ErrBuildFailed, maxLevel, o.FailOnLevel)
    }
    
    return nil
}

// noopSink is used when no sink is provided
type noopSink struct{}

func (noopSink) Report(Diagnostic)                               {}
func (noopSink) Diagnostics() []Diagnostic                       { return nil }
func (noopSink) DiagnosticsAtLevel(DiagnosticLevel) []Diagnostic { return nil }
func (noopSink) HasLevel(DiagnosticLevel) bool                   { return false }
func (noopSink) MaxLevel() DiagnosticLevel                       { return DiagnosticLevel(-1) }
func (noopSink) Clear()                                          {}
```

### Application Integration

```go
// cmd/internal/builder.go

type BuildResult struct {
    Duration    time.Duration
    Error       error
    Reason      string
    Paths       []string
    Number      int
    Diagnostics []build.Diagnostic // Add this
}

func (b *Builder) BuildDev(ctx context.Context) BuildResult {
    start := time.Now()
    
    // In dev mode, collect everything including debug for verbose output
    collector := build.NewDiagnosticCollector(
        build.WithMinLevel(build.LevelDebug),
    )

    steps := []build.Step{
        build.StepStatic(),
        build.StepContent(),
    }

    opts := []build.Option{
        build.WithContext(ctx),
        build.WithMaxWorkers(4),
        build.WithDev(),
        build.WithDiagnosticSink(collector),
        build.WithLenientErrors(),           // Demote errors to warnings in dev
        build.WithFailOnLevel(build.LevelError), // Only fail on actual errors (after demotion, there should be none)
    }

    err := build.Build(steps, b.config, opts...)
    duration := time.Since(start)

    return BuildResult{
        Duration:    duration,
        Error:       err,
        Diagnostics: collector.Diagnostics(),
    }
}

func (b *Builder) Build(ctx context.Context) BuildResult {
    start := time.Now()
    
    // In prod, only collect warnings and above (skip debug/info noise)
    collector := build.NewDiagnosticCollector(
        build.WithMinLevel(build.LevelWarning),
    )

    steps := []build.Step{
        build.StepStatic(),
        build.StepContent(),
    }

    opts := []build.Option{
        build.WithContext(ctx),
        build.WithMaxWorkers(4),
        build.WithDiagnosticSink(collector),
        build.WithFailOnLevel(build.LevelError), // Fail on errors
        // For strict mode: build.WithFailOnLevel(build.LevelWarning)
    }

    err := build.Build(steps, b.config, opts...)
    duration := time.Since(start)

    return BuildResult{
        Duration:    duration,
        Error:       err,
        Diagnostics: collector.Diagnostics(),
    }
}

// BuildStrict is like Build but fails on warnings too
func (b *Builder) BuildStrict(ctx context.Context) BuildResult {
    start := time.Now()
    
    collector := build.NewDiagnosticCollector(
        build.WithMinLevel(build.LevelWarning),
    )

    steps := []build.Step{
        build.StepStatic(),
        build.StepContent(),
    }

    opts := []build.Option{
        build.WithContext(ctx),
        build.WithMaxWorkers(4),
        build.WithDiagnosticSink(collector),
        build.WithFailOnLevel(build.LevelWarning), // Strict: fail on warnings
    }

    err := build.Build(steps, b.config, opts...)
    duration := time.Since(start)

    return BuildResult{
        Duration:    duration,
        Error:       err,
        Diagnostics: collector.Diagnostics(),
    }
}
```

### UI Integration

```go
// cmd/internal/ui.go - updates to model

// levelPrefix returns a display prefix for each diagnostic level
func levelPrefix(level build.DiagnosticLevel) string {
    switch level {
    case build.LevelDebug:
        return "DBG "
    case build.LevelInfo:
        return "INFO"
    case build.LevelWarning:
        return "WARN"
    case build.LevelError:
        return "ERR "
    default:
        return "    "
    }
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // ... existing cases ...
    
    case buildResultMsg:
        m.building = false
        m.buildCount = x.Number
        m.lastReason = x.Reason
        m.lastDur = x.Duration
        m.lastChanged = x.Paths
        
        // Display diagnostics (filter by display level if desired)
        for _, d := range x.Diagnostics {
            prefix := levelPrefix(d.Level)
            if d.Source != "" {
                m.appendLog(fmt.Sprintf("%s %s: %s", prefix, d.Source, d.Message))
            } else {
                m.appendLog(fmt.Sprintf("%s %s", prefix, d.Message))
            }
        }
        
        if x.Error != nil {
            m.lastErr = x.Error.Error()
            m.appendLog(fmt.Sprintf("ERR  build #%d in %s: %v", x.Number, x.Duration.Truncate(time.Millisecond), x.Error))
        } else {
            m.lastErr = ""
            summary := summarizeDiagnostics(x.Diagnostics)
            if summary != "" {
                m.appendLog(fmt.Sprintf("OK   build #%d in %s (%s)", 
                    x.Number, x.Duration.Truncate(time.Millisecond), summary))
            } else {
                m.appendLog(fmt.Sprintf("OK   build #%d in %s", x.Number, x.Duration.Truncate(time.Millisecond)))
            }
        }
        // ... rest ...
}

func summarizeDiagnostics(diagnostics []build.Diagnostic) string {
    counts := make(map[build.DiagnosticLevel]int)
    for _, d := range diagnostics {
        counts[d.Level]++
    }
    
    if len(counts) == 0 {
        return ""
    }
    
    var parts []string
    for _, level := range []build.DiagnosticLevel{build.LevelError, build.LevelWarning, build.LevelInfo, build.LevelDebug} {
        if count := counts[level]; count > 0 {
            parts = append(parts, fmt.Sprintf("%d %s", count, level))
        }
    }
    return strings.Join(parts, ", ")
}
```

## Implementation Plan

### Phase 1: Core Types
1. Create `pkg/build/diagnostic.go` with types and collector
2. Add tests for `DiagnosticCollector`

### Phase 2: Integration
1. Update `Options` with new fields and option functions
2. Update `StepContext` with helper methods
3. Update `Build()` to check diagnostics at end

### Phase 3: Step Updates
1. Update `StepContent()` to use diagnostic helpers
2. Update any other steps that need non-fatal errors

### Phase 4: Application Integration
1. Update `Builder` to create and pass collector
2. Update `BuildResult` to include diagnostics
3. Update UI to display diagnostics

### Phase 5: CLI Flags
1. Add `--fail-on-warnings` flag to build command
2. Document new behavior

## Alternatives Considered

### Event Channel
Using a channel similar to watch events for streaming diagnostics.

**Pros**: Fits existing event-driven architecture, real-time streaming
**Cons**: More complex lifecycle management, harder for aggregation

### Simple Callback
Just `Reporter func(level, msg string)` callback.

**Pros**: Very simple
**Cons**: Less structured, harder to aggregate/filter

### Accumulating Errors
Return `[]error` from steps instead of single error.

**Pros**: Uses standard Go error handling
**Cons**: Loses distinction between warnings/errors, awkward API

## Open Questions

1. **Diagnostic persistence**: Should we persist diagnostics to disk for tooling integration (e.g., JSON output for CI)?
2. **Source locations**: Should `Diagnostic` include line/column info for editor integration?
3. **Step filtering**: Should we support filtering diagnostics by step ID in addition to level?
4. **Colored output**: Should diagnostic display use ANSI colors in terminal output?

## Success Criteria

- [x] Dev server continues running when a page has no template
- [x] Dev server shows warnings in UI without stopping
- [x] Debug/info messages display during dev server operation
- [x] Production build fails on errors (current behavior preserved)
- [x] `--fail-on-level` flag allows configuring failure threshold (via `WithFailOnLevel`)
- [x] `--fail-on-warnings` convenience flag works as shorthand
- [x] Diagnostic collector filters by minimum level
- [x] All diagnostics are aggregated and displayed at build end
- [x] Diagnostic summary shows counts by level

## Additional Features Implemented

### Fallback Template (`WithFallbackTemplate`)

When a page references a missing template, instead of failing, the build can use a fallback template. This is particularly useful in dev mode:

```go
opts := []build.Option{
    build.WithFallbackTemplate(fallbackTmpl),
    build.WithLenientErrors(),
}
```

The fallback template receives full `PageTemplate` data including the new `Meta` and `SiteMeta` fields.

### PageMeta and SiteMeta

Templates now receive additional metadata about the build context:

```go
type PageMeta struct {
    Source   string // Original source file path
    Target   string // Output target path  
    Template string // Template name used to render this page
}

type SiteMeta struct {
    BuildTime string // When the build started (RFC3339)
    Dev       bool   // Whether this is a dev build
}

type PageTemplate struct {
    Page     Page
    Site     Site
    Meta     PageMeta
    SiteMeta SiteMeta
}
```

### Embedded Files (`cmd/embed`)

A new `cmd/embed` package provides embedded files for the CLI:

- `cmd/embed/templates/fallback.html` - A styled fallback template that displays page metadata when the specified template is missing

This scaffolding can be extended for future `shizuka init` or similar commands.

## API Summary

### Levels (lowest to highest severity)

| Level | Use Case | Default Behavior |
|-------|----------|------------------|
| `LevelDebug` | Verbose troubleshooting info | Collected in dev, ignored in prod |
| `LevelInfo` | General progress information | Collected in dev, ignored in prod |
| `LevelWarning` | Something wrong, but recoverable | Always collected, displayed |
| `LevelError` | Serious issue | Causes build failure (unless lenient) |

### StepContext Methods

| Method | Level | Returns Error? |
|--------|-------|----------------|
| `Debug(source, msg)` | Debug | No |
| `Debugf(source, fmt, args...)` | Debug | No |
| `Info(source, msg)` | Info | No |
| `Infof(source, fmt, args...)` | Info | No |
| `Warn(source, msg, err)` | Warning | No |
| `Warnf(source, fmt, args...)` | Warning | No |
| `Error(source, msg, err)` | Error (or Warning if lenient) | Yes (unless lenient) |
| `Errorf(source, fmt, args...)` | Error (or Warning if lenient) | Yes (unless lenient) |

### Build Options

| Option | Description |
|--------|-------------|
| `WithDiagnosticSink(sink)` | Provide a collector for diagnostics |
| `WithFailOnLevel(level)` | Set minimum level that causes build failure |
| `WithFailOnWarnings()` | Shorthand for `WithFailOnLevel(LevelWarning)` |
| `WithLenientErrors()` | Demote errors to warnings (for dev mode) |

### Collector Options

| Option | Description |
|--------|-------------|
| `WithMinLevel(level)` | Only collect diagnostics at or above this level |
| `WithOnReport(fn)` | Callback for real-time streaming |
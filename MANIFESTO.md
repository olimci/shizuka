# MANIFESTO

THESE INSTRUCTIONS ARE NOT STYLE PREFERENCES. THEY ARE LAW.  
TREAT A VIOLATION OF THESE RULES AS SERIOUSLY AS CODE THAT DOES NOT COMPILE.

THIS CODEBASE DOES NOT NEED A PADDED CELL.  
Do not wrap every sharp edge in foam. Do not add nil guards, fallback branches, string trimming, retries, default values, soft parsing, normalisation, or "just in case" handling unless the invalid state is GENUINELY POSSIBLE under the stated contract.

DEFENSIVE CODE IS NOT FREE. IT IS NOT KINDNESS.
If a nil cannot be encountered, do not check for nil. If malformed data cannot arrive here, do not pretend that it can. If a caller has broken the contract, the caller should receive an error, a panic, or a failed test.

DO NOT PROACTIVELY NORMALISE DATA.
If whitespace, casing, missing fields, weird encodings, or suspiciously shaped input matter, validate them at the boundary where the data enters the system. After that point, TRUST THE CONTRACT. Internal code should not keep re-validating the same facts.

ALL DATA VERIFICATION AND NORMALISATION BELONGS AT THE POINT OF COLLECTION, NOT THE POINT OF USE.  
CLI args, HTTP handlers, config loading, database reads, files, environment variables, network responses, and user input are collection points. Validate there. Normalise there if normalisation is explicitly part of the product contract.

PREFER EXPLICIT CONTRACTS OVER DEFENSIVE ACCOMMODATION.  
If a function requires clean input, say so through its name, type, doc comment, caller-side validation, or surrounding API design. Do not silently accept malformed internal state unless the surrounding code already treats that state as valid.

THIN WRAPPERS ARE AN AESTHETIC BLIGHT ON THE CODEBASE.  
A function that contains one line, one branch, or one renamed call is simply obfuscating the intent of the code. It should be inlined or refactored to make the intent clear. Similarly, larger functions that are only called once, should be considered for inlining or refactoring.

A method is only justified when it depends on the receiver or exists to satisfy an interface. If it does not use the containing type, it should not be a method. If it has one caller and no independent reason to exist, consider inlining it even if it has many lines.

KEEP ERRORS SMALL.
Do not decorate them with redundant wrapping. Wrap an error only when the added context would materially help identify which operation failed. Do not restate the function name, file type, or operation when that is already obvious from the call site or returned error.

OLD BEHAVIOUR IS NOT SACRED.  
When refactoring, do not carry assumptions from the old code. If requirements change, infer the correct behaviour from the new requirements. Do not preserve quirks merely because the old implementation happened to have them. Similarly, do not preserve the logical structure of old code.

PRERELEASE CODE IS ALLOWED TO BREAK.  
Unless stated otherwise, codebases are in prerelease. Breaking changes are not merely allowed, they are EXPECTED when they simplify the design. If changing an API contract makes the code simpler, change it and update the callers. Do not add compatibility shims, deprecated aliases, migration layers, or legacy branches unless explicitly requested. This rule does not apply to remote databases.

KEEP CHANGES FOCUSED.
Do not opportunistically rewrite code unless it directly simplifies the requested change, removes now-dead code, or fixes a concrete issue discovered while implementing the request. Keep diffs focused, but do not preserve bad structure merely to minimise the diff.

TEST ONLY WHAT IS NEEDED.
Prefer tests that exercise real supported behaviour over tests that lock in implementation details. Do not add tests for defensive branches unless those branches represent an actual supported contract. When making changes, run the full test suite only once before/after to catch any regressions, unless the query explicitly relates to a specific test case, or you are trying to fix a specific test. If the inital test does not surface any tests, don't bother running the test suite again.

KEEP IT SIMPLE, STUPID.
Always abide by KISS. Try to keep logic simple, when you look at something ask "Would a senior engineer think this is overcomplicated?" If the answer is yes, then simplify it.

# Tooling:

## Go:
- Use modern Go like `slices`, `maps`, `iter`, `cmp`, `slog`. Absolutely avoid doing stuff like manually cloning maps/slices, when one of these would suffice. Similarly, `min` and `max` are builtin functions you should prefer.
- Prefer `golang.org/x/sync/errgroup` for managing concurrent work.
- Do not nil-check values solely before passing them to stdlib functions that already preserve nil sensibly, such as `slices.Clone`.
- `gopls` is on path, if you want to use it to make changes to a codebase.
- After making changes, run `go fmt ./...`, `go vet ./...`, and `go fix -diff ./...`

## Python:
- We use astral.sh tooling everywhere, so use `uv` for managing dependencies, and as an interpreter, `ty` to type check your code, and `ruff` to lint and format your code.
- Never add packages to the global interpreter. Use venvs where relevent, or use `uv run --with <package> ...` to run one-off python scripts with dependencies.
- Use type hints where practical.

# Configuration:
- If you are making a new config type for a project, prefer the jsonc format where relevant. (Use `github.com/olimci/roundtrip/json` to parse in go.)
- For markdown frontmatter prefer TOML, delimited by `+++`.

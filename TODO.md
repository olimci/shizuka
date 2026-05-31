# TODO:

- [ ] Logging/UX:
  - [x] Currently if you split logging out like `shizuka build --debug > debug.log`, the stderr output is not captured in the log file. I think the best UX would be to output stderr to output, but also replicate it to stdout if we are emitting to a file like this.
  - [x] key rendering in logging is kind of ugly as it stands. and the debug color is way too dim.
  - [x] debug logging isn't that useful as it stands, tells you step starts etc, but it would be nice if steps reported more information, like (found x pages) warnings etc.
  - [x] shizuka dev user messages shouldn't be emitted via the logger, as it makes the hints harder to read/less obvious.
  - [x] evaluate if setting raw mode would be valuable here (saves each keybind requiring enter) update: perhaps just disable ICANON

## not for now:

- [ ] extensions plan
  - [ ] registry probably needs some kind of "material" tag, for things that can be serialized and handed to extensions. then if an extension tries to register a read/write of an immaterial type, it will be rejected. We could probably introduce something like immutability with this too.
  - [ ] current plan- 
    - subprocesses with json rpc communication
    - actions include step patching, and uh, idk
    - Make helper libs for go + python, maybe js.
  - [ ] should we store helper libs in this repo (monorepo style, or keep them separate? former would probably want a restructuring of folder, as i feel pkg/ is the place for them, perhaps move everything else under src/?)

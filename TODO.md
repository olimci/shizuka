# TODO:

## not for now:

- [ ] extensions plan
  - [ ] registry probably needs some kind of "material" tag, for things that can be serialized and handed to extensions. then if an extension tries to register a read/write of an immaterial type, it will be rejected. We could probably introduce something like immutability with this too.
  - [ ] current plan- 
    - subprocesses with json rpc communication
    - actions include step patching, and uh, idk
    - Make helper libs for go + python, maybe js.
  - [ ] should we store helper libs in this repo (monorepo style, or keep them separate? former would probably want a restructuring of folder, as i feel pkg/ is the place for them, perhaps move everything else under src/?)

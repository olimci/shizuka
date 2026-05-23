# TODO:

## later:
- [ ] markdown processing
  - [ ] add wikilinks extension to gmmd + options
  - [ ] add syntax highlighting extension to gmmd + options
  - [ ] prescribe options in config more strongly
  - [ ] document splitting, so we can break documents up into sections?
  - [ ] markdown templating

## longterm:
- [ ] extensions plan
  - [ ] registry probably needs some kind of "material" tag, for things that can be serialized and handed to extensions. then if an extension tries to register a read/write of an immaterial type, it will be rejected. We could probably introduce something like immutability with this too.
  - [ ] current plan- 
    - subprocesses with json rpc communication
    - actions include step patching, and uh, idk
    - Make helper libs for go + python, maybe js.

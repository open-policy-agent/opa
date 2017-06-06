1.2.0 / 2014-10-05
------------------

- Fixed usWinAscent/usWinDescent - should not go below ascent/descent.
- Upgraded ByteBuffer internal lib.
- Code cleanup.


1.1.2 / 2013-12-02
------------------

- Fixed crash on SVG with empty <metadata> (#8)
- Fixed descent when input font has descent = 0 (@nfroidure)


1.1.1 / 2013-09-26
------------------

- SVG parser moved to external package
- Speed opts
- Code refactoring/cleanup


1.1.0 / 2013-09-25
------------------

- Rewritten svg parser to improve speed
- API changed, now returns buffer as array/Uint8Array


1.0.7 / 2013-09-22
------------------

- Improved speed x2.5 times


1.0.6 / 2013-09-12
------------------

- Improved handling glyphs without codes or names
- Fixed crash on glyphs with `v`/`h` commands
- Logic cleanup


1.0.5 / 2013-08-27
------------------

- Added CLI option `-c` to set copyright string
- Fixed crash when some metrics missed in source SVG
- Minor code cleanup


1.0.4 / 2013-08-09
------------------

- Fixed importing into OSX Font Book


1.0.3 / 2013-08-02
------------------

- Fixed maxp table max points count (solved chrome problems under windozze)


1.0.2 / 2013-07-24
------------------

- Fixed htmx table size
- Fixed loca table size for long format
- Fixed glyph bounding boxes writing


1.0.1 / 2013-07-24
------------------

- Added options support
- Added `ttfinfo` utility
- Multiple fixes


1.0.0 / 2013-07-19
------------------

- First release


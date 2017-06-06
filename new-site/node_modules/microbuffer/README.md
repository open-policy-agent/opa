microbuffer
===========

[![Build Status](https://img.shields.io/travis/fontello/microbuffer/master.svg?style=flat)](https://travis-ci.org/fontello/microbuffer)
[![NPM version](https://img.shields.io/npm/v/microbuffer.svg?style=flat)](https://www.npmjs.org/package/microbuffer)

> Light implementation of binary buffer with helpers for easy access.

This library was written for fontello's font convertors -
[svg2ttf](https://github.com/fontello/svg2ttf)
[ttf2woff](https://github.com/fontello/ttf2woff)
[ttf2eot](https://github.com/fontello/ttf2eot). Main features are:
- good speed & compact size (no dependencies)
- transparent typed arrays support in browsers
- methods to simplify binary data read/write


API
---

### Constructor

- `new MicroBuffer(microbuffer [, offset, length])` - wrap MicroBuffer
  instanse, sharing the same data.
- `new MicroBuffer(Uint8Array|Array [, offset, length])` - wrap Uint8Array|Array.
- `new MicroBuffer(size)` - create new MicroBuffer of specified size.

### Methods

- `.getUint8(pos)`
- `.getUint16(pos, littleEndian)`
- `.getUint32(pos, littleEndian)`
- `.setUint8(pos, value)`
- `.setUint16(pos, value, littleEndian)`
- `.setUint32(pos, value, littleEndian)`

With position update:

- `.writeUint8(value)`
- `.writeInt8(value)`
- `.writeUint16(value, littleEndian)`
- `.writeInt16(value, littleEndian)`
- `.writeUint32(value, littleEndian)`
- `.writeInt32(value, littleEndian)`
- `.writeUint64(value)`

Other:

- `.seek(pos)`
- `.fill(value)`
- `.writeBytes(Uint8Array|Array)`
- `.toString()`
- `.toArray()`

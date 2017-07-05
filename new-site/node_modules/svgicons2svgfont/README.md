# svgicons2svgfont
> svgicons2svgfont is a simple tool to merge multiple icons to an SVG font.

[![NPM version](https://badge.fury.io/js/svgicons2svgfont.png)](https://npmjs.org/package/svgicons2svgfont) [![Build status](https://secure.travis-ci.org/nfroidure/svgicons2svgfont.png)](https://travis-ci.org/nfroidure/svgicons2svgfont) [![Dependency Status](https://david-dm.org/nfroidure/svgicons2svgfont.png)](https://david-dm.org/nfroidure/svgicons2svgfont) [![devDependency Status](https://david-dm.org/nfroidure/svgicons2svgfont/dev-status.png)](https://david-dm.org/nfroidure/svgicons2svgfont#info=devDependencies) [![Coverage Status](https://coveralls.io/repos/nfroidure/svgicons2svgfont/badge.png?branch=master)](https://coveralls.io/r/nfroidure/svgicons2svgfont?branch=master) [![Code Climate](https://codeclimate.com/github/nfroidure/svgicons2svgfont.png)](https://codeclimate.com/github/nfroidure/svgicons2svgfont)

'rect', 'line', 'circle', 'ellipsis', 'polyline' and 'polygon' shapes will be
 converted to pathes. Multiple pathes will be merged.

Transform attributes support is currenly experimental,
 [report issues if any](https://github.com/nfroidure/svgicons2svgfont/issues/6).

You can test this library with the
 [frontend generator](http://nfroidure.github.io/svgiconfont/).

You may want to convert fonts to icons, if so use
 [svgfont2svgicons](https://github.com/nfroidure/svgifont2svgicons).

## Usage
NodeJS module:
```js
var svgicons2svgfont = require('svgicons2svgfont')
  , fs = require('fs');
  , fontStream = svgicons2svgfont([{
    codepoint: 0xE001,
    stream: fs.createReadStream('icons/icon1.svg')
  }, {
    codepoint: 0xE002,
    stream: fs.createReadStream('icons/icon2.svg')
  }], options);

// Saving in a file
fontStream.pipe(fs.createWriteStream('font/destination/file.svg'))
  .on('finish',function() {
    console.log('Font written !')
  });
```

CLI (install the module globally):
```sh
svgicons2svgfont icons/directory font/destination/file.svg
```

## API

### svgicons2svgfont(options)

#### options.fontName
Type: `String`
Default value: `'iconfont'`

The font family name you want.

#### options.fixedWidth
Type: `Boolean`
Default value: `false`

Creates a monospace font of the width of the largest input icon.

#### options.centerHorizontally
Type: `Boolean`
Default value: `false`

Calculate the bounds of a glyph and center it horizontally.

**Warning:** The bounds calculation is currently a naive implementation that
 may not work for some icons. We need to create a svg-pathdata-draw module on
 top of svg-pathdata to get the real bounds of the icon. It's in on the bottom
 of my to do, but feel free to work on it. Discuss it in the
 [related issue](https://github.com/nfroidure/svgicons2svgfont/issues/18).

#### options.normalize
Type: `Boolean`
Default value: `false`

Normalize icons by scaling them to the height of the highest icon.

#### options.fontHeight
Type: `Number`
Default value: `MAX(icons.height)`
The outputted font height  (defaults to the height of the highest input icon).

#### options.round
Type: `Number`
Default value: `10e12`
Setup SVG path rounding.

#### options.descent
Type: `Number`
Default value: `0`

The font descent. It is usefull to fix the font baseline yourself.

**Warning:**  The descent is a positive value!

The ascent formula is: ascent = fontHeight - descent.

#### options.log
Type: `Function`
Default value: `false`

Allows you to provide your own logging function. Set to `function(){}` to
 impeach logging.

## Build systems

### Grunt plugins

[grunt-svgicons2svgfont](https://github.com/nfroidure/grunt-svgicons2svgfont)
 and [grunt-webfont](https://github.com/sapegin/grunt-webfont).

### Gulp plugins

Try [gulp-iconfont](https://github.com/nfroidure/gulp-iconfont) and
  [gulp-svgicons2svgfont](https://github.com/nfroidure/gulp-svgicons2svgfont).

### Stylus plugin

Use [stylus-iconfont](https://www.npmjs.org/package/stylus-iconfont).

### Mimosa plugin

Use [mimosa-svgs-to-iconfonts](https://www.npmjs.org/package/mimosa-svgs-to-iconfonts).

## CLI alternatives

You can combine this plugin's CLI interface with
 [svg2ttf](https://www.npmjs.com/package/),
 [ttf2eot](https://www.npmjs.com/package/),
 [ttf2woff](https://www.npmjs.com/package/)
 and [ttf2woff2](https://www.npmjs.com/package/).
You can also use [webfonts-generator](https://www.npmjs.com/package/webfonts-generator).

## Stats

[![NPM](https://nodei.co/npm/svgicons2svgfont.png?downloads=true&stars=true)](https://nodei.co/npm/svgicon2svgfont/)
[![NPM](https://nodei.co/npm-dl/svgicons2svgfont.png)](https://nodei.co/npm/svgicon2svgfont/)

## Contributing
Feel free to push your code if you agree with publishing under the MIT license.


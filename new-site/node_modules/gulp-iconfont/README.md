# gulp-iconfont
> Create a SVG/TTF/EOT/WOFF font from several SVG icons with [Gulp](http://gulpjs.com/).

[![NPM version](https://badge.fury.io/js/gulp-iconfont.png)](https://npmjs.org/package/gulp-iconfont) [![Build status](https://secure.travis-ci.org/nfroidure/gulp-iconfont.png)](https://travis-ci.org/nfroidure/gulp-iconfont) [![Dependency Status](https://david-dm.org/nfroidure/gulp-iconfont.png)](https://david-dm.org/nfroidure/gulp-iconfont) [![devDependency Status](https://david-dm.org/nfroidure/gulp-iconfont/dev-status.png)](https://david-dm.org/nfroidure/gulp-iconfont#info=devDependencies) [![Coverage Status](https://coveralls.io/repos/nfroidure/gulp-iconfont/badge.png?branch=master)](https://coveralls.io/r/nfroidure/gulp-iconfont?branch=master) [![Code Climate](https://codeclimate.com/github/nfroidure/gulp-iconfont.png)](https://codeclimate.com/github/nfroidure/gulp-iconfont)

You can test this library with the
 [frontend generator](http://nfroidure.github.io/svgiconfont/).

## Usage

First, install `gulp-iconfont` as a development dependency:

```shell
npm install --save-dev gulp-iconfont
```

Then, add it to your `gulpfile.js`:

```javascript
var iconfont = require('gulp-iconfont');

gulp.task('Iconfont', function(){
  gulp.src(['assets/icons/*.svg'])
    .pipe(iconfont({
      fontName: 'myfont', // required
      appendCodepoints: true // recommended option
    }))
      .on('codepoints', function(codepoints, options) {
        // CSS templating, e.g.
        console.log(codepoints, options);
      })
    .pipe(gulp.dest('www/fonts/'));
});
```

`gulp-iconfont` bundles several plugins to bring a simpler API
 (`gulp-svgicons2svgfont`, `gulp-svg2tff`, `gulp-ttf2eot`, `gulp-ttf2woff`)
 for more flexibility, feel free to use them separately.
 
 If some font glyphs aren't converted properly you should append the `normalize:true` option and a `fontHeight` greater than 1000 (`fontHeight: 1001`)

### Make your CSS

To use this font in your CSS, you could add a mixin like in this
 [real world example](https://github.com/ChtiJS/chtijs.francejs.org/blob/master/documents/less/_icons.less).
 You can also generate your CSS automatically with
 [`gulp-iconfont-css`](https://github.com/backflip/gulp-iconfont-css).
 
It's also easy to make a CSS template by yourself. Like
 [this example](https://github.com/cognitom/symbols-for-sketch/blob/master/gulpfile.js#L17),
 `gulp-consolidate` is useful to handling
 [such a template](https://github.com/cognitom/symbols-for-sketch/blob/master/templates/fontawesome-style.css).

```javascript
var gulp = require('gulp');
var iconfont = require('gulp-iconfont');
var consolidate = require('gulp-consolidate');

gulp.task('Iconfont', function(){
  gulp.src(['assets/icons/*.svg'])
    .pipe(iconfont({ fontName: 'myfont' }))
    .on('codepoints', function(codepoints, options) {
      gulp.src('templates/myfont.css')
        .pipe(consolidate('lodash', {
          glyphs: codepoints,
          fontName: 'myfont',
          fontPath: '../fonts/',
          className: 's'
        }))
        .pipe(gulp.dest('www/css/'));
    })
    .pipe(gulp.dest('www/fonts/'));
});
```

### Font hinting

You may also want to hint your TTF fonts, you can use `gulp-spawn`,
 `gulp-filter` and `ttfautohint` for that matter. First install `ttfautohint`
 (use at least the 0.93 version), then, in your gulpfile:
```js
var ttfFilter = filter('*.ttf');

  gulp.src(['assets/icons/*.svg'])
  .pipe(iconfont(/* ... */))
  .pipe(ttfFilter)
  .pipe(spawn({
    cmd: '/bin/sh',
    args: [
      '-c',
      'cat | ttfautohint /dev/stdin /dev/stdout | cat'
    ]
  }))
  .pipe(ttfFilter.restore())
  .pipe(gulp.dest('www/fonts/'))
```

## Issues

Add issues to the right repos:
* the plugin doesn't work at all, submit your issue in this repo.
* every fonts doens't display as expected: submit the issue to the
 [svgicons2svgfont](https://github.com/nfroidure/svgicons2svgfont) repository.
* only some fonts are damaged? Please look at the font format the targeted
 browser actually use and then, submit you issue to one of those projects:
 [svg2ttf](https://github.com/fontello/svg2ttf),
 [ttf2eot](https://github.com/fontello/ttf2eot),
 [ttf2woff](https://github.com/fontello/ttf2woff).

## API

### iconfont(options)

#### options.*
The [svgicons2svgfont](https://github.com/nfroidure/svgicons2svgfont#svgicons2svgfontoptions)
 are available:
* options.fontName
* options.fixedWidth
* options.centerHorizontally
* options.normalize
* options.fontHeight
* options.descent
* options.log

So are the [gulp-svgicons2svgfont](https://github.com/nfroidure/gulp-svgicons2svgfont#svgicons2svgfontoptions):
* options.appendCodepoints

## Preparing SVG's

Beware that your SVG icons must have a high enough height. 500 is a minimum. If
 you do not want to resize them, you can try to combine the `fontHeight` and
 the `normalize` option to get them in a correct size.

### Inkscape
Degroup every shapes (Ctrl+Shift+G), convert to pathes (Ctrl+Maj+C)  and merge
 them (Ctrl++). Then save you SVG, prefer 'simple SVG' file type.

### Illustrator

Save your file as SVG with the following settings:

- SVG Profiles: SVG 1.1
- Fonts Type: SVG
- Fonts Subsetting: None
- Options Image Location: Embed
- Advanced Options
  - CSS Properties: Presentation Attributes
  - Decimal Places: 1
  - Encoding: UTF-8
  - Output fewer <tspan> elements: check

Leave the rest unchecked.

More in-depth information: [http://www.adobe.com/inspire/2013/09/exporting-svg-illustrator.html](http://www.adobe.com/inspire/2013/09/exporting-svg-illustrator.html)

### Sketch

[Sketch](http://bohemiancoding.com/sketch/) is a relatively new drawing tool on Mac. With help of [Sketch Tools](http://bohemiancoding.com/sketch/tool/) and [gulp-sketch](https://github.com/cognitom/gulp-sketch), you can directly create fonts from your Sketch file. No need to export intermediate SVGs.

![Directly create fonts from your Sketch file](https://github.com/cognitom/symbols-for-sketch/raw/master/images/webfonts.png)

Here is a sample repo "[Symbols for Sketch](https://github.com/cognitom/symbols-for-sketch)".

0. [Download the zipped repo](https://github.com/cognitom/symbols-for-sketch/archive/master.zip) and extract it.
0. Go to the directory. `$ cd path/to/dir`
0. Install some tools. `$ npm install`
0. Create fonts and CSS `$ gulp symbols`

## Contributing
Feel free to push your code if you agree with publishing under the MIT license.

## Stats
[![NPM](https://nodei.co/npm/gulp-iconfont.png?downloads=true&stars=true)](https://nodei.co/npm/gulp-iconfont/)
[![NPM](https://nodei.co/npm-dl/gulp-iconfont.png)](https://nodei.co/npm/gulp-iconfont/)


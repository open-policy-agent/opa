# gulp-svg2ttf
> Create a TTF font from an SVG font with [Gulp](http://gulpjs.com/).

[![NPM version](https://badge.fury.io/js/gulp-svg2ttf.png)](https://npmjs.org/package/gulp-svg2ttf) [![Build status](https://secure.travis-ci.org/nfroidure/gulp-svg2ttf.png)](https://travis-ci.org/nfroidure/gulp-svg2ttf) [![Dependency Status](https://david-dm.org/nfroidure/gulp-svg2ttf.png)](https://david-dm.org/nfroidure/gulp-svg2ttf) [![devDependency Status](https://david-dm.org/nfroidure/gulp-svg2ttf/dev-status.png)](https://david-dm.org/nfroidure/gulp-svg2ttf#info=devDependencies) [![Coverage Status](https://coveralls.io/repos/nfroidure/gulp-svg2ttf/badge.png?branch=master)](https://coveralls.io/r/nfroidure/gulp-svg2ttf?branch=master) [![Code Climate](https://codeclimate.com/github/nfroidure/gulp-svg2ttf.png)](https://codeclimate.com/github/nfroidure/gulp-svg2ttf)

## Usage

First, install `gulp-svg2ttf` as a development dependency:

```shell
npm install --save-dev gulp-svg2ttf
```

Then, add it to your `gulpfile.js`:

```javascript
var svg2ttf = require('gulp-svg2ttf');

gulp.task('svg2ttf', function(){
  gulp.src(['fonts/*.svg'])
    .pipe(svg2ttf())
    .pipe(gulp.dest('fonts/'));
});
```

## Stats

[![NPM](https://nodei.co/npm/gulp-svg2ttf.png?downloads=true&stars=true)](https://nodei.co/npm/gulp-svg2ttf/)
[![NPM](https://nodei.co/npm-dl/gulp-svg2ttf.png)](https://nodei.co/npm/gulp-svg2ttf/)

## API

### svg2ttf(options)

#### options.ignoreExt
Type: `Boolean`
Default value: `false`

Set to true to also convert files that doesn't have the .svg extension.

#### options.clone
Type: `Boolean`
Default value: `false`

Set to true to clone the file before converting him so that it will output the
 original file too.

### Note

You may look after a full Gulp web font workflow, see
 [gulp-iconfont](https://github.com/nfroidure/gulp-iconfont)
  fot that matter.

### Contributing / Issues

Please submit SVG to TTF related issues to the
 [svg2ttf project](https://github.com/fontello/svg2ttf)
 on wich gulp-svg2ttf is built.

This repository issues is only for gulp and gulp tasks related issues.

You may want to contribute to this project, pull requests are welcome if you
 accept to publish under the MIT licence.

# gulp-ttf2woff
> Create a WOFF font from a TTF font with [Gulp](http://gulpjs.com/).

[![NPM version](https://badge.fury.io/js/gulp-ttf2woff.png)](https://npmjs.org/package/gulp-iconfont) [![Build status](https://secure.travis-ci.org/nfroidure/gulp-iconfont.png)](https://travis-ci.org/nfroidure/gulp-iconfont) [![Dependency Status](https://david-dm.org/nfroidure/gulp-iconfont.png)](https://david-dm.org/nfroidure/gulp-iconfont) [![devDependency Status](https://david-dm.org/nfroidure/gulp-iconfont/dev-status.png)](https://david-dm.org/nfroidure/gulp-iconfont#info=devDependencies) [![Coverage Status](https://coveralls.io/repos/nfroidure/gulp-iconfont/badge.png?branch=master)](https://coveralls.io/r/nfroidure/gulp-iconfont?branch=master) [![Code Climate](https://codeclimate.com/github/nfroidure/gulp-iconfont.png)](https://codeclimate.com/github/nfroidure/gulp-iconfont)

## Usage

First, install `gulp-ttf2woff` as a development dependency:

```shell
npm install --save-dev gulp-ttf2woff
```

Then, add it to your `gulpfile.js`:

```javascript
var ttf2woff = require('gulp-ttf2woff');

gulp.task('ttf2woff', function(){
  gulp.src(['fonts/*.ttf'])
    .pipe(ttf2woff())
    .pipe(gulp.dest('fonts/'));
});
```

## API

### ttf2woff(options)

#### options.ignoreExt
Type: `Boolean`
Default value: `false`

Set to true to also convert files that doesn't have the .ttf extension.

#### options.clone
Type: `Boolean`
Default value: `false`

Set to true to clone the file before converting him so that it will output the
 original file too.

## Stats

[![NPM](https://nodei.co/npm/gulp-ttf2woff.png?downloads=true&stars=true)](https://nodei.co/npm/gulp-iconfont/)
[![NPM](https://nodei.co/npm-dl/gulp-ttf2woff.png)](https://nodei.co/npm/gulp-iconfont/)

### Contributing / Issues

Please submit TTF to WOFF related issues to the
 [ttf2woff project](https://github.com/fontello/ttf2woff)
 on wich gulp-ttf2woff is built.

This repository issues is only for gulp and gulp tasks related issues.

You may want to contribute to this project, pull requests are welcome if you
 accept to publish under the MIT licence.

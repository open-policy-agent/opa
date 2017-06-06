# gulp-ttf2eot
> Create a EOT font from a TTF font with [Gulp](http://gulpjs.com/).

[![NPM version](https://badge.fury.io/js/gulp-ttf2eot.png)](https://npmjs.org/package/gulp-ttf2eot) [![Build status](https://secure.travis-ci.org/nfroidure/gulp-ttf2eot.png)](https://travis-ci.org/nfroidure/gulp-ttf2eot) [![Dependency Status](https://david-dm.org/nfroidure/gulp-ttf2eot.png)](https://david-dm.org/nfroidure/gulp-ttf2eot) [![devDependency Status](https://david-dm.org/nfroidure/gulp-ttf2eot/dev-status.png)](https://david-dm.org/nfroidure/gulp-ttf2eot#info=devDependencies) [![Coverage Status](https://coveralls.io/repos/nfroidure/gulp-ttf2eot/badge.png?branch=master)](https://coveralls.io/r/nfroidure/gulp-ttf2eot?branch=master) [![Code Climate](https://codeclimate.com/github/nfroidure/gulp-ttf2eot.png)](https://codeclimate.com/github/nfroidure/gulp-ttf2eot)

## Usage

First, install `gulp-ttf2eot` as a development dependency:

```shell
npm install --save-dev gulp-ttf2eot
```

Then, add it to your `gulpfile.js`:

```javascript
var ttf2eot = require('gulp-ttf2eot');

gulp.task('ttf2eot', function(){
  gulp.src(['fonts/*.ttf'])
    .pipe(ttf2eot())
    .pipe(gulp.dest('fonts/'));
});
```

## API

### ttf2eot(options)

#### options.ignoreExt
Type: `Boolean`
Default value: `false`

Set to true to also convert files that doesn't have the .ttf extension.

#### options.clone
Type: `Boolean`
Default value: `false`

Set to true to clone the file before converting him so that it will output the
 original file too.

### Note

You may look after a full Gulp web font workflow, see
 [gulp-iconfont](https://github.com/nfroidure/gulp-iconfont)
  fot that matter.

## Stats

[![NPM](https://nodei.co/npm/gulp-ttf2eot.png?downloads=true&stars=true)](https://nodei.co/npm/gulp-ttf2eot/)
[![NPM](https://nodei.co/npm-dl/gulp-ttf2eot.png)](https://nodei.co/npm/gulp-ttf2eot/)

### Contributing / Issues

Please submit TTF to EOT related issues to the
 [ttf2eot project](https://github.com/fontello/ttf2eot)
 on wich gulp-ttf2eot is built.

This repository issues is only for gulp and gulp tasks related issues.

You may want to contribute to this project, pull requests are welcome if you
 accept to publish under the MIT licence.

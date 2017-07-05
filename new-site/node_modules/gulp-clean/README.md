Deprecated in favor of [https://github.com/gulpjs/gulp/blob/master/docs/recipes/delete-files-folder.md](https://github.com/gulpjs/gulp/blob/master/docs/recipes/delete-files-folder.md)

# [gulp](https://github.com/wearefractal/gulp)-clean [![Build Status](https://secure.travis-ci.org/peter-vilja/gulp-clean.png?branch=master)](https://travis-ci.org/peter-vilja/gulp-clean) [![NPM version](https://badge.fury.io/js/gulp-clean.png)](http://badge.fury.io/js/gulp-clean)

> Removes files and folders.

## Install

Install with [npm](https://npmjs.org/package/gulp-clean).

```
npm install --save-dev gulp-clean
```

## Examples

```js
var gulp = require('gulp');
var clean = require('gulp-clean');

gulp.task('default', function () {
	return gulp.src('app/tmp', {read: false})
		.pipe(clean());
});
```
Option read:false prevents gulp from reading the contents of the file and makes this task a lot faster. If you need the file and its contents after cleaning in the same stream, do not set the read option to false.

```js
var gulp = require('gulp');
var clean = require('gulp-clean');

gulp.task('default', function () {
	return gulp.src('app/tmp/index.js')
		.pipe(clean({force: true}))
		.pipe(gulp.dest('dist'));
});
```

##### For safety files and folders outside the current working directory can be removed only with option force set to true.

Clean as a dependency:

```js
var gulp = require('gulp');
var clean = require('gulp-clean');

gulp.task('clean-scripts', function () {
  return gulp.src('app/tmp/*.js', {read: false})
    .pipe(clean());
});

gulp.task('scripts', ['clean-scripts'], function () {
  gulp.src('app/scripts/*.js')
    .pipe(gulp.dest('app/tmp'));
});

gulp.task('default', ['scripts']);
```

Make sure to return the stream so that gulp knows the clean task is [asynchronous](https://github.com/gulpjs/gulp/blob/master/docs/API.md#async-task-support) and waits for it to terminate before starting the dependent one.

## License

[MIT](http://en.wikipedia.org/wiki/MIT_License) @ Peter Vilja

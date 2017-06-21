/**
* Compiles SCSS to CSS
**/

'use strict';

var paths = require('./config.js').paths;
var gulp = require('gulp');
var autoprefixer = require('gulp-autoprefixer');
var runSequence = require('run-sequence');
var plumber = require('gulp-plumber');
var onError = require('./on-error.js');

// Paths
var watchPath = 'index.html';
var destPath = 'index.html';

gulp.task('views', function() {
  return gulp.src([watchPath]);
});

gulp.task('views:watch', ['views'], function() {
  return gulp.watch(watchPath).on('change', function() {
    runSequence('views', 'browser-sync-reload');
  });
});

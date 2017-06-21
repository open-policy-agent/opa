/**
 * Compiles src/js to public/js
 **/

'use strict';

var paths = require('./config.js').paths;
var gulp = require('gulp');
var concat = require('gulp-concat');
var runSequence = require('run-sequence');
var plumber = require('gulp-plumber');
var onError = require('./on-error.js');

// Paths
var watchPath = 'js/script.js';
var destPath = paths.dest_scripts;

gulp.task('scripts', function() {
  return gulp.src(watchPath)
    .pipe(plumber({
      errorHandler: onError
    }))
    .pipe(concat('script.js'))
    .pipe(gulp.dest(destPath));
});

gulp.task('scripts:watch', ['scripts'], function() {
  return gulp.watch(watchPath).on('change', function() {
    runSequence('scripts', 'browser-sync-reload');
  });
});

/**
* Compiles SCSS to CSS
**/

'use strict';

var paths = require('./config.js').paths;
var gulp = require('gulp');
var sass = require('gulp-sass');
var autoprefixer = require('gulp-autoprefixer');
var runSequence = require('run-sequence');
var plumber = require('gulp-plumber');
var onError = require('./on-error.js');

// Paths
var watchPath = paths.src_styles + '/**/*.scss';
var destPath = paths.dest_styles;

gulp.task('styles', function() {
  return gulp.src([watchPath])
    .pipe(plumber({
      errorHandler: onError
    }))
    .pipe(sass({
      errLogToConsole: true
    }))
    .pipe(autoprefixer())
    .pipe(gulp.dest(destPath));
});

gulp.task('styles:watch', ['styles'], function() {
  return gulp.watch(watchPath).on('change', function() {
    runSequence('styles', 'browser-sync-reload');
  });
});

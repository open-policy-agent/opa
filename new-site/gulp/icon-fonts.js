/**
 * Compiles JS to dist
 **/

'use strict';

var paths = require('./config.js').paths;
var gulp = require('gulp');
var runSequence = require('run-sequence');
var iconfont = require('gulp-iconfont');
var iconfontCSS = require('gulp-iconfont-css');
var plumber = require('gulp-plumber');
var onError = require('./on-error.js');

// Paths
var watchPath = paths.src_icons + '/**/*.svg';
// relative to 'fonts/icon-fonts' path
var targetPath = '../../src/scss/components/icon/_icon-fonts.scss';
// relative to icon-fonts.scss
var fontPath = '../../fonts/icon-fonts/';
var destPath = paths.dest_iconFonts;
var imagesDestPath = paths.dest_icons;

gulp.task('icon-fonts', ['icon-fonts:copy-to-images'], function() {
  return gulp.src([watchPath])
    .pipe(plumber({
      errorHandler: onError
    }))
    .pipe(iconfontCSS({
      fontName: 'icons',
      targetPath: targetPath,
      fontPath: fontPath
    }))
    .pipe(iconfont({
      fontName: 'icons', // required
      appendCodepoints: true, // recommended option
      normalize: true,
      formats: ['svg', 'ttf', 'eot', 'woff']
    }))
    .on('codepoints', function(codepoints, options) {
      // CSS templating, e.g.
      console.log(codepoints, options);
    })
    .pipe(gulp.dest(destPath));
});

gulp.task('icon-fonts:copy-to-images', function() {
  return gulp.src([watchPath])
    .pipe(gulp.dest(imagesDestPath));
});

gulp.task('icon-fonts:watch', ['icon-fonts'], function() {
  return gulp.watch(watchPath).on('change', function() {
    runSequence('icon-fonts', 'styles', 'browser-sync-reload');
  });
});

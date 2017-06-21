/**
 * Browser Sync setup and tasks
 **/

'use strict';

var gulp = require('gulp');
var browserSync = require('browser-sync');
var config = require('./config.js');

var baseDir = config.paths.dest;

gulp.task('browser-sync', function() {
  browserSync.init({
    'notify': true,
    'server': {
      'baseDir': baseDir
    }
  });
});

gulp.task('browser-sync-reload', function() {
  browserSync.reload();
});

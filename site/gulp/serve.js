/**
 * Serve site
 **/

'use strict';

var gulp = require('gulp');
var runSequence = require('run-sequence');

gulp.task('serve', function(cb) {
  return runSequence(
    ['compile', 'browser-sync', 'watch'],
    cb
  );
});

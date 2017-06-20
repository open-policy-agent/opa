/**
 * Build site
 **/

'use strict';

var gulp = require('gulp');
var build = require('gulp-build');

gulp.task('build', function() {
  gulp.src('js/*')
      .pipe(gulp.dest('deploy/js'));

  gulp.src('css/*.css')
      .pipe(gulp.dest('deploy/css'));

  gulp.src('images/*')
      .pipe(gulp.dest('deploy/images'));

  gulp.src('fonts/*')
      .pipe(gulp.dest('deploy/fonts'));

  gulp.src('*.html')
      .pipe(gulp.dest('deploy'));

  gulp.src('doc/_book/*.html')
      .pipe(gulp.dest('deploy/docs'));

  gulp.src('doc/_book/*.json')
      .pipe(gulp.dest('deploy/docs'));

  gulp.src('doc/_book/gitbook/*/**')
      .pipe(gulp.dest('deploy/docs/gitbook'));

  gulp.src('doc/_book/gitbook/*')
      .pipe(gulp.dest('deploy/docs/gitbook'));

  gulp.src('doc/_book/images/*')
      .pipe(gulp.dest('deploy/docs/images'));

  gulp.src('doc/_book/js/*')
      .pipe(gulp.dest('deploy/docs/js'));

  gulp.src('doc/_book/styles/*/**')
      .pipe(gulp.dest('deploy/docs/styles'));

  gulp.src('doc/_book/styles/*')
      .pipe(gulp.dest('deploy/docs/styles'));

});

gulp.task('doc-build', function() {

  gulp.src('doc/_book/*.html')
      .pipe(gulp.dest('docs/'));

  gulp.src('doc/_book/*.json')
      .pipe(gulp.dest('docs/'));

  gulp.src('doc/_book/gitbook/*/**')
      .pipe(gulp.dest('docs/gitbook'));

  gulp.src('doc/_book/gitbook/*')
      .pipe(gulp.dest('docs/gitbook'));

  gulp.src('doc/_book/images/*')
      .pipe(gulp.dest('docs/images'));

  gulp.src('doc/_book/js/*')
      .pipe(gulp.dest('docs/js'));

  gulp.src('doc/_book/styles/*/**')
      .pipe(gulp.dest('docs/styles'));

  gulp.src('doc/_book/styles/*')
      .pipe(gulp.dest('docs/styles'));

});

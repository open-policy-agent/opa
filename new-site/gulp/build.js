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

  gulp.src('*.html')
      .pipe(gulp.dest('deploy'));

  gulp.src('doc/_book/*.html')
      .pipe(gulp.dest('deploy/doc/_book'));

  gulp.src('doc/_book/*.json')
      .pipe(gulp.dest('deploy/doc/_book'));

  gulp.src('doc/_book/gitbook/*/**')
      .pipe(gulp.dest('deploy/doc/_book/gitbook'));

  gulp.src('doc/_book/gitbook/*')
      .pipe(gulp.dest('deploy/doc/_book/gitbook'));

  gulp.src('doc/_book/images/*')
      .pipe(gulp.dest('deploy/doc/_book/images'));

  gulp.src('doc/_book/js/*')
      .pipe(gulp.dest('deploy/doc/_book/js'));

  gulp.src('doc/_book/styles/*/**')
      .pipe(gulp.dest('deploy/doc/_book/styles'));

  gulp.src('doc/_book/styles/*')
      .pipe(gulp.dest('deploy/doc/_book/styles'));

});

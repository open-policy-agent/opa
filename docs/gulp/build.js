/**
 * Build site
 **/

'use strict';

var gulp = require('gulp');
var build = require('gulp-build');

gulp.task('build', function() {
  gulp.src('js/*')
      .pipe(gulp.dest('_site/js'));

  gulp.src('css/*.css')
      .pipe(gulp.dest('_site/css'));

  gulp.src('images/*')
      .pipe(gulp.dest('_site/images'));

  gulp.src('fonts/*')
      .pipe(gulp.dest('_site/fonts'));

  gulp.src('*.html')
      .pipe(gulp.dest('_site'));

  gulp.src('book/_book/*.html')
      .pipe(gulp.dest('_site/docs'));

  gulp.src('book/_book/*.json')
      .pipe(gulp.dest('_site/docs'));

  gulp.src('book/_book/gitbook/*/**')
      .pipe(gulp.dest('_site/docs/gitbook'));

  gulp.src('book/_book/gitbook/*')
      .pipe(gulp.dest('_site/docs/gitbook'));

  gulp.src('book/_book/images/*')
      .pipe(gulp.dest('_site/docs/images'));

  gulp.src('book/_book/js/*')
      .pipe(gulp.dest('_site/docs/js'));

  gulp.src('book/_book/styles/*/**')
      .pipe(gulp.dest('_site/docs/styles'));

  gulp.src('book/_book/styles/*')
      .pipe(gulp.dest('_site/docs/styles'));

});

gulp.task('copy-book', function() {

  gulp.src('book/_book/*.html')
      .pipe(gulp.dest('docs/'));

  gulp.src('book/_book/*.json')
      .pipe(gulp.dest('docs/'));

  gulp.src('book/_book/gitbook/*/**')
      .pipe(gulp.dest('docs/gitbook'));

  gulp.src('book/_book/gitbook/*')
      .pipe(gulp.dest('docs/gitbook'));

  gulp.src('book/_book/images/*')
      .pipe(gulp.dest('docs/images'));

  gulp.src('book/_book/js/*')
      .pipe(gulp.dest('docs/js'));

  gulp.src('book/_book/styles/*/**')
      .pipe(gulp.dest('docs/styles'));

  gulp.src('book/_book/styles/*')
      .pipe(gulp.dest('docs/styles'));

});

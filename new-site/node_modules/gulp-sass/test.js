var gulp = require('gulp');
var sass = require('.');

gulp.task('sass', function() {
    gulp.src('*.scss')
        .pipe(sass({
            sourceComments: true,
            outputStyle: 'expanded',
            errLogToConsole: true
        }))
        .pipe(gulp.dest('dest'));
});

gulp.task('watch', ['sass'], function() {
    var sassWatcher = gulp.watch('*.scss', ['sass']);
});

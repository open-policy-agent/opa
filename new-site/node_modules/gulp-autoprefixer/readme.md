# gulp-autoprefixer [![Build Status](https://travis-ci.org/sindresorhus/gulp-autoprefixer.svg?branch=master)](https://travis-ci.org/sindresorhus/gulp-autoprefixer)

> Prefix CSS with [Autoprefixer](https://github.com/postcss/autoprefixer)

*Issues with the output should be reported on the Autoprefixer [issue tracker](https://github.com/postcss/autoprefixer/issues).*

---

<p align="center"><b>🔥 Want to strengthen your core JavaScript skills and master ES6?</b><br>I would personally recommend this awesome <a href="https://ES6.io/friend/AWESOME">ES6 course</a> by Wes Bos.</p>

---


## Install

```
$ npm install --save-dev gulp-autoprefixer
```


## Usage

```js
const gulp = require('gulp');
const autoprefixer = require('gulp-autoprefixer');

gulp.task('default', () =>
	gulp.src('src/app.css')
		.pipe(autoprefixer({
			browsers: ['last 2 versions'],
			cascade: false
		}))
		.pipe(gulp.dest('dist'))
);
```


## API

### autoprefixer([options])

#### options

See the Autoprefixer [options](https://github.com/postcss/autoprefixer#options).


## Source Maps

Use [gulp-sourcemaps](https://github.com/floridoo/gulp-sourcemaps) like this:

```js
const gulp = require('gulp');
const sourcemaps = require('gulp-sourcemaps');
const autoprefixer = require('gulp-autoprefixer');
const concat = require('gulp-concat');

gulp.task('default', () =>
	gulp.src('src/**/*.css')
		.pipe(sourcemaps.init())
		.pipe(autoprefixer())
		.pipe(concat('all.css'))
		.pipe(sourcemaps.write('.'))
		.pipe(gulp.dest('dist'))
);
```


## Tip

You might want to use Autoprefixer as a [PostCSS plugin](https://github.com/postcss/autoprefixer#gulp) if you use other PostCSS plugins in your build process.


## License

MIT © [Sindre Sorhus](https://sindresorhus.com)

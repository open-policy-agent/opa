# Gulp Sass Changelog

## v2.1.0-beta
**September 21, 2015**

* **Change** :arrow_up: Bump to Node Sass 3.4.0-beta1

## v2.0.4
**July 15, 2015**

* **Fix** Relative file path now uses `file.relative` instead of arcane `split('/').pop` magic. Resolves lots of issues with source map paths.

* **Fix** Empty partials no longer copied to CSS folder

## v2.0.3
**June 27, 2015**

* **Fix** Empty partials no longer copied to CSS folder

## v2.0.2
**June 25, 2015**

* **Fix** Error in watch stream preventing watch from continuing

## v2.0.1
**May 13, 2015**

* **Fix** Source maps now work as expected with Autoprefixer
* **Fix** Current file directory `unshift` onto includePaths stack so it's checked first
* **Fix** Error message returned is unformatted so as to not break other error handling (*i.e.* `gulp-notify`)

## v2.0.0
**May 6, 2015**

* **Change** :arrow_up: Bump to Node Sass 3.0.0

## v2.0.0-alpha.1
**March 26, 2015**

* **New** Added `renderSync` option that can be used through `sass.sync()`

### March 24, 2015
* **Change** Updated to `node-sass` 3.0.0-alpha.1
* **New** Added support for `gulp-sourcemaps` including tests
* **New** Added `.editorconfig` for development consistency
* **New** Added linting and test for said linting
* **Change** Updated the README
* **New** `logError` function to make streaming errors possible instead of breaking the stream

### 1.3.3

* updated to node-sass 2.0 (final)
* should now work with node 0.12 and io.js

### 1.3.2

* fixed errLogToConsole

### 1.3.1

* bug fix

## Version 1.3.0

* Supports node-sass 2.0 (thanks laurelnaiad!)

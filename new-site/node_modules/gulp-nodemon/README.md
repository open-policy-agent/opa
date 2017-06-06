gulp-nodemon
===========

it's gulp + nodemon + convenience

## Install

```bash
$ npm install --save-dev gulp-nodemon
```

## Usage

Gulp-nodemon looks almost exactly like regular nodemon, but it's made for use with gulp tasks.

### **nodemon([options])**

You can pass an object to gulp-nodemon with options [like you would in nodemon config](https://github.com/remy/nodemon#config-files).

Example below will start `server.js` in `development` mode and watch for changes, as well as watch all `.html` and `.js` files in the directory.
```js
gulp.task('start', function () {
  nodemon({
    script: 'server.js'
  , ext: 'js html'
  , env: { 'NODE_ENV': 'development' }
  })
})
```

## Synchronous Build Tasks

*NOTE: This feature requires Node v0.12 because of `child_process.spawnSync`.*

Nodemon is powerful but lacks the ability to compile/cleanup code prior to restarting the application... until now! Most build systems can never be complete without compilation, and now it works harmoniously with your nodemon loop.

### **{ tasks: [Array || Function(changedFiles)] }**

If you want to lint your code when you make changes that's easy to do with a simple event. But what if you need to wait while your project re-builds before you start it up again? This isn't possible with vanilla nodemon, and can be tedious to implement yourself, but it's easy with gulp-nodemon:
```js
nodemon({
  script: 'index.js'
, tasks: ['browserify']
})
```

What if you want to decouple your build processes by language? Or even by file? Easy, just set the `tasks` option to a function. Gulp-nodemon will pass you the list of changed files and it'll let you return a list of tasks you want run.
```js
nodemon({
  script: './index.js'
, ext: 'js css'
, tasks: function (changedFiles) {
    var tasks = []
    changedFiles.forEach(function (file) {
      if (path.extname(file) === '.js' && !~tasks.indexOf('lint')) tasks.push('lint')
      if (path.extname(file) === '.css' && !~tasks.indexOf('cssmin')) tasks.push('cssmin')
    })
    return tasks
  }
})
```

## Events

gulp-nodemon returns a stream just like any other NodeJS stream, **except for the `on` method**, which conveniently accepts gulp task names in addition to the typical function.

### **.on([event], [Array || Function])**

1. `[event]` is an event name as a string. See [nodemon events](https://github.com/remy/nodemon/blob/master/doc/events.md).
2. `[tasks]` An array of gulp task names or a function to execute.

### **.emit([event])**
1. `event`   is an event name as a string. See [nodemon events](https://github.com/remy/nodemon/blob/master/doc/events.md#using-nodemon-events).

## Examples

### Basic Usage

The following example will run your code with nodemon, lint it when you make changes, and log a message when nodemon runs it again.

```js
// Gulpfile.js
var gulp = require('gulp')
  , nodemon = require('gulp-nodemon')
  , jshint = require('gulp-jshint')

gulp.task('lint', function () {
  gulp.src('./**/*.js')
    .pipe(jshint())
})

gulp.task('develop', function () {
  var stream = nodemon({ script: 'server.js'
          , ext: 'html js'
          , ignore: ['ignored.js']
          , tasks: ['lint'] })

  stream
      .on('restart', function () {
        console.log('restarted!')
      })
      .on('crash', function() {
        console.error('Application has crashed!\n')
         stream.emit('restart', 10)  // restart the server in 10 seconds
      })
})
```

_**You can also plug an external version or fork of nodemon**_
```js
gulp.task('pluggable', function() {
  nodemon({ nodemon: require('nodemon'),
            script: 'server.js'})
})
```

### Bunyan Logger integration

The [bunyan](https://github.com/trentm/node-bunyan/) logger includes a `bunyan` script that beautifies JSON logging when piped to it. Here's how you can you can pipe your output to `bunyan` when using `gulp-nodemon`:

```js
gulp.task('run', ['default', 'watch'], function() {
    var nodemon = require('gulp-nodemon'),
        spawn   = require('child_process').spawn,
        bunyan

    nodemon({
        script: paths.server,
        ext:    'js json',
        ignore: [
            'var/',
            'node_modules/'
        ],
        watch:    [paths.etc, paths.src],
        stdout:   false,
        readable: false
    })
    .on('readable', function() {

        // free memory
        bunyan && bunyan.kill()

        bunyan = spawn('./node_modules/bunyan/bin/bunyan', [
            '--output', 'short',
            '--color'
        ])

        bunyan.stdout.pipe(process.stdout)
        bunyan.stderr.pipe(process.stderr)

        this.stdout.pipe(bunyan.stdin)
        this.stderr.pipe(bunyan.stdin)
    });
})
```

## Using `gulp-nodemon` with React, Browserify, Babel, ES2015, etc.

Gulp-nodemon is made to work with the "groovy" new tools like Babel, JSX, and other JavaScript compilers/bundlers/transpilers.

In gulp-nodemon land, you'll want one task for compilation that uses an on-disk cache (e.g. `gulp-file-cache`, `gulp-cache-money`) along with your bundler (e.g. `gulp-babel`, `gulp-react`, etc.). Then you'll put `nodemon({})` in another task and pass the entire compile task in your config:

```js
var gulp = require('gulp')
  , nodemon = require('gulp-nodemon')
  , babel = require('gulp-babel')
  , Cache = require('gulp-file-cache')

var cache = new Cache();

gulp.task('compile', function () {
  var stream = gulp.src('./src/**/*.js') // your ES2015 code
                   .pipe(cache.filter()) // remember files
                   .pipe(babel({ ... })) // compile new ones
                   .pipe(cache.cache()) // cache them
                   .pipe(gulp.dest('./dist')) // write them
  return stream // important for gulp-nodemon to wait for completion
})

gulp.task('watch', ['compile'], function () {
  var stream = nodemon({
                 script: 'dist/' // run ES5 code
               , watch: 'src' // watch ES2015 code
               , tasks: ['compile'] // compile synchronously onChange
               })

  return stream
})
```

The cache keeps your development flow moving quickly and the `return stream` line ensure that your tasks get run in order. If you want them to run async, just remove that line.

## Using `gulp-nodemon` with `browser-sync`

Some people want to use `browser-sync`. That's totally fine, just start browser sync in the same task as `nodemon({})` and use gulp-nodemon's `.on('start', function () {})` to trigger browser-sync. Don't use the `.on('restart')` event because it will fire before your app is up and running.

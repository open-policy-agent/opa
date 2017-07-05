'use strict'

var nodemon
  , colors  = require('colors')
  , gulp    = require('gulp')
  , cp      = require('child_process')
  , bus     = require('nodemon/lib/utils/bus')

module.exports = function (options) {
  options = options || {};

  // plug nodemon
  if (options.nodemon && typeof options.nodemon === 'function') {
    nodemon = options.nodemon
    delete options.nodemon
  } else {
    nodemon = require('nodemon')
  }

  // Our script
  var script            = nodemon(options)
    , originalOn        = script.on
    , originalListeners = bus.listeners('restart')

  // Allow for injection of tasks on file change
  if (options.tasks) {
    // Remove all 'restart' listeners
    bus.removeAllListeners('restart')

    // Place our listener in first position
    bus.on('restart', function (files){
      if (!options.quiet) nodemonLog('running tasks...')

      if (typeof options.tasks === 'function') run(options.tasks(files))
      else run(options.tasks)
    })

    // Re-add all other listeners
    for (var i = 0; i < originalListeners.length; i++) {
      bus.on('restart', originalListeners[i])
    }
  }

  // Capture ^C
  var exitHandler = function (options){
    if (options.exit) script.emit('exit')
    if (options.quit) process.exit(0)
  }
  process.once('exit', exitHandler.bind(null, { exit: true }))
  process.once('SIGINT', exitHandler.bind(null, { quit: true }))

  // Forward log messages and stdin
  script.on('log', function (log){
    nodemonLog(log.message)
  })

  // Shim 'on' for use with gulp tasks
  script.on = function (event, tasks){
    var tasks = Array.prototype.slice.call(arguments)
      , event = tasks.shift()

    if (event === 'change') {
      script.changeTasks = tasks
    } else {
      for (var i = 0; i < tasks.length; i++) {
        void function (tasks){
          if (tasks instanceof Function) {
            originalOn(event, tasks)
          } else {
            originalOn(event, function (){
              if (Array.isArray(tasks)) {
                tasks.forEach(function (task){
                  run(task)
                })
              } else run(tasks)
            })
          }
        }(tasks[i])
      }
    }

    return script
  }

  return script

  // Synchronous alternative to gulp.run()
  function run(tasks) {
    if (typeof tasks === 'string') tasks = [tasks]
    if (tasks.length === 0) return
    if (!(tasks instanceof Array)) throw new Error('Expected task name or array but found: ' + tasks)
    cp.spawnSync(process.platform === 'win32' ? 'gulp.cmd' : 'gulp', tasks, { stdio: [0, 1, 2] })
  }
}

function nodemonLog(message){
  console.log('[' + new Date().toString().split(' ')[4].gray + '] ' + ('[nodemon] ' + message).yellow)
}

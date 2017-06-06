/*============================================================================
 * Copyright(c) 2010 Mario L Gutierrez <mario@mgutz.com>
 * MIT Licensed
 *==========================================================================*/

var temp = require('temp');
var fs = require('fs');
var isWindows = require('os').platform().indexOf('win') === 0;

var shell;
if (isWindows) {
  if (!fs.existsSync(__dirname + '/build/Release/shell.node')) {
    var nodeVersion = process.version;
    var shellLib = 'v0.8/shell';
    if (nodeVersion.indexOf('v0.10') === 0)
      shellLib = 'v0.10/shell';

    try {
      //! Loading an add-on built for another version of nodeJs may
      //! cause seg faults. Windows + open source is not always pleasant
      shell = require('./win32/' + shellLib);
    } catch (err) {
      console.error(err);
      throw new Error('execSync incompatible with installed nodejs');
    }
  }
}

if (!shell) {
  shell = require('./build/Release/shell');
}

/**
 * Runs `cmd` synchronously returning the exit code.
 */
function run(cmd) {
  try {
    if (isWindows)
      cmd = 'cmd /C ' + cmd;
    var code = shell.exec(cmd);
    return code;
  } catch (err) {
    if (err) {
      console.error(err)
    }
    return 1;
  }
}


/**
 * Executes `command` synchronously capturing the output.
 *
 * This is a wrapper around `run` function.
 */
function exec(command) {
  var tempName = temp.path({suffix: '.exec'});
  var cmd;
  if (isWindows)
    cmd = command + ' > ' + tempName + ' 2>&1';
  else
    cmd = '(' + command + ') > ' + tempName + ' 2>&1';

  var code = run(cmd);
  var text;

  if (fs.existsSync(tempName)) {
    try {
      text = fs.readFileSync(tempName, 'utf8');
      fs.unlink(tempName);
    } catch (err) {
      throw new Error('ERROR: could not delete capture file');
    }
  } else {
    throw new Error('ERROR: output not captured');
  }

  return {
    code: code,
    stdout: text
  }
}

module.exports = {
    run: run,
    exec: exec
};

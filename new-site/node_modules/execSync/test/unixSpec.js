/* globals describe, it*/


'use strict';

var assert = require('chai').assert;
var fs = require('fs');
var sh = require('..');

describe('execSync', function() {
  var run = sh.run;

  it ('should perform git operations', function() {
    run('rm -rf tmp');
    run('git clone git://github.com/mgutz/execSync tmp');
    run('cd tmp');
    run('git pull origin master')
    run('cd ..');
    var result = sh.exec('cat tmp/README.md');
    assert.include(result.stdout, 'mgutz');
    assert.equal(result.code, 0);
  });

  it ('should capture stdout, stderr and exit code', function() {
    var result = sh.exec('echo my_bad 1>&2; echo foo; echo your_bad 1>&2; exit 42');
    assert.equal(result.stdout, 'my_bad\nfoo\nyour_bad\n');
    assert.equal(result.code, 42);
  });
});

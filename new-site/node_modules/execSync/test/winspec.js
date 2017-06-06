/* globals describe, it*/
'use strict';

var assert = require('chai').assert;
var fs = require('fs');
var sh = require('..');

describe('exec', function() {

  it ('should perform git operations', function() {
    //! if git bash is used this will fail since it will use bash version not
    //! Windows version
    sh.run('rmdir /S /Q tmp');
    sh.run('git clone git://github.com/mgutz/execSync tmp');
    sh.run('cd tmp');
    sh.run('git pull origin master')
    sh.run('cd ..');
    var result = sh.exec('type tmp\\README.md');
    assert.include(result.stdout, 'mgutz');
    assert.equal(result.code, 0);
  });
});

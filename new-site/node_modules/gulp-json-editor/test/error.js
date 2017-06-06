/* jshint node: true */
/* global it */

var jedit  = require('../');
var gutil  = require('gulp-util');
var fs     = require('fs');
var should = require('should');
require('mocha');

it('should raise error when missing option', function(done) {
  should(function(){jedit();}).throw('missing "editor" option');
  done();
});


it('should raise error when invalid type of option', function(done) {
  should(function(){jedit(1);}).throw('"editor" option must be a function or object');
  done();
});


it('should do path-through when input is null', function(done) {
  jedit({})
    .on('data',  function(file) {
      should(file.contents).eql(null);
      done();
    })
    .write(new gutil.File({}));
});


it('should raise error when streaming input', function(done) {
  jedit({})
    .on('error', function(err) {
      err.message.should.equal('Streaming is not supported');
      done();
    })
    .write(new gutil.File({
      contents: fs.createReadStream('test/test.json')
    }));
});

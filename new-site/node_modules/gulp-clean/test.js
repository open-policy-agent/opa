/*global describe, before, it*/
'use strict';
var fs = require('fs');
var path = require('path');
var gutil = require('gulp-util');
var clean = require('./');
var expect = require('chai').expect;

function noop() {}

describe('gulp-clean plugin', function () {

  var cwd = process.cwd();

  before(function () {
    var exists = fs.existsSync('tmp');
    if (!exists) { fs.mkdirSync('tmp'); }
  });

  function createTree(callback) {
    fs.mkdir('tmp/tree/', function () {
      fs.mkdir('tmp/tree/leaf', function () {
        fs.mkdir('tmp/tree/leaf/node', function () {
          fs.writeFile('tmp/tree/leaf/node/leaf.js', 'console.log("leaf")', function () {
            fs.mkdir('tmp/tree/leftleaf', function () {
              fs.writeFile('tmp/tree/leftleaf/leaf1.js', 'console.log("leaf")', function () {
                callback();
              });
            });
          });
        });
      });
    });
  }

  it('removes a file', function (done) {
    var stream = clean();
    var content = 'testing';
    fs.writeFile('tmp/test.js', content, function () {
      stream.on('data', noop);
      stream.on('end', function () {
        fs.exists('tmp/test.js', function (exists) {
          expect(exists).to.be.false;
          done();
        });
      });

      stream.write(new gutil.File({
        cwd: cwd,
        base: cwd + '/tmp/',
        path: cwd + '/tmp/test.js',
        contents: new Buffer(content)
      }));

      stream.end();
    });
  });

  it('removes a directory', function (done) {
    fs.mkdir('tmp/test', function () {
      var stream = clean();

      stream.on('end', function () {
        fs.exists('tmp/test', function (exists) {
          expect(exists).to.be.false;
          done();
        });
      });

      stream.write(new gutil.File({
        cwd: cwd,
        base: cwd + '/tmp/',
        path: cwd + '/tmp/test/'
      }));
      stream.on('data', noop);
      stream.end();
    });
  });

  it('removes all from the tree', function (done) {
    createTree(function () {
      var stream = clean();

      stream.on('end', function () {
        fs.exists('tmp/tree/leaf/node/leaf.js', function (exists) {
          expect(exists).to.be.false;
          fs.exists('tmp/tree', function (exists) {
            expect(exists).to.be.false;
            done();
          });
        });
      });
      stream.on('data', noop);
      stream.write(new gutil.File({
        cwd: cwd,
        base: cwd + '/tmp',
        path: cwd + '/tmp/tree/'
      }));

      stream.end();
    });
  });

  it('cannot remove the current working directory', function (done) {
    var stream = clean();

    stream.on('error', function () {
      fs.exists('.', function (exists) {
        expect(exists).to.be.true;
      });
    });

    stream.on('end', function () {
      fs.exists('.', function (exists) {
        expect(exists).to.be.true;
        done();
      });
    });

    stream.on('data', noop);
    stream.write(new gutil.File({
      cwd: cwd,
      path: cwd
    }));

    stream.end();
  });

  it('cannot delete anything outside the current working directory', function (done) {
    var stream = clean();

    if (!fs.existsSync('../secrets')) { fs.mkdirSync('../secrets'); }

    stream.on('error', function () {
      fs.exists('../secrets', function (exists) {
        expect(exists).to.be.true;
      });
    });

    stream.on('end', function () {
      fs.exists('../secrets', function (exists) {
        expect(exists).to.be.true;
        done();
      });
    });

    stream.on('data', noop);

    stream.write(new gutil.File({
      cwd: path.resolve(cwd),
      path: path.resolve(cwd + '/../secrets/')
    }));

    stream.end();
  });

  it('cannot delete a folder outside the current working directory', function (done) {
    var stream = clean();

    if (!fs.existsSync('../gulp-cleanTemp')) { fs.mkdirSync('../gulp-cleanTemp'); }

    stream.on('error', function () {
      fs.exists('../gulp-cleanTemp', function (exists) {
        expect(exists).to.be.true;
      });
    });

    stream.on('data', noop);

    stream.on('end', function () {
      fs.exists('../gulp-cleanTemp', function (exists) {
        expect(exists).to.be.true;
        fs.unlink('../gulp-cleanTemp', function () {
          done();
        });
      });
    });

    stream.write(new gutil.File({
      cwd: path.resolve(cwd),
      path: path.resolve(cwd + '/../gulp-cleanTemp/')
    }));

    stream.end();
  });

  it('can delete contents outside the current working directory with option force true', function (done) {
    var stream = clean({force: true});

    if (!fs.existsSync('../gulp-cleanTemp')) { fs.mkdirSync('../gulp-cleanTemp'); }
    stream.on('data', noop);
    stream.on('end', function () {
      fs.exists('../gulp-cleanTemp', function (exists) {
        expect(exists).to.be.false;
        done();
      });
    });

    stream.write(new gutil.File({
      cwd: path.resolve(cwd),
      path: path.resolve(cwd + '/../gulp-cleanTemp/')
    }));

    stream.end();
  });
});

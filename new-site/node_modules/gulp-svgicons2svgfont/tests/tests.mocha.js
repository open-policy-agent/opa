var fs = require('fs');
var gulp = require('gulp');
var gutil = require('gulp-util');
var es = require('event-stream');
var svgicons2svgfont = require('../src/index');
var assert = require('assert');
var rimraf = require('rimraf');
var Stream = require('stream');

describe('gulp-svgicons2svgfont', function() {

  afterEach(function() {
    rimraf.sync(__dirname + '/results');
  });

  describe('with null contents', function() {

    it('should let null files pass through', function(done) {

      var s = svgicons2svgfont({
        fontName: 'cleanicons'
      });
      var n = 0;
      s.pipe(es.through(function(file) {
          assert.equal(file.path,'bibabelula.svg');
          assert.equal(file.contents, null);
          n++;
        }, function() {
          assert.equal(n,1);
          done();
        }));
      s.write(new gutil.File({
        path: 'bibabelula.svg',
        contents: null
      }));
      s.end();

    });

  });

  describe('in stream mode', function() {

    it('should work with cleanicons', function(done) {
      gulp.src(__dirname + '/fixtures/cleanicons/*.svg', {buffer: false})
        .pipe(svgicons2svgfont({
          fontName: 'cleanicons'
        })).on('data', function(file) {
          assert.equal(file.isStream(), true);
          file.pipe(es.wait(function(err, data) {
            assert.equal(err, undefined);
            assert.equal(
              data,
              fs.readFileSync(__dirname + '/expected/test-cleanicons-font.svg', 'utf8')
            );
            done();
          }));
        });
    });

    it('should work with prefixedicons', function(done) {
      gulp.src(__dirname + '/fixtures/prefixedicons/*.svg', {buffer: false})
        .pipe(svgicons2svgfont({
          fontName: 'prefixedicons'
        })).on('data', function(file) {
          assert.equal(file.isStream(), true);
          file.pipe(es.wait(function(err, data) {
            assert.equal(err, undefined);
            assert.equal(
              data,
              fs.readFileSync(__dirname + '/expected/test-prefixedicons-font.svg', 'utf8')
            );
            done();
          }));
        });
    });

    it('should work with originalicons', function(done) {
      gulp.src(__dirname + '/fixtures/originalicons/*.svg', {buffer: false})
        .pipe(svgicons2svgfont({
          fontName: 'originalicons'
        })).on('data', function(file) {
          assert.equal(file.isStream(), true);
          file.pipe(es.wait(function(err, data) {
            assert.equal(err, undefined);
            assert.equal(
              data,
              fs.readFileSync(__dirname + '/expected/test-originalicons-font.svg', 'utf8')
            );
            done();
          }));
        });
    });

    it('should work with unprefixed icons', function(done) {
      var cnt;
      gulp.src(__dirname + '/fixtures/unprefixedicons/*.svg', {buffer: false})
        .pipe(gulp.dest(__dirname + '/results/unprefixedicons/'))
        .pipe(es.wait(function() {
          gulp.src(__dirname + '/results/unprefixedicons/*.svg', {buffer: false})
            .pipe(svgicons2svgfont({
              fontName: 'unprefixedicons',
              appendCodepoints: true
            }))
            .on('data', function(file) {
              assert.equal(file.isStream(), true);
              file.contents.pipe(es.wait(function(err, data) {
                assert.equal(err, undefined);
                cnt = data;
              }));
            })
            .pipe(gulp.dest(__dirname + '/results/'))
            .on('end', function() {
              assert.equal(fs.existsSync(__dirname +
                '/results/unprefixedicons/uE001-arrow-down.svg'), true);
              assert.equal(
                fs.readFileSync(__dirname + '/results/unprefixedicons/uE001-arrow-down.svg', 'utf8'),
                fs.readFileSync(__dirname + '/fixtures/unprefixedicons/arrow-down.svg', 'utf8')
              );
              assert.equal(fs.existsSync(__dirname
                + '/results/unprefixedicons/uE002-arrow-left.svg'), true);
              assert.equal(
                fs.readFileSync(__dirname + '/results/unprefixedicons/uE002-arrow-left.svg', 'utf8'),
                fs.readFileSync(__dirname + '/fixtures/unprefixedicons/arrow-left.svg', 'utf8')
              );
              assert.equal(fs.existsSync(__dirname
                + '/results/unprefixedicons/uE003-arrow-right.svg'), true);
              assert.equal(
                fs.readFileSync(__dirname + '/results/unprefixedicons/uE003-arrow-right.svg', 'utf8'),
                fs.readFileSync(__dirname + '/fixtures/unprefixedicons/arrow-right.svg', 'utf8')
              );
              assert.equal(fs.existsSync(__dirname
                + '/results/unprefixedicons/uE004-arrow-up.svg'), true);
              assert.equal(
                fs.readFileSync(__dirname + '/results/unprefixedicons/uE004-arrow-up.svg', 'utf8'),
                fs.readFileSync(__dirname + '/fixtures/unprefixedicons/arrow-up.svg', 'utf8')
              );
              assert.equal(
                cnt,
                fs.readFileSync(__dirname + '/expected/test-unprefixedicons-font.svg', 'utf8')
              );
              done();
            });
        }));
    });

    it('should let non-svg files pass through', function(done) {

        var s = svgicons2svgfont({
          fontName: 'unprefixedicons',
          appendCodepoints: true
        });
        s.pipe(es.through(function(file) {
            assert.equal(file.path,'bibabelula.foo');
            assert(file.contents instanceof Stream.PassThrough);
          }, function() {
              done();
          }));
        s.write(new gutil.File({
          path: 'bibabelula.foo',
          contents: new Stream.PassThrough('ohyeah')
        }));
        s.end();

    });

    it('should let non-svg files pass through', function(done) {

      var s = svgicons2svgfont({
          fontName: 'unprefixedicons'
        })
        , n = 0;
      s.pipe(es.through(function(file) {
          assert.equal(file.path,'bibabelula.foo');
          assert.equal(file.contents.toString('utf-8'), 'ohyeah');
          n++;
        }, function() {
          assert.equal(n,1);
          done();
        }));
      s.write(new gutil.File({
        path: 'bibabelula.foo',
        contents: new Buffer('ohyeah')
      }));
      s.end();

    });

    it('should emit an event with the codepoint mapping', function(done) {
      var codepoints;
      gulp.src(__dirname + '/fixtures/cleanicons/*.svg', {buffer: false})
        .pipe(svgicons2svgfont({
          fontName: 'cleanicons'
        })).on('codepoints', function(cpts) {
          codepoints = cpts;
        }).on('data', function(file) {
          assert.equal(file.isStream(), true);
          file.pipe(es.wait(function(err, data) {
            assert.equal(err, undefined);
            assert.deepEqual(
              codepoints, JSON.parse(fs.readFileSync(
                __dirname + '/expected/test-codepoints.json', 'utf8')) );
            done();
          }));
        });
    });

  });


  describe('in buffer mode', function() {

    it('should work with cleanicons', function(done) {
      gulp.src('tests/fixtures/cleanicons/*.svg', {buffer: true})
        .pipe(svgicons2svgfont({
          fontName: 'cleanicons'
        })).on('data', function(file) {
            assert.equal(file.isBuffer(), true);
            assert.equal(
              file.contents.toString('utf8'),
              fs.readFileSync(__dirname + '/expected/test-cleanicons-font.svg')
            );
            done();
        });
    });

    it('should work with prefixedicons', function(done) {
      gulp.src(__dirname + '/fixtures/prefixedicons/*.svg', {buffer: true})
        .pipe(svgicons2svgfont({
          fontName: 'prefixedicons'
        })).on('data', function(file) {
            assert.equal(file.isBuffer(), true);
            assert.equal(
              file.contents.toString('utf8'),
              fs.readFileSync(__dirname + '/expected/test-prefixedicons-font.svg','utf8')
            );
            done();
        });
    });

    it('should work with originalicons', function(done) {
      gulp.src(__dirname + '/fixtures/originalicons/*.svg', {buffer: true})
        .pipe(svgicons2svgfont({
          fontName: 'originalicons'
        })).on('data', function(file) {
            assert.equal(file.isBuffer(), true);
            assert.equal(
              file.contents.toString('utf8'),
              fs.readFileSync(__dirname + '/expected/test-originalicons-font.svg', {
                encoding: 'utf8'
              })
            );
            done();
        });
    });

    it('should let non-svg files pass through', function(done) {

        var s = svgicons2svgfont({
          fontName: 'unprefixedicons',
          appendCodepoints: true
        });
        s.pipe(es.through(function(file) {
            assert.equal(file.path,'bibabelula.foo');
            assert.equal(file.contents.toString('utf-8'), 'ohyeah');
          }, function() {
              done();
          }));
        s.write(new gutil.File({
          path: 'bibabelula.foo',
          contents: new Buffer('ohyeah')
        }));
        s.end();

    });

    it('should let non-svg files pass through', function(done) {

      var s = svgicons2svgfont({
          fontName: 'unprefixedicons'
        })
        , n = 0;
      s.pipe(es.through(function(file) {
          assert.equal(file.path,'bibabelula.foo');
          assert(file.contents instanceof Stream.PassThrough);
          n++;
        }, function() {
          assert.equal(n,1);
          done();
        }));
      s.write(new gutil.File({
        path: 'bibabelula.foo',
        contents: new Stream.PassThrough()
      }));
      s.end();

    });

  });


  describe('Using gulp.dest in buffer mode', function() {

    it('should work with cleanicons', function(done) {
      gulp.src(__dirname + '/fixtures/cleanicons/*.svg', {buffer: true})
        .pipe(svgicons2svgfont({
          fontName: 'cleanicons'
        }))
        .pipe(gulp.dest(__dirname + '/results/'))
        .on('end', function() {
            assert.equal(
              fs.readFileSync(__dirname + '/results/cleanicons.svg',{
                encoding: 'utf-8'
              }),
              fs.readFileSync(__dirname + '/expected/test-cleanicons-font.svg',{
                encoding: 'utf-8'
              })
            );
            done();
        });
    });

    it('should work with unprefixed icons (stream)', function(done) {
      gulp.src(__dirname + '/fixtures/unprefixedicons/*.svg', {buffer: true})
        .pipe(gulp.dest(__dirname + '/results/unprefixedicons/'))
        .pipe(es.wait(function() {
          gulp.src(__dirname + '/results/unprefixedicons/*.svg', {buffer: true})
            .pipe(svgicons2svgfont({
              fontName: 'unprefixedicons',
              appendCodepoints: true
            }))
            .pipe(gulp.dest(__dirname + '/results/'))
            .on('data', function(file) {
              assert.equal(fs.existsSync(__dirname
                + '/results/unprefixedicons/uE001-arrow-down.svg'), true);
              assert.equal(
                fs.readFileSync(__dirname + '/results/unprefixedicons/uE001-arrow-down.svg', 'utf8'),
                fs.readFileSync(__dirname + '/fixtures/unprefixedicons/arrow-down.svg', 'utf8')
              );
              assert.equal(fs.existsSync(__dirname
                + '/results/unprefixedicons/uE002-arrow-left.svg'), true);
              assert.equal(
                fs.readFileSync(__dirname + '/results/unprefixedicons/uE002-arrow-left.svg', 'utf8'),
                fs.readFileSync(__dirname + '/fixtures/unprefixedicons/arrow-left.svg', 'utf8')
              );
              assert.equal(fs.existsSync(__dirname
                + '/results/unprefixedicons/uE003-arrow-right.svg'), true);
              assert.equal(
                fs.readFileSync(__dirname + '/results/unprefixedicons/uE003-arrow-right.svg', 'utf8'),
                fs.readFileSync(__dirname + '/fixtures/unprefixedicons/arrow-right.svg', 'utf8')
              );
              assert.equal(fs.existsSync(__dirname
                + '/results/unprefixedicons/uE004-arrow-up.svg'), true);
              assert.equal(
                fs.readFileSync(__dirname + '/results/unprefixedicons/uE004-arrow-up.svg', 'utf8'),
                fs.readFileSync(__dirname + '/fixtures/unprefixedicons/arrow-up.svg', 'utf8')
              );
              assert.equal(file.isBuffer(), true);
              file.pipe(es.wait(function(err, data) {
                assert.equal(err, undefined);
                assert.equal(
                  data,
                  fs.readFileSync(__dirname + '/expected/test-unprefixedicons-font.svg', 'utf8')
                );
                done();
              }));
            });
        }));
    });

  });


  describe('must throw error when not fontname', function() {

    assert.throws(function() {
      svgicons2svgfont();
    });

  });


});

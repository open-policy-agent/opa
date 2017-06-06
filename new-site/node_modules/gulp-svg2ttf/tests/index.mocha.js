'use strict';

var gulp = require('gulp')
  , assert = require('assert')
  , StreamTest = require('streamtest')
  , fs = require('fs')
  , svg2ttf = require(__dirname + '/../src/index.js')
  , Stream = require('stream')
  , gutil = require('gulp-util')
;

// Erasing date to get an invariant created and modified font date
// See: https://github.com/fontello/svg2ttf/blob/c6de4bd45d50afc6217e150dbc69f1cd3280f8fe/lib/sfnt.js#L19
Date = (function(d) {
  function Date() {
    d.call(this, 3600);
  }
  Date.now = d.now;
  return Date;
})(Date);

describe('gulp-svg2ttf conversion', function() {
  var filename = __dirname + '/fixtures/iconsfont';
  var ttf = fs.readFileSync(filename + '.ttf');

  // Iterating through versions
  StreamTest.versions.forEach(function(version) {

    describe('for ' + version + ' streams', function() {

      describe('with null contents', function() {

        it('should let null files pass through', function(done) {
            
            StreamTest[version].fromObjects([new gutil.File({
              path: 'bibabelula.foo',
              contents: null
            })])
            .pipe(svg2ttf())
            .pipe(StreamTest[version].toObjects(function(err, objs) {
              if(err) {
                done(err);
              }
              assert.equal(objs.length, 1);
              assert.equal(objs[0].path, 'bibabelula.foo');
              assert.equal(objs[0].contents, null);
              done();
            }));

        });

      });

      describe('in buffer mode', function() {

        it('should work', function(done) {

          gulp.src(filename + '.svg', {buffer: true})
            .pipe(svg2ttf())
            // Uncomment to regenerate the test files if changes in the svg2ttf lib
            // .pipe(gulp.dest(__dirname + '/fixtures/'))
            .pipe(StreamTest[version].toObjects(function(err, objs) {
              if(err) {
                done(err);
              }
              assert.equal(objs.length, 1);
              assert.equal(objs[0].path, filename + '.ttf');
              assert.equal(objs[0].contents.toString('utf-8'), ttf.toString('utf-8'));
              done();
            }));

        });

        it('should work with the clone option', function(done) {

          gulp.src(filename + '.svg', {buffer: true})
            .pipe(svg2ttf({clone: true}))
            .pipe(StreamTest[version].toObjects(function(err, objs) {
              if(err) {
                done(err);
              }
              assert.equal(objs.length, 2);
              assert.equal(objs[0].path, filename + '.svg');
              assert.equal(objs[0].contents.toString('utf-8'), fs.readFileSync(filename + '.svg','utf-8'));
              assert.equal(objs[1].path, filename + '.ttf');
              assert.equal(objs[1].contents.toString('utf-8'), ttf.toString('utf-8'));
              done();
            }));

        });

        it('should let non-svg files pass through', function(done) {
            
            StreamTest[version].fromObjects([new gutil.File({
              path: 'bibabelula.foo',
              contents: new Buffer('ohyeah')
            })])
            .pipe(svg2ttf())
            .pipe(StreamTest[version].toObjects(function(err, objs) {
                assert.equal(objs.length, 1);
                assert.equal(objs[0].path, 'bibabelula.foo');
                assert.equal(objs[0].contents.toString('utf-8'), 'ohyeah');
                done();
            }));

        });
      });


      describe('in stream mode', function() {
        it('should work', function(done) {

          gulp.src(filename + '.svg', {buffer: false})
            .pipe(svg2ttf())
            .pipe(StreamTest[version].toObjects(function(err, objs) {
              if(err) {
                done(err);
              }
              assert.equal(objs.length, 1);
              assert.equal(objs[0].path, filename + '.ttf');
              objs[0].contents.pipe(StreamTest[version].toText(function(err, text) {
                assert.equal(text, ttf.toString('utf-8'));
                done();
              }));
            }));

        });

        it('should work with the clone option', function(done) {

          gulp.src(filename + '.svg', {buffer: false})
            .pipe(svg2ttf({clone: true}))
            .pipe(StreamTest[version].toObjects(function(err, objs) {
              if(err) {
                done(err);
              }
              assert.equal(objs.length, 2);
              assert.equal(objs[0].path, filename + '.svg');
              assert.equal(objs[1].path, filename + '.ttf');
              objs[0].contents.pipe(StreamTest[version].toText(function(err, text) {
                assert.equal(text, fs.readFileSync(filename + '.svg','utf-8'));
                objs[1].contents.pipe(StreamTest[version].toText(function(err, text) {
                  assert.equal(text, ttf.toString('utf-8'));
                  done();
                }));
              }));
            }));

        });

        it('should let non-svg files pass through', function(done) {
            
          StreamTest[version].fromObjects([new gutil.File({
            path: 'bibabelula.foo',
            contents: new Stream.PassThrough()
          })])
          .pipe(svg2ttf())
          .pipe(StreamTest[version].toObjects(function(err, objs) {
            if(err) {
              done(err);
            }
            assert.equal(objs.length, 1);
            assert.equal(objs[0].path, 'bibabelula.foo');
            assert(objs[0].contents instanceof Stream.PassThrough);
            done();
          }));

        });
      });

    });

  });

});

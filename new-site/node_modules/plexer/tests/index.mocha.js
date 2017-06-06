var assert = require('assert')
  , es = require('event-stream')
  , Duplexer = require('../src')
  , PlatformStream = require('stream')
  , Stream = require('readable-stream')
;

// Tests
[PlatformStream, Stream].slice(PlatformStream.Readable ? 0 : 1)
  .forEach(function(Stream) {

describe('Duplexer', function() {

  describe('in binary mode', function() {

    describe('and with async streams', function() {

      it('should work with functionnal API', function(done) {
        var readable = new Stream.PassThrough()
          , writable = new Stream.PassThrough()
          , duplex = Duplexer({}, writable, readable)
        ;

        // Checking writable content
        writable.pipe(es.wait(function(err, data) {
          assert.equal(data,'oudelali');
        }));

        // Checking duplex output
        duplex.pipe(es.wait(function(err, data) {
          assert.equal(data,'bibabeloola');
          done();
        }));

        setImmediate(function() {
          // Writing content to duplex
          duplex.write('oude');
          duplex.write('lali');
          duplex.end();

          // Writing content to readable
          readable.write('biba');
          readable.write('beloola');
          readable.end();
        });

      });

      it('should work with POO API', function(done) {
        var readable = new Stream.PassThrough()
          , writable = new Stream.PassThrough()
          , duplex = new Duplexer(writable, readable)
        ;

        // Checking writable content
        writable.pipe(es.wait(function(err, data) {
          assert.equal(data,'oudelali');
        }));

        // Checking duplex output
        duplex.pipe(es.wait(function(err, data) {
          assert.equal(data,'bibabeloola');
          done();
        }));

        setImmediate(function() {
          // Writing content to duplex
          duplex.write('oude');
          duplex.write('lali');
          duplex.end();

          // Writing content to readable
          readable.write('biba');
          readable.write('beloola');
          readable.end();
        });

      });

      it('should reemit errors', function(done) {
        var readable = new Stream.PassThrough()
          , writable = new Stream.PassThrough()
          , duplex = new Duplexer(writable, readable)
          , errorsCount = 0
        ;

        // Checking writable content
        writable.pipe(es.wait(function(err, data) {
          assert.equal(data,'oudelali');
        }));

        // Checking duplex output
        duplex.pipe(es.wait(function(err, data) {
          assert.equal(data,'bibabeloola');
          assert.equal(errorsCount, 2);
          done();
        }));

        duplex.on('error', function() {
          errorsCount++;
        });

        setImmediate(function() {
          // Writing content to duplex
          duplex.write('oude');
          writable.emit('error', new Error('hip'));
          duplex.write('lali');
          duplex.end();

          // Writing content to readable
          readable.write('biba');
          readable.emit('error', new Error('hip'));
          readable.write('beloola');
          readable.end();
        });

      });

      it('should not reemit errors when option is set', function(done) {
        var readable = new Stream.PassThrough()
          , writable = new Stream.PassThrough()
          , duplex = new Duplexer({reemitErrors: false}, writable, readable)
          , errorsCount = 0
        ;

        // Checking writable content
        writable.pipe(es.wait(function(err, data) {
          assert.equal(data,'oudelali');
        }));

        // Checking duplex output
        duplex.pipe(es.wait(function(err, data) {
          assert.equal(data,'bibabeloola');
          assert.equal(errorsCount, 0);
          done();
        }));

        duplex.on('error', function() {
          errorsCount++;
        });

        // Catch error events
        readable.on('error', function(){})
        writable.on('error', function(){})

        setImmediate(function() {
          // Writing content to duplex
          duplex.write('oude');
          writable.emit('error', new Error('hip'));
          duplex.write('lali');
          duplex.end();

          // Writing content to readable
          readable.write('biba');
          readable.emit('error', new Error('hip'));
          readable.write('beloola');
          readable.end();
        });

      });

    });

    describe('and with sync streams', function() {

      it('should work with functionnal API', function(done) {
        var readable = new Stream.PassThrough()
          , writable = new Stream.PassThrough()
          , duplex = Duplexer({}, writable, readable)
        ;

        // Checking writable content
        writable.pipe(es.wait(function(err, data) {
          assert.equal(data,'oudelali');
        }));

        // Checking duplex output
        duplex.pipe(es.wait(function(err, data) {
          assert.equal(data,'bibabeloola');
          done();
        }));

        // Writing content to duplex
        duplex.write('oude');
        duplex.write('lali');
        duplex.end();

        // Writing content to readable
        readable.write('biba');
        readable.write('beloola');
        readable.end();

      });

      it('should work with POO API', function(done) {
        var readable = new Stream.PassThrough()
          , writable = new Stream.PassThrough()
          , duplex = new Duplexer(writable, readable)
        ;

        // Checking writable content
        writable.pipe(es.wait(function(err, data) {
          assert.equal(data,'oudelali');
        }));

        // Checking duplex output
        duplex.pipe(es.wait(function(err, data) {
          assert.equal(data,'bibabeloola');
          done();
        }));

        // Writing content to duplex
        duplex.write('oude');
        duplex.write('lali');
        duplex.end();


        // Writing content to readable
        readable.write('biba');
        readable.write('beloola');
        readable.end();

      });

      it('should reemit errors', function(done) {
        var readable = new Stream.PassThrough()
          , writable = new Stream.PassThrough()
          , duplex = new Duplexer(null, writable, readable)
          , errorsCount = 0
        ;

        // Checking writable content
        writable.pipe(es.wait(function(err, data) {
          assert.equal(data,'oudelali');
        }));

        // Checking duplex output
        duplex.pipe(es.wait(function(err, data) {
          assert.equal(data,'bibabeloola');
          assert.equal(errorsCount, 2);
          done();
        }));

        duplex.on('error', function() {
          errorsCount++;
        });

        // Writing content to duplex
        duplex.write('oude');
        writable.emit('error', new Error('hip'));
        duplex.write('lali');
        duplex.end();

        // Writing content to readable
        readable.write('biba');
        readable.emit('error', new Error('hip'));
        readable.write('beloola');
        readable.end();

      });

      it('should not reemit errors when option is set', function(done) {
        var readable = new Stream.PassThrough()
          , writable = new Stream.PassThrough()
          , duplex = new Duplexer({reemitErrors: false}, writable, readable)
          , errorsCount = 0
        ;

        // Checking writable content
        writable.pipe(es.wait(function(err, data) {
          assert.equal(data,'oudelali');
        }));

        // Checking duplex output
        duplex.pipe(es.wait(function(err, data) {
          assert.equal(data,'bibabeloola');
          assert.equal(errorsCount, 0);
          done();
        }));

        duplex.on('error', function() {
          errorsCount++;
        });

        // Catch error events
        readable.on('error', function(){})
        writable.on('error', function(){})

        // Writing content to duplex
        duplex.write('oude');
        writable.emit('error', new Error('hip'));
        duplex.write('lali');
        duplex.end();

        // Writing content to readable
        readable.write('biba');
        readable.emit('error', new Error('hip'));
        readable.write('beloola');
        readable.end();

      });

    });

  });

  describe('in object mode', function() {

    describe('and with async streams', function() {

      it('should work with functionnal API', function(done) {
        var readable = new Stream.PassThrough({objectMode: true})
          , writable = new Stream.PassThrough({objectMode: true})
          , duplex = Duplexer({objectMode: true}, writable, readable)
          , wrtCount = 0
          , dplCount = 0
        ;

        // Checking writable content
        writable.pipe(es.map(function(data, cb) {
          if(1 == ++wrtCount) {
            assert.equal(data.cnt, 'oude');
          } else {
            assert.equal(data.cnt, 'lali');
          }
          cb();
        }));

        // Checking duplex output
        duplex.pipe(es.map(function(data, cb) {
          if(1 == ++dplCount) {
            assert.equal(data.cnt, 'biba');
          } else {
            assert.equal(data.cnt, 'beloola');
          }
          cb();
        })).pipe(es.wait(function(data, cb) {
          assert.equal(wrtCount, 2);
          assert.equal(dplCount, 2);
          done();
        }));

        setImmediate(function() {
          // Writing content to duplex
          duplex.write({cnt: 'oude'});
          duplex.write({cnt: 'lali'});
          duplex.end();

          // Writing content to readable
          readable.write({cnt: 'biba'});
          readable.write({cnt: 'beloola'});
          readable.end();
        });

      });

      it('should work with POO API', function(done) {
        var readable = new Stream.PassThrough({objectMode: true})
          , writable = new Stream.PassThrough({objectMode: true})
          , duplex = new Duplexer({objectMode: true}, writable, readable)
          , wrtCount = 0
          , dplCount = 0
        ;

        // Checking writable content
        writable.pipe(es.map(function(data, cb) {
          if(1 == ++wrtCount) {
            assert.equal(data.cnt, 'oude');
          } else {
            assert.equal(data.cnt, 'lali');
          }
          cb();
        }));

        // Checking duplex output
        duplex.pipe(es.map(function(data, cb) {
          if(1 == ++dplCount) {
            assert.equal(data.cnt, 'biba');
          } else {
            assert.equal(data.cnt, 'beloola');
          }
          cb();
        })).pipe(es.wait(function(data, cb) {
          assert.equal(wrtCount, 2);
          assert.equal(dplCount, 2);
          done();
        }));

        setImmediate(function() {
          // Writing content to duplex
          duplex.write({cnt: 'oude'});
          duplex.write({cnt: 'lali'});
          duplex.end();

          // Writing content to readable
          readable.write({cnt: 'biba'});
          readable.write({cnt: 'beloola'});
          readable.end();
        });

      });

      it('should reemit errors', function(done) {
        var readable = new Stream.PassThrough({objectMode: true})
          , writable = new Stream.PassThrough({objectMode: true})
          , duplex = new Duplexer({objectMode: true}, writable, readable)
          , errorsCount = 0
          , wrtCount = 0
          , dplCount = 0
        ;

        // Checking writable content
        writable.pipe(es.map(function(data, cb) {
          if(1 == ++wrtCount) {
            assert.equal(data.cnt, 'oude');
          } else {
            assert.equal(data.cnt, 'lali');
          }
          cb();
        }));

        // Checking duplex output
        duplex.pipe(es.map(function(data, cb) {
          if(1 == ++dplCount) {
            assert.equal(data.cnt, 'biba');
          } else {
            assert.equal(data.cnt, 'beloola');
          }
          cb();
        })).pipe(es.wait(function(data, cb) {
          assert.equal(wrtCount, 2);
          assert.equal(dplCount, 2);
          assert.equal(errorsCount, 2);
          done();
        }));

        duplex.on('error', function() {
          errorsCount++;
        });

        setImmediate(function() {
          // Writing content to duplex
          duplex.write({cnt: 'oude'});
          writable.emit('error', new Error('hip'));
          duplex.write({cnt: 'lali'});
          duplex.end();

          // Writing content to readable
          readable.write({cnt: 'biba'});
          readable.emit('error', new Error('hip'));
          readable.write({cnt: 'beloola'});
          readable.end();
        });

      });

      it('should not reemit errors when option is set', function(done) {
        var readable = new Stream.PassThrough({objectMode: true})
          , writable = new Stream.PassThrough({objectMode: true})
          , duplex = new Duplexer({objectMode: true, reemitErrors: false}, writable, readable)
          , errorsCount = 0
          , wrtCount = 0
          , dplCount = 0
        ;

        // Checking writable content
        writable.pipe(es.map(function(data, cb) {
          if(1 == ++wrtCount) {
            assert.equal(data.cnt, 'oude');
          } else {
            assert.equal(data.cnt, 'lali');
          }
          cb();
        }));

        // Checking duplex output
        duplex.pipe(es.map(function(data, cb) {
          if(1 == ++dplCount) {
            assert.equal(data.cnt, 'biba');
          } else {
            assert.equal(data.cnt, 'beloola');
          }
          cb();
        })).pipe(es.wait(function(data, cb) {
          assert.equal(wrtCount, 2);
          assert.equal(dplCount, 2);
          assert.equal(errorsCount, 0);
          done();
        }));

        duplex.on('error', function() {
          errorsCount++;
        });

        // Catch error events
        readable.on('error', function(){})
        writable.on('error', function(){})

        setImmediate(function() {
          // Writing content to duplex
          duplex.write({cnt: 'oude'});
          writable.emit('error', new Error('hip'));
          duplex.write({cnt: 'lali'});
          duplex.end();

          // Writing content to readable
          readable.write({cnt: 'biba'});
          readable.emit('error', new Error('hip'));
          readable.write({cnt: 'beloola'});
          readable.end();
        });

      });

    });

    describe('and with sync streams', function() {

      it('should work with functionnal API', function(done) {
        var readable = new Stream.PassThrough({objectMode: true})
          , writable = new Stream.PassThrough({objectMode: true})
          , duplex = Duplexer({objectMode: true}, writable, readable)
          , wrtCount = 0
          , dplCount = 0
        ;

        // Checking writable content
        writable.pipe(es.map(function(data, cb) {
          if(1 == ++wrtCount) {
            assert.equal(data.cnt, 'oude');
          } else {
            assert.equal(data.cnt, 'lali');
          }
          cb();
        }));

        // Checking duplex output
        duplex.pipe(es.map(function(data, cb) {
          if(1 == ++dplCount) {
            assert.equal(data.cnt, 'biba');
          } else {
            assert.equal(data.cnt, 'beloola');
          }
          cb();
        })).pipe(es.wait(function(data, cb) {
          assert.equal(wrtCount, 2);
          assert.equal(dplCount, 2);
          done();
        }));

        // Writing content to duplex
        duplex.write({cnt: 'oude'});
        duplex.write({cnt: 'lali'});
        duplex.end();

        // Writing content to readable
        readable.write({cnt: 'biba'});
        readable.write({cnt: 'beloola'});
        readable.end();

      });

      it('should work with POO API', function(done) {
        var readable = new Stream.PassThrough({objectMode: true})
          , writable = new Stream.PassThrough({objectMode: true})
          , duplex = new Duplexer({objectMode: true}, writable, readable)
          , wrtCount = 0
          , dplCount = 0
        ;

        // Checking writable content
        writable.pipe(es.map(function(data, cb) {
          if(1 == ++wrtCount) {
            assert.equal(data.cnt, 'oude');
          } else {
            assert.equal(data.cnt, 'lali');
          }
          cb();
        }));

        // Checking duplex output
        duplex.pipe(es.map(function(data, cb) {
          if(1 == ++dplCount) {
            assert.equal(data.cnt, 'biba');
          } else {
            assert.equal(data.cnt, 'beloola');
          }
          cb();
        })).pipe(es.wait(function(data, cb) {
          assert.equal(wrtCount, 2);
          assert.equal(dplCount, 2);
          done();
        }));

        // Writing content to duplex
        duplex.write({cnt: 'oude'});
        duplex.write({cnt: 'lali'});
        duplex.end();

        // Writing content to readable
        readable.write({cnt: 'biba'});
        readable.write({cnt: 'beloola'});
        readable.end();

      });

      it('should reemit errors', function(done) {
        var readable = new Stream.PassThrough({objectMode: true})
          , writable = new Stream.PassThrough({objectMode: true})
          , duplex = new Duplexer({objectMode: true}, writable, readable)
          , errorsCount = 0
          , wrtCount = 0
          , dplCount = 0
        ;

        // Checking writable content
        writable.pipe(es.map(function(data, cb) {
          if(1 == ++wrtCount) {
            assert.equal(data.cnt, 'oude');
          } else {
            assert.equal(data.cnt, 'lali');
          }
          cb();
        }));

        // Checking duplex output
        duplex.pipe(es.map(function(data, cb) {
          if(1 == ++dplCount) {
            assert.equal(data.cnt, 'biba');
          } else {
            assert.equal(data.cnt, 'beloola');
          }
          cb();
        })).pipe(es.wait(function(data, cb) {
          assert.equal(wrtCount, 2);
          assert.equal(dplCount, 2);
          assert.equal(errorsCount, 2);
          done();
        }));

        duplex.on('error', function() {
          errorsCount++;
        });

        // Writing content to duplex
        duplex.write({cnt: 'oude'});
        writable.emit('error', new Error('hip'));
        duplex.write({cnt: 'lali'});
        duplex.end();

        // Writing content to readable
        readable.write({cnt: 'biba'});
        readable.emit('error', new Error('hip'));
        readable.write({cnt: 'beloola'});
        readable.end();

      });

      it('should not reemit errors when option is set', function(done) {
        var readable = new Stream.PassThrough({objectMode: true})
          , writable = new Stream.PassThrough({objectMode: true})
          , duplex = new Duplexer({objectMode: true, reemitErrors: false}, writable, readable)
          , errorsCount = 0
          , wrtCount = 0
          , dplCount = 0
        ;

        // Checking writable content
        writable.pipe(es.map(function(data, cb) {
          if(1 == ++wrtCount) {
            assert.equal(data.cnt, 'oude');
          } else {
            assert.equal(data.cnt, 'lali');
          }
          cb();
        }));

        // Checking duplex output
        duplex.pipe(es.map(function(data, cb) {
          if(1 == ++dplCount) {
            assert.equal(data.cnt, 'biba');
          } else {
            assert.equal(data.cnt, 'beloola');
          }
          cb();
        })).pipe(es.wait(function(data, cb) {
          assert.equal(wrtCount, 2);
          assert.equal(dplCount, 2);
          assert.equal(errorsCount, 0);
          done();
        }));

        duplex.on('error', function() {
          errorsCount++;
        });

        // Catch error events
        readable.on('error', function(){})
        writable.on('error', function(){})

        // Writing content to duplex
        duplex.write({cnt: 'oude'});
        writable.emit('error', new Error('hip'));
        duplex.write({cnt: 'lali'});
        duplex.end();

        // Writing content to readable
        readable.write({cnt: 'biba'});
        readable.emit('error', new Error('hip'));
        readable.write({cnt: 'beloola'});
        readable.end();

      });

    });

  });

});

});

var path = require('path');
var Stream = require('readable-stream');
var gutil = require('gulp-util');
var BufferStreams = require('bufferstreams');
var ttf2eot = require('ttf2eot');

const PLUGIN_NAME = 'gulp-ttf2eot';

// File level transform function
function ttf2eotTransform(opt) {
  // Return a callback function handling the buffered content
  return function(err, buf, cb) {

    // Handle any error
    if(err) {
      cb(new gutil.PluginError(PLUGIN_NAME, err, {showStack: true}));
    }

    // Use the buffered content
      try {
        buf = new Buffer(ttf2eot(new Uint8Array(buf)).buffer);
        cb(null, buf);
      } catch(err) {
        cb(new gutil.PluginError(PLUGIN_NAME, err, {showStack: true}));
      }

  };
}

// Plugin function
function ttf2eotGulp(options) {

  options = options || {};
  options.ignoreExt = options.ignoreExt || false;
  options.clone = options.clone || false;

  var stream = new Stream.Transform({objectMode: true});

  stream._transform = function(file, unused, done) {
     // When null just pass through
    if(file.isNull()) {
      stream.push(file); done();
      return;
    }

    // If the ext doesn't match, pass it through
    if((!options.ignoreExt) && '.ttf' !== path.extname(file.path)) {
      stream.push(file); done();
      return;
    }

    // Fix for the vinyl clone method...
    // https://github.com/wearefractal/vinyl/pull/9
    if(options.clone) {
      if(file.isBuffer()) {
        stream.push(file.clone());
      } else {
        var cntStream = file.contents;
        file.contents = null;
        var newFile = file.clone();
        file.contents = cntStream.pipe(new Stream.PassThrough());
        newFile.contents = cntStream.pipe(new Stream.PassThrough());
        stream.push(newFile);
      }
    }
  
    file.path = gutil.replaceExtension(file.path, ".eot");

    // Buffers
    if(file.isBuffer()) {
      try {
        file.contents = new Buffer(ttf2eot(
          new Uint8Array(file.contents)
        ).buffer);
      } catch(err) {
        stream.emit('error', new gutil.PluginError(PLUGIN_NAME, err, {
          showStack: true
        }));
      }

    // Streams
    } else {
      file.contents = file.contents.pipe(new BufferStreams(ttf2eotTransform()));
    }
    
    stream.push(file);
    done();
  };

  return stream;

};

// Export the file level transform function for other plugins usage
ttf2eotGulp.fileTransform = ttf2eotTransform;

// Export the plugin main function
module.exports = ttf2eotGulp;


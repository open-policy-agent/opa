// Need to keep a ref to platform stream constructors since readable-stream
// doens't inherit of them'
// See: https://github.com/isaacs/readable-stream/pull/87
var PlatformStream = require('stream')
  , Stream = require('readable-stream')
  , util = require('util')
;

// Helper to test instances
function streamInstanceOf(stream) {
  var args = [].slice(arguments, 1)
    , curConstructor;
  if(!(stream instanceof Stream || stream instanceof PlatformStream)) {
    return false;
  }
  while(args.length) {
    curConstructor = arg.pop();
    if(!(stream instanceof Stream[curConstructor]
      || 'undefined' === PlatformStream[curConstructor]
      || stream instanceof PlatformStream[curConstructor]
    )) {
      return false;
    }
  }
  return true; // Defaults to true since checking isn't possible with 0.8
}

// Inherit of Duplex stream
util.inherits(Duplexer, Stream.Duplex);

// Constructor
function Duplexer(options, writableStream, readableStream) {
  var _self = this;

  // Ensure new were used
  if (!(this instanceof Duplexer)) {
    return new (Duplexer.bind.apply(Duplexer,
      [Duplexer].concat([].slice.call(arguments,0))));
  }

  // Mapping args
  if(streamInstanceOf(options)) {
    readableStream = writableStream;
    writableStream = options;
    options = {};
  } else {
    options = options || {};
  }
  this._reemitErrors = 'boolean' === typeof options.reemitErrors
    ? options.reemitErrors : true;
  delete options.reemitErrors;

  // Checking arguments
  if(!streamInstanceOf(writableStream, 'Writable', 'Duplex')) {
    throw new Error('The writable stream must be an instanceof Writable or Duplex.');
  }
  if(!streamInstanceOf(readableStream, 'Readable')) {
    throw new Error('The readable stream must be an instanceof Readable.');
  }

  // Parent constructor
  Stream.Duplex.call(this, options);

  // Save streams refs
  this._writable = writableStream;
  this._readable = readableStream;

  // Internal state
  this._waitDatas = false;
  this._hasDatas = false;

  if('undefined' == typeof this._readable._readableState) {
    this._readable = (new Stream.Readable({
      objectMode: options.objectMode || false
    })).wrap(this._readable);
  }

  if(this._reemitErrors) {
    this._writable.on('error', function(err) {
      _self.emit('error', err);
    });
    this._readable.on('error', function(err) {
      _self.emit('error', err);
    });
  }

  this._writable.on("drain", function() {
    _self.emit("drain");
  });

  this.once('finish', function() {
    _self._writable.end();
  });

  this._writable.once('finish', function() {
    _self.end();
  });

  this._readable.on('readable', function() {
    _self._hasDatas = true;
    if(_self._waitDatas) {
      _self._pushAll();
    }
  });

  this._readable.once('end', function() {
    _self.push(null);
  });
}

Duplexer.prototype._read = function(n) {
  this._waitDatas = true;
  if(this._hasDatas) {
    this._pushAll();
  }
};

Duplexer.prototype._pushAll = function() {
  var _self = this, chunk;
  do {
    chunk = _self._readable.read();
    if(null !== chunk) {
      this._waitDatas = _self.push(chunk);
    }
    this._hasDatas = (null !== chunk);
  } while(this._waitDatas && this._hasDatas);
};

Duplexer.prototype._write = function(chunk, encoding, callback) {
  return this._writable.write(chunk, encoding, callback);
};

module.exports = Duplexer;


//
// Light version of byte buffer
//

'use strict';

// wraps and reuses buffer, possibly cropped (offset, length)
var ByteBuffer = function (buffer, start, length) {
  /*jshint bitwise:false*/

  var isNested = buffer instanceof ByteBuffer;

  this.buffer = isNested ? buffer.buffer : buffer;
  this.start = (start || 0) + (isNested ? buffer.start : 0);
  this.length = length || (this.buffer.length - this.start);
  this.offset = 0;

  this.isTyped = this.buffer instanceof Uint8Array;

  this.getUint8 = function (pos) {
    return this.buffer[pos + this.start];
  };

  this.getUint16 = function (pos, littleEndian) {
    var val;
    if (littleEndian) {
      throw new Error('not implemented');
    } else {
      val = this.buffer[pos + 1 + this.start];
      val = val + (this.buffer[pos + this.start] << 8 >>> 0);
    }
    return val;
  };

  this.getUint32 = function (pos, littleEndian) {
    var val;
    if (littleEndian) {
      throw new Error('not implemented');
    } else {
      val = this.buffer[pos + 1 + this.start] << 16;
      val |= this.buffer[pos + 2 + this.start] << 8;
      val |= this.buffer[pos + 3 + this.start];
      val = val + (this.buffer[pos + this.start] << 24 >>> 0);
    }
    return val;
  };

  this.setUint8 = function (pos, value) {
    this.buffer[pos + this.start] = value & 0xFF;
  };

  this.setUint16 = function (pos, value, littleEndian) {
    var offset = pos + this.start;
    var buf = this.buffer;
    if (littleEndian) {
      buf[offset] = value & 0xFF;
      buf[offset + 1] = (value >>> 8) & 0xFF;
    } else {
      buf[offset] = (value >>> 8) & 0xFF;
      buf[offset + 1] = value & 0xFF;
    }
  };

  this.setUint32 = function (pos, value, littleEndian) {
    var offset = pos + this.start;
    var buf = this.buffer;
    if (littleEndian) {
      buf[offset] = value & 0xFF;
      buf[offset + 1] = (value >>> 8) & 0xFF;
      buf[offset + 2] = (value >>> 16) & 0xFF;
      buf[offset + 3] = (value >>> 24) & 0xFF;
    } else {
      buf[offset] = (value >>> 24) & 0xFF;
      buf[offset + 1] = (value >>> 16) & 0xFF;
      buf[offset + 2] = (value >>> 8) & 0xFF;
      buf[offset + 3] = value & 0xFF;
    }
  };

  this.writeUint8 = function (value) {
    this.buffer[this.offset + this.start] = value & 0xFF;
    this.offset++;
  };

  this.writeInt8 = function (value) {
    this.setUint8(this.offset, (value < 0) ? 0xFF + value + 1 : value);
    this.offset++;
  };

  this.writeUint16 = function (value, littleEndian) {
    this.setUint16(this.offset, value, littleEndian);
    this.offset += 2;
  };

  this.writeInt16 = function (value, littleEndian) {
    this.setUint16(this.offset, (value < 0) ? 0xFFFF + value + 1 : value, littleEndian);
    this.offset += 2;
  };

  this.writeUint32 = function (value, littleEndian) {
    this.setUint32(this.offset, value, littleEndian);
    this.offset += 4;
  };

  this.writeInt32 = function (value, littleEndian) {
    this.setUint32(this.offset, (value < 0) ? 0xFFFFFFFF + value + 1 : value, littleEndian);
    this.offset += 4;
  };

};


// get current position
//
ByteBuffer.prototype.tell = function() {
  return this.offset;
};

// set current position
//
ByteBuffer.prototype.seek = function (pos) {
  this.offset = pos;
};

ByteBuffer.prototype.fill = function (value) {
  var index = this.length - 1;
  while (index >= 0) {
    this.buffer[index + this.start] = value;
    index--;
  }
};

ByteBuffer.prototype.create = function (size) {
  var buf = Uint8Array ? new Uint8Array(size) : new Array(size);
  return new ByteBuffer(buf);
};

ByteBuffer.prototype.writeUint64 = function (value) {
  // we canot use bitwise operations for 64bit values because of JavaScript limitations,
  // instead we should divide it to 2 Int32 numbers
  // 2^32 = 4294967296
  var hi = Math.floor(value / 4294967296);
  var lo = value - hi * 4294967296;
  this.writeUint32(hi);
  this.writeUint32(lo);
};

ByteBuffer.prototype.writeBytes = function (data) {
  var buffer = this.buffer;
  var offset = this.offset + this.start;
  if (this.isTyped) {
    //console.log('fast  write');
    buffer.set(data, offset);
  } else {
    for (var i = 0; i < data.length; i++) {
      buffer[i + offset] = data[i];
    }
  }
  this.offset += data.length;
};

ByteBuffer.prototype.toString = function (offset, length) {
  // default values if not set
  offset = (offset || 0);
  length = length || (this.length - offset);

  // add buffer shift
  var start = offset + this.start;
  var end = start + length;

  var string = '';
  for (var i = start; i < end; i++) {
    string += String.fromCharCode(this.buffer[i]);
  }
  return string;
};

ByteBuffer.prototype.toArray = function () {
  if (this.isTyped) {
    return this.buffer.subarray(this.start, this.start + this.length);
  } else {
    return this.buffer.slice(this.start, this.start + this.length);
  }
};



module.exports = ByteBuffer;

// Light implementation of binary buffer with helpers for easy access.
//
'use strict';


var TYPED_OK =  (typeof Uint8Array !== 'undefined');

function createArray(size) {
  return TYPED_OK ? new Uint8Array(size) : Array(size);
}


function MicroBuffer(buffer, start, length) {

  var isInherited = buffer instanceof MicroBuffer;

  this.buffer = isInherited ?
    buffer.buffer
  :
    (typeof buffer === 'number' ? createArray(buffer) : buffer);

  this.start = (start || 0) + (isInherited ? buffer.start : 0);
  this.length = length || (this.buffer.length - this.start);
  this.offset = 0;

  this.isTyped = !Array.isArray(this.buffer);
}


MicroBuffer.prototype.getUint8 = function (pos) {
  return this.buffer[pos + this.start];
};


MicroBuffer.prototype.getUint16 = function (pos, littleEndian) {
  var val;
  if (littleEndian) {
    throw new Error('not implemented');
  } else {
    val = this.buffer[pos + 1 + this.start];
    val += this.buffer[pos + this.start] << 8 >>> 0;
  }
  return val;
};


MicroBuffer.prototype.getUint32 = function (pos, littleEndian) {
  var val;
  if (littleEndian) {
    throw new Error('not implemented');
  } else {
    val = this.buffer[pos + 1 + this.start] << 16;
    val |= this.buffer[pos + 2 + this.start] << 8;
    val |= this.buffer[pos + 3 + this.start];
    val += this.buffer[pos + this.start] << 24 >>> 0;
  }
  return val;
};


MicroBuffer.prototype.setUint8 = function (pos, value) {
  this.buffer[pos + this.start] = value & 0xFF;
};


MicroBuffer.prototype.setUint16 = function (pos, value, littleEndian) {
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


MicroBuffer.prototype.setUint32 = function (pos, value, littleEndian) {
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


MicroBuffer.prototype.writeUint8 = function (value) {
  this.buffer[this.offset + this.start] = value & 0xFF;
  this.offset++;
};


MicroBuffer.prototype.writeInt8 = function (value) {
  this.setUint8(this.offset, (value < 0) ? 0xFF + value + 1 : value);
  this.offset++;
};


MicroBuffer.prototype.writeUint16 = function (value, littleEndian) {
  this.setUint16(this.offset, value, littleEndian);
  this.offset += 2;
};


MicroBuffer.prototype.writeInt16 = function (value, littleEndian) {
  this.setUint16(this.offset, (value < 0) ? 0xFFFF + value + 1 : value, littleEndian);
  this.offset += 2;
};


MicroBuffer.prototype.writeUint32 = function (value, littleEndian) {
  this.setUint32(this.offset, value, littleEndian);
  this.offset += 4;
};


MicroBuffer.prototype.writeInt32 = function (value, littleEndian) {
  this.setUint32(this.offset, (value < 0) ? 0xFFFFFFFF + value + 1 : value, littleEndian);
  this.offset += 4;
};


// get current position
//
MicroBuffer.prototype.tell = function () {
  return this.offset;
};


// set current position
//
MicroBuffer.prototype.seek = function (pos) {
  this.offset = pos;
};


MicroBuffer.prototype.fill = function (value) {
  var index = this.length - 1;
  while (index >= 0) {
    this.buffer[index + this.start] = value;
    index--;
  }
};


MicroBuffer.prototype.writeUint64 = function (value) {
  // we canot use bitwise operations for 64bit values because of JavaScript limitations,
  // instead we should divide it to 2 Int32 numbers
  // 2^32 = 4294967296
  var hi = Math.floor(value / 4294967296);
  var lo = value - hi * 4294967296;
  this.writeUint32(hi);
  this.writeUint32(lo);
};


MicroBuffer.prototype.writeBytes = function (data) {
  var buffer = this.buffer;
  var offset = this.offset + this.start;
  if (this.isTyped) {
    buffer.set(data, offset);
  } else {
    for (var i = 0; i < data.length; i++) {
      buffer[i + offset] = data[i];
    }
  }
  this.offset += data.length;
};


MicroBuffer.prototype.toString = function (offset, length) {
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


MicroBuffer.prototype.toArray = function () {
  if (this.isTyped) {
    return this.buffer.subarray(this.start, this.start + this.length);
  }

  return this.buffer.slice(this.start, this.start + this.length);
};


module.exports = MicroBuffer;

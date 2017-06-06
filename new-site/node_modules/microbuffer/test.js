'use strict';

/*global describe, it*/

var assert      = require('assert');
var _           = require('lodash');
var MicroBuffer = require('./');


var mb;


function cmpBuf(a, b) {
  if (a.length !== b.length) {
    throw new assert.AssertionError({
      actual: a,
      expected: b,
      operator: 'compare'
    });
  }

  for (var i = 0; i < a.length; i++) {
    if (a[i] !== b[i]) {
      throw new assert.AssertionError({
        actual: a,
        expected: b,
        operator: 'compare'
      });
    }
  }
}


describe('MicroBuffer', function () {

  it('create by size', function () {
    mb = new MicroBuffer(5);

    assert.equal(mb.length, 5);
    assert.ok(_.isTypedArray(mb.buffer));
  });


  it('wrap array', function () {
    mb = new MicroBuffer([ 1, 2, 3, 4 ]);
    cmpBuf(mb.toArray(), [ 1, 2, 3, 4 ]);

    mb = new MicroBuffer([ 1, 2, 3, 4 ], 1, 2);
    cmpBuf(mb.toArray(), [ 2, 3 ]);
  });


  it('wrap typed array', function () {
    mb = new MicroBuffer(new Uint8Array([ 1, 2, 3, 4 ]));
    cmpBuf(mb.toArray(), [ 1, 2, 3, 4 ]);

    mb = new MicroBuffer(new Uint8Array([ 1, 2, 3, 4 ]), 1, 2);
    cmpBuf(mb.toArray(), [ 2, 3 ]);
  });


  it('wrap MicroBuffer', function () {
    mb = new MicroBuffer(new MicroBuffer([ 1, 2, 3, 4 ]));
    cmpBuf(mb.toArray(), [ 1, 2, 3, 4 ]);

    mb = new MicroBuffer(new MicroBuffer([ 1, 2, 3, 4 ]), 1, 2);
    cmpBuf(mb.toArray(), [ 2, 3 ]);
  });


  it('get/set numbers', function () {
    mb = new MicroBuffer(4);
    mb.setUint8(0, 0xAA);
    mb.setUint8(1, 0x55);
    mb.setUint16(2, 0x88EE);

    assert.equal(mb.getUint8(0), 0xAA);
    assert.equal(mb.getUint8(1), 0x55);
    assert.equal(mb.getUint8(2), 0x88);
    assert.equal(mb.getUint8(3), 0xEE);

    assert.equal(mb.getUint16(0), 0xAA55);
    assert.equal(mb.getUint16(2), 0x88EE);

    assert.equal(mb.getUint32(0), 0xAA5588EE);

    mb = new MicroBuffer(4);
    mb.setUint32(0, 0xAA5588EE);

    assert.equal(mb.getUint32(0), 0xAA5588EE);
  });


  it('get/set numbers LE', function () {
    mb = new MicroBuffer(4);
    mb.setUint16(0, 0x88EE, true);

    assert.equal(mb.getUint16(0), 0xEE88);

    mb = new MicroBuffer(4);
    mb.setUint32(0, 0xAA5588EE, true);

    assert.equal(mb.getUint32(0), 0xEE8855AA);
  });


  it('write numbers', function () {
    mb = new MicroBuffer(14);

    mb.writeUint8(1);
    assert.equal(mb.tell(), 1);

    mb.writeInt8(-1);
    assert.equal(mb.tell(), 2);

    mb.writeUint16(0xAA55);
    assert.equal(mb.tell(), 4);

    mb.writeInt16(-2);
    assert.equal(mb.tell(), 6);

    mb.writeUint32(0xEE33AA55);
    assert.equal(mb.tell(), 10);

    mb.writeInt32(-3);
    assert.equal(mb.tell(), 14);

    cmpBuf(mb.toArray(), [
      1,
      0xFF,
      0xAA, 0x55,
      0xFF, 0xFE,
      0xEE, 0x33, 0xAA, 0x55,
      0xFF, 0xFF, 0xFF, 0xFD
    ]);

  });


  it('write numbers LE', function () {
    mb = new MicroBuffer(4);

    mb.writeUint16(0xAA55, true);
    mb.writeInt16(-2, true);

    cmpBuf(mb.toArray(), [
      0x55, 0xAA,
      0xFE, 0xFF
    ]);

    mb = new MicroBuffer(8);

    mb.writeUint32(0xEE33AA55, true);
    mb.writeInt32(-3, true);

    cmpBuf(mb.toArray(), [
      0x55, 0xAA, 0x33, 0xEE,
      0xFD, 0xFF, 0xFF, 0xFF
    ]);
  });


  it('write Uint64', function () {
    mb = new MicroBuffer(8);

    mb.writeUint64(0x112233445566);
    cmpBuf(mb.toArray(), [
      0x00, 0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66
    ]);
  });


  it('seek/fill', function () {
    mb = new MicroBuffer(4);

    mb.fill(0x99);
    mb.seek(2);
    mb.writeUint16(0xAA55);

    assert.equal(mb.getUint32(0), 0x9999AA55);
  });


  it('writeBytes', function () {
    mb = new MicroBuffer(4);

    mb.writeBytes([ 0x00, 0xFF ]);
    mb.writeBytes(new Uint8Array([ 0xAA, 0x55 ]));

    assert.equal(mb.getUint32(0), 0x00FFAA55);
  });


  it('toString', function () {
    mb = new MicroBuffer([ 0xAA, 0x55, 0x00, 0xFF ]);

    var str = mb.toString();

    assert.equal(str.charCodeAt(0), 0xAA);
    assert.equal(str.charCodeAt(1), 0x55);
    assert.equal(str.charCodeAt(2), 0x00);
    assert.equal(str.charCodeAt(3), 0xFF);
  });

});

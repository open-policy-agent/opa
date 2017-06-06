'use strict';

// See documentation here: http://www.microsoft.com/typography/otspec/maxp.htm

var _ = require('lodash');
var ByteBuffer = require('../../byte_buffer.js');

// Find max points in glyph TTF contours.
function getMaxPoints(font) {
  return _.max(_.map(font.glyphs, function(glyph) {
    return _.reduce(glyph.ttfContours, function (sum, ctr) { return sum + ctr.length; }, 0);
  }));
}

function getMaxContours(font) {
  return _.max(_.map(font.glyphs, function (glyph) {
    return glyph.ttfContours.length;
  }));
}

function createMaxpTable(font) {

  var buf = ByteBuffer.prototype.create(32);

  buf.writeInt32(0x10000); // version
  buf.writeUint16(font.glyphs.length); // numGlyphs
  buf.writeUint16(getMaxPoints(font)); // maxPoints
  buf.writeUint16(getMaxContours(font)); // maxContours
  buf.writeUint16(0); // maxCompositePoints
  buf.writeUint16(0); // maxCompositeContours
  buf.writeUint16(2); // maxZones
  buf.writeUint16(0); // maxTwilightPoints
  // It is unclear how to calculate maxStorage, maxFunctionDefs and maxInstructionDefs.
  // These are magic constants now, with values exceeding values from FontForge
  buf.writeUint16(10); // maxStorage
  buf.writeUint16(10); // maxFunctionDefs
  buf.writeUint16(0); // maxInstructionDefs
  buf.writeUint16(255); // maxStackElements
  buf.writeUint16(0); // maxSizeOfInstructions
  buf.writeUint16(0); // maxComponentElements
  buf.writeUint16(0); // maxComponentDepth

  return buf;
}

module.exports = createMaxpTable;

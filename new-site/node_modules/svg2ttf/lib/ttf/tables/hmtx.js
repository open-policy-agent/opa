'use strict';

// See documentation here: http://www.microsoft.com/typography/otspec/hmtx.htm

var _ = require('lodash');
var ByteBuffer = require('../../byte_buffer.js');

function createHtmxTable(font) {

  var buf = ByteBuffer.prototype.create(font.glyphs.length * 4);

  _.forEach(font.glyphs, function (glyph) {
    buf.writeUint16(glyph.width); //advanceWidth
    buf.writeInt16(glyph.lsb); //lsb
  });
  return buf;
}

module.exports = createHtmxTable;

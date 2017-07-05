'use strict';

// See documentation here: http://www.microsoft.com/typography/otspec/loca.htm

var _ = require('lodash');
var ByteBuffer = require('../../byte_buffer.js');

function tableSize(font, isShortFormat) {
  var result = (font.glyphs.length + 1) * (isShortFormat ? 2 : 4); // by glyph count + tail
  return result;
}

function createLocaTable(font) {

  var isShortFormat = font.ttf_glyph_size < 0x20000;

  var buf = ByteBuffer.prototype.create(tableSize(font, isShortFormat));

  var location = 0;
  // Array of offsets in GLYF table for each glyph
  _.forEach(font.glyphs, function (glyph) {
    if (isShortFormat) {
      buf.writeUint16(location);
      location += glyph.ttf_size / 2; // actual location must be divided to 2 in short format
    } else {
      buf.writeUint32(location);
      location += glyph.ttf_size; //actual location is stored as is in long format
    }
  });

  // The last glyph location is stored to get last glyph length
  if (isShortFormat) {
    buf.writeUint16(location);
  } else {
    buf.writeUint32(location);
  }

  return buf;
}

module.exports = createLocaTable;

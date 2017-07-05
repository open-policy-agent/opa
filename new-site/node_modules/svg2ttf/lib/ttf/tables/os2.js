'use strict';

// See documentation here: http://www.microsoft.com/typography/otspec/os2.htm

var _ = require('lodash');
var ByteBuffer = require('../../byte_buffer.js');

//get first glyph unicode
function getFirstCharIndex(font) {
  var minGlyph = _.min(font.glyphs, function(glyph) {
    return (glyph.unicode === 0) ? 0xFFFF : (glyph.unicode || 0);
  });
  if (minGlyph) {
    return minGlyph.unicode > 0xFFFF ? 0xFFFF : (minGlyph.unicode || 0);
  } else {
    return 0xFFFF;
  }
}

//get last glyph unicode
function getLastCharIndex(font) {
  var maxGlyph = _.max(font.glyphs, 'unicode');
  if (maxGlyph) {
    return maxGlyph.unicode > 0xFFFF ? 0xFFFF : (maxGlyph.unicode || 0);
  } else {
    return 0xFFFF;
  }
}

function createOS2Table(font) {

  var buf = ByteBuffer.prototype.create(86);

  buf.writeUint16(1); //version
  buf.writeInt16(font.avgWidth); // xAvgCharWidth
  buf.writeUint16(font.weightClass); // usWeightClass
  buf.writeUint16(font.widthClass); // usWidthClass
  buf.writeInt16(font.fsType); // fsType
  buf.writeInt16(font.ySubscriptXSize); // ySubscriptXSize
  buf.writeInt16(font.ySubscriptYSize); //ySubscriptYSize
  buf.writeInt16(font.ySubscriptXOffset); // ySubscriptXOffset
  buf.writeInt16(font.ySubscriptYOffset); // ySubscriptYOffset
  buf.writeInt16(font.ySuperscriptXSize); // ySuperscriptXSize
  buf.writeInt16(font.ySuperscriptYSize); // ySuperscriptYSize
  buf.writeInt16(font.ySuperscriptXOffset); // ySuperscriptXOffset
  buf.writeInt16(font.ySuperscriptYOffset); // ySuperscriptYOffset
  buf.writeInt16(font.yStrikeoutSize); // yStrikeoutSize
  buf.writeInt16(font.yStrikeoutPosition); // yStrikeoutPosition
  buf.writeInt16(font.familyClass); // sFamilyClass
  buf.writeUint8(font.panose.familyType); // panose.bFamilyType
  buf.writeUint8(font.panose.serifStyle); // panose.bSerifStyle
  buf.writeUint8(font.panose.weight); // panose.bWeight
  buf.writeUint8(font.panose.proportion); // panose.bProportion
  buf.writeUint8(font.panose.contrast); // panose.bContrast
  buf.writeUint8(font.panose.strokeVariation); // panose.bStrokeVariation
  buf.writeUint8(font.panose.armStyle); // panose.bArmStyle
  buf.writeUint8(font.panose.letterform); // panose.bLetterform
  buf.writeUint8(font.panose.midline); // panose.bMidline
  buf.writeUint8(font.panose.xHeight); // panose.bXHeight
  // TODO: This field is used to specify the Unicode blocks or ranges based on the 'cmap' table.
  buf.writeUint32(0); // ulUnicodeRange1
  buf.writeUint32(0); // ulUnicodeRange2
  buf.writeUint32(0); // ulUnicodeRange3
  buf.writeUint32(0); // ulUnicodeRange4
  buf.writeUint32(0x50664564); // achVendID, equal to PfEd
  buf.writeUint16(font.fsSelection); // fsSelection
  buf.writeUint16(getFirstCharIndex(font)); // usFirstCharIndex
  buf.writeUint16(getLastCharIndex(font)); // usLastCharIndex
  buf.writeInt16(font.ascent); // sTypoAscender
  buf.writeInt16(font.descent); // sTypoDescender
  buf.writeInt16(font.lineGap); // lineGap
  // Enlarge win acscent/descent to avoid clipping
  buf.writeInt16(Math.max(font.yMax, font.ascent)); // usWinAscent
  buf.writeInt16(-Math.min(font.yMin, font.descent)); // usWinDescent
  buf.writeInt32(1); // ulCodePageRange1, Latin 1
  buf.writeInt32(0); // ulCodePageRange2

  return buf;
}

module.exports = createOS2Table;

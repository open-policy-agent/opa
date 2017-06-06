'use strict';

// See documentation here: http://www.microsoft.com/typography/otspec/glyf.htm

var _ = require('lodash');
var ByteBuffer = require('../../byte_buffer.js');

function getFlags(glyph) {
  var result = [];

  _.forEach(glyph.ttfContours, function (contour) {
    _.forEach(contour, function (point) {
      var flag = point.onCurve ? 1 : 0;
      if (point.x === 0) {
        flag += 16;
      } else {
        if (-0xFF <= point.x && point.x <= 0xFF) {
          flag += 2; // the corresponding x-coordinate is 1 byte long
        }
        if (point.x > 0 && point.x <= 0xFF) {
          flag += 16; // If x-Short Vector is set, this bit describes the sign of the value, with 1 equalling positive and 0 negative
        }
      }
      if (point.y === 0) {
        flag += 32;
      } else {
        if (-0xFF <= point.y && point.y <= 0xFF) {
          flag += 4; // the corresponding y-coordinate is 1 byte long
        }
        if (point.y > 0 && point.y <= 0xFF) {
          flag += 32; // If y-Short Vector is set, this bit describes the sign of the value, with 1 equalling positive and 0 negative.
        }
      }
      result.push(flag);
    });
  });
  return result;
}

//repeating flags can be packed
function compactFlags(flags) {
  var result = [];
  var prevFlag = -1;
  var firstRepeat = false;

  _.forEach(flags, function (flag) {
    if (prevFlag === flag) {
      if (firstRepeat) {
        result[result.length - 1] += 8; //current flag repeats previous one, need to set 3rd bit of previous flag and set 1 to the current one
        result.push(1);
        firstRepeat = false;
      } else {
        result[result.length - 1]++; //when flag is repeating second or more times, we need to increase the last flag value
      }
    } else {
      firstRepeat = true;
      prevFlag = flag;
      result.push(flag);
    }
  });
  return result;
}

function getCoords(glyph, coordName) {
  var result = [];
  _.forEach(glyph.ttfContours, function (contour) {
    result.push.apply(result, _.pluck(contour, coordName));
  });
  return result;
}

function compactCoords(coords) {
  return _.filter(coords, function (coord) {
    return coord !== 0;
  });
}

//calculates length of glyph data in GLYF table
function glyphDataSize(glyph) {

  if (!glyph.contours.length) {
    return 0;
  }

  var result = 12; //glyph fixed properties
  result += glyph.contours.length * 2; //add contours

  _.forEach(glyph.ttf_x, function (x) {
    //add 1 or 2 bytes for each coordinate depending of its size
    result += ((-0xFF <= x && x <= 0xFF)) ? 1 : 2;
  });

  _.forEach(glyph.ttf_y, function (y) {
    //add 1 or 2 bytes for each coordinate depending of its size
    result += ((-0xFF <= y && y <= 0xFF)) ? 1 : 2;
  });

  // Add flags length to glyph size.
  result += glyph.ttf_flags.length;

  if (result % 4 !== 0) { // glyph size must be divisible by 4.
    result += 4 - result % 4;
  }
  return result;
}

function tableSize(font) {
  var result = 0;
  _.forEach(font.glyphs, function (glyph) {
    glyph.ttf_size = glyphDataSize(glyph);
    result += glyph.ttf_size;
  });
  font.ttf_glyph_size = result; //sum of all glyph lengths
  return result;
}

function createGlyfTable(font) {

  _.forEach(font.glyphs, function (glyph) {
    glyph.ttf_flags = getFlags(glyph);
    glyph.ttf_flags = compactFlags(glyph.ttf_flags);
    glyph.ttf_x = getCoords(glyph, 'x');
    glyph.ttf_x = compactCoords(glyph.ttf_x);
    glyph.ttf_y = getCoords(glyph, 'y');
    glyph.ttf_y = compactCoords(glyph.ttf_y);
  });

  var buf = ByteBuffer.prototype.create(tableSize(font));

  _.forEach(font.glyphs, function (glyph) {

    if (!glyph.contours.length) {
      return;
    }

    var offset = buf.tell();
    buf.writeInt16(glyph.contours.length); // numberOfContours
    buf.writeInt16(glyph.xMin); // xMin
    buf.writeInt16(glyph.yMin); // yMin
    buf.writeInt16(glyph.xMax); // xMax
    buf.writeInt16(glyph.yMax); // yMax

    // Array of end points
    var endPtsOfContours = -1;

    var ttfContours = glyph.ttfContours;

    _.forEach(ttfContours, function (contour) {
      endPtsOfContours += contour.length;
      buf.writeInt16(endPtsOfContours);
    });

    buf.writeInt16(0); // instructionLength, is not used here

    // Array of flags
    _.forEach(glyph.ttf_flags, function (flag) {
      buf.writeInt8(flag);
    });

    // Array of X relative coordinates
    _.forEach(glyph.ttf_x, function (x) {
      if (-0xFF <= x && x <= 0xFF) {
        buf.writeUint8(Math.abs(x));
      } else {
        buf.writeInt16(x);
      }
    });

    // Array of Y relative coordinates
    _.forEach(glyph.ttf_y, function (y) {
      if (-0xFF <= y && y <= 0xFF) {
        buf.writeUint8(Math.abs(y));
      } else {
        buf.writeInt16(y);
      }
    });

    var tail = (buf.tell() - offset) % 4;
    if (tail !== 0) { // glyph size must be divisible by 4.
      for (; tail < 4; tail++) {
        buf.writeUint8(0);
      }
    }
  });
  return buf;
}

module.exports = createGlyfTable;

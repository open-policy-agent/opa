'use strict';

var _ = require('lodash');
var DOMParser = require('xmldom').DOMParser;
var math  = require('./math');

// supports multibyte characters
function getUnicode(character) {
  if (character.length === 1) {
    // 2 bytes
    return character.charCodeAt(0);
  } else if (character.length === 2) {
    // 4 bytes
    var surrogate1 = character.charCodeAt(0);
    var surrogate2 = character.charCodeAt(1);
    /*jshint bitwise: false*/
    return ((surrogate1 & 0x3FF) << 10) + (surrogate2 & 0x3FF) + 0x10000;
  }
}

function getGlyph(glyphElem) {
  var glyph = {};

  glyph.d = glyphElem.getAttribute('d');

  if (glyphElem.getAttribute('unicode')) {
    glyph.character = glyphElem.getAttribute('unicode');
    glyph.unicode = getUnicode(glyph.character);
  }

  glyph.name = glyphElem.getAttribute('glyph-name');

  if (glyphElem.getAttribute('horiz-adv-x')) {
    glyph.width = parseInt(glyphElem.getAttribute('horiz-adv-x'), 10);
  }

  return glyph;
}

function load(str) {
  var attrs;

  var doc = (new DOMParser()).parseFromString(str, "application/xml");

  var metadata = doc.getElementsByTagName('metadata')[0];
  var fontElem = doc.getElementsByTagName('font')[0];
  var fontFaceElem = fontElem.getElementsByTagName('font-face')[0];

  var font = {
    id: fontElem.getAttribute('id') || 'fontello',
    familyName: fontFaceElem.getAttribute('font-family') || 'fontello',
    glyphs: [],
    stretch: fontFaceElem.getAttribute('font-stretch') || 'normal'
  };

  // Doesn't work with complex content like <strong>Copyright:></strong><em>Fontello</em>
  if (metadata && metadata.textContent) {
    font.metadata = metadata.textContent;
  }

  // Get <font> numeric attributes
  attrs = {
    width:        'horiz-adv-x',
    //height:       'vert-adv-y',
    horizOriginX: 'horiz-origin-x',
    horizOriginY: 'horiz-origin-y',
    vertOriginX:  'vert-origin-x',
    vertOriginY:  'vert-origin-y'
  };
  _.forEach(attrs, function(val, key) {
    if (fontElem.hasAttribute(val)) { font[key] = parseInt(fontElem.getAttribute(val), 10); }
  });

  // Get <font-face> numeric attributes
  attrs = {
    ascent:     'ascent',
    descent:    'descent',
    unitsPerEm: 'units-per-em'
  };
  _.forEach(attrs, function(val, key) {
    if (fontFaceElem.hasAttribute(val)) { font[key] = parseInt(fontFaceElem.getAttribute(val), 10); }
  });

  if (fontFaceElem.hasAttribute('font-weight')) {
    font.weightClass = fontFaceElem.getAttribute('font-weight');
  }

  var missingGlyphElem = fontElem.getElementsByTagName('missing-glyph')[0];
  if (missingGlyphElem) {

    font.missingGlyph = {};
    font.missingGlyph.d = missingGlyphElem.getAttribute('d') || '';

    if (missingGlyphElem.getAttribute('horiz-adv-x')) {
      font.missingGlyph.width = parseInt(missingGlyphElem.getAttribute('horiz-adv-x'), 10);
    }
  }

  _.forEach(fontElem.getElementsByTagName('glyph'), function (glyphElem) {
    font.glyphs.push(getGlyph(glyphElem));
  });

  return font;
}


function cubicToQuad(segment, index, x, y) {
  if (segment[0] === 'C') {
    var quadCurves = math.bezierCubicToQuad(
      new math.Point(x, y),
      new math.Point(segment[1], segment[2]),
      new math.Point(segment[3], segment[4]),
      new math.Point(segment[5], segment[6]),
      0.3
    );

    var res = [];
    _.forEach(quadCurves, function(curve) {
      res.push(['Q', curve[1].x, curve[1].y, curve[2].x, curve[2].y]);
    });
    return res;
  }
}


// Converts svg points to contours.  All points must be converted
// to relative ones, smooth curves must be converted to generic ones
// before this conversion.
//
function toSfntCoutours(svgPath) {
  var resContours = [];
  var resContour = [];

  svgPath.iterate(function(segment, index, x, y) {

    //start new contour
    if (index === 0 || segment[0] === 'M') {
      resContour = [];
      resContours.push(resContour);
    }

    var name = segment[0];
    if (name === 'Q') {
      //add control point of quad spline, it is not on curve
      resContour.push({ x: segment[1], y: segment[2], onCurve: false });
    }

    // add on-curve point
    if (name === 'H') {
      // vertical line has Y coordinate only, X remains the same
      resContour.push({ x: segment[1], y: y, onCurve: true });
    } else if (name === 'V') {
      // horizontal line has X coordinate only, Y remains the same
      resContour.push({ x: x, y: segment[1], onCurve: true });
    } else if (name !== 'Z') {
      // for all commands (except H and V) X and Y are placed in the end of the segment
      resContour.push({ x: segment[segment.length - 2], y: segment[segment.length - 1], onCurve: true });
    }

  });
  return resContours;
}


module.exports.load = load;
module.exports.cubicToQuad = cubicToQuad;
module.exports.toSfntCoutours = toSfntCoutours;

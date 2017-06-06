/*
 * Copyright: Vitaly Puzrin
 * Author: Sergey Batishchev <snb2003@rambler.ru>
 *
 * Written for fontello.com project.
 */

'use strict';

var _       = require('lodash');
var SvgPath = require('svgpath');
var svg     = require('./lib/svg');
var sfnt    = require('./lib/sfnt');

function svg2ttf(svgString, options) {
  var font = new sfnt.Font();
  var svgFont = svg.load(svgString);

  options = options || {};

  font.id = options.id || svgFont.id;
  font.familyName = options.familyname || svgFont.familyName || svgFont.id;
  font.copyright = options.copyright || svgFont.metadata;
  font.sfntNames.push({ id: 2, value: options.subfamilyname || 'Regular' }); // subfamily name
  font.sfntNames.push({ id: 4, value: options.fullname || svgFont.id }); // full name
  font.sfntNames.push({ id: 5, value: 'Version 1.0' }); // version ID for TTF name table
  font.sfntNames.push({ id: 6, value: options.fullname || svgFont.id }); // Postscript name for the font, required for OSX Font Book

  // Try to fill font metrics or guess defaults
  //
  font.unitsPerEm   = svgFont.unitsPerEm || 1000;
  font.horizOriginX = svgFont.horizOriginX || 0;
  font.horizOriginY = svgFont.horizOriginY || 0;
  font.vertOriginX  = svgFont.vertOriginX || 0;
  font.vertOriginY  = svgFont.vertOriginY || 0;
  // need to correctly convert text values, use default (400) until compleete
  //font.weightClass = svgFont.weightClass;
  font.width    = svgFont.width || svgFont.unitsPerEm;
  font.height   = svgFont.height || svgFont.unitsPerEm;
  font.descent  = !isNaN(svgFont.descent) ? svgFont.descent : -font.vertOriginY;
  font.ascent   = svgFont.ascent || (font.unitsPerEm - font.vertOriginY);

  var glyphs = font.glyphs;

  // add SVG glyphs to SFNT font
  _.forEach(svgFont.glyphs, function (svgGlyph) {
    var glyph = new sfnt.Glyph();

    glyph.unicode = svgGlyph.unicode;
    glyph.name = svgGlyph.name;
    glyph.d = svgGlyph.d;
    glyph.height = svgGlyph.height || font.height;
    glyph.width = svgGlyph.width || font.width;
    glyphs.push(glyph);
  });

  var notDefGlyph = _.find(glyphs, function(glyph) {
    return glyph.name === '.notdef';
  });

  // add missing glyph to SFNT font
  // also, check missing glyph existance and single instance
  var missingGlyph;
  if (svgFont.missingGlyph) {
    missingGlyph = new sfnt.Glyph();
    missingGlyph.unicode = 0;
    missingGlyph.d = svgFont.missingGlyph.d;
    missingGlyph.height = svgFont.missingGlyph.height || font.height;
    missingGlyph.width = svgFont.missingGlyph.width || font.width;
    glyphs.push(missingGlyph);

    if (notDefGlyph) { //duplicate definition, we need to remove .notdef glyph
      glyphs.splice(_.indexOf(glyphs, notDefGlyph), 1);
    }
  } else if (notDefGlyph) { // .notdef glyph is exists, we need to set its unicode to 0
    notDefGlyph.unicode = 0;
  }
  else { // no missing glyph and .notdef glyph, we need to create missing glyph
    missingGlyph = new sfnt.Glyph();
    missingGlyph.unicode = 0;
    glyphs.push(missingGlyph);
  }

  // sort glyphs by unicode
  glyphs.sort(function (a, b) {
    if ((a.unicode === undefined) !== (b.unicode === undefined)) {
      return b.unicode === undefined ? -1 : 1;
    } else {
      return a.unicode < b.unicode ? -1 : 1;
    }
  });

  var nextID = 0;

  //add IDs
  _.forEach(glyphs, function(glyph) {
    glyph.id = nextID;
    nextID++;
  });

  _.forEach(glyphs, function (glyph) {

    //SVG transformations
    var svgPath = new SvgPath(glyph.d)
      .abs()
      .unshort()
      .iterate(svg.cubicToQuad);
    var sfntContours = svg.toSfntCoutours(svgPath);

    // Add contours to SFNT font
    glyph.contours = _.map(sfntContours, function (sfntContour) {
      var contour = new sfnt.Contour();

      contour.points = _.map(sfntContour, function (sfntPoint) {
        var point = new sfnt.Point();
        point.x = sfntPoint.x;
        point.y = sfntPoint.y;
        point.onCurve = sfntPoint.onCurve;
        return point;
      });

      return contour;
    });
  });

  var ttf = sfnt.toTTF(font);
  return ttf;
}

module.exports = svg2ttf;

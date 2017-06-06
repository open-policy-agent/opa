/*
 * svgicons2svgfont
 * https://github.com/nfroidure/svgicons2svgfont
 *
 * Copyright (c) 2013 Nicolas Froidure, Cameron Hunter
 * Licensed under the MIT license.
 */
"use strict";

// http://www.whizkidtech.redprince.net/bezier/circle/
var KAPPA = ((Math.sqrt(2)-1)/3)*4;

// Transform helpers (will move elsewhere later)
function parseTransforms(value) {
 return value.match(
  /(rotate|translate|scale|skewX|skewY|matrix)\s*\(([^\)]*)\)\s*/g
 ).map(function(transform) {
  return transform.match(/[\w\.\-]+/g);
 });
}
function transformPath(path, transforms) {
  transforms.forEach(function(transform) {
    path[transform[0]].apply(path, transform.slice(1).map(function(n) {
      return parseFloat(n, 10);
    }));
  });
  return path;
}
function applyTransforms(d, parents) {
  var transforms = [];
  parents.forEach(function(parent) {
    if('undefined' !== typeof parent.attributes.transform) {
      transforms = transforms.concat(parseTransforms(parent.attributes.transform));
    }
  });
  return transformPath(new SVGPathData(d), transforms).encode();
}

// Shapes helpers (should also move elsewhere)


function rectToPath(attributes) {
  var x = 'undefined' !== typeof attributes.x ?
    parseFloat(attributes.x, 10) :
    0;
  var y = 'undefined' !== typeof attributes.y ?
    parseFloat(attributes.y, 10) :
    0;
  var width = 'undefined' !== typeof attributes.width ?
    parseFloat(attributes.width, 10) :
    0;
  var height = 'undefined' !== typeof attributes.height ?
    parseFloat(attributes.height, 10) :
    0;
  var rx = 'undefined' !== typeof attributes.rx ?
    parseFloat(attributes.rx, 10) :
    0;
  var ry = 'undefined' !== typeof attributes.ry ?
    parseFloat(attributes.ry, 10) :
    0;

  return '' +
    // start at the left corner
    'M' + (x + rx) + ' ' + y +
    // top line
    'h' + (width - (rx * 2)) +
    // upper right corner
    ( rx || ry ?
      'a ' + rx + ' ' + ry + ' 0 0 1 ' + rx + ' ' + ry :
      ''
    ) +
    // Draw right side
    'v' + (height - (ry * 2)) +
    // Draw bottom right corner
    ( rx || ry ?
      'a ' + rx + ' ' + ry + ' 0 0 1 ' + (rx * -1) + ' ' + ry :
      ''
    ) +
    // Down the down side
    'h' + ((width  - (rx * 2)) * -1) +
    // Draw bottom right corner
    ( rx || ry ?
      'a ' + rx + ' ' + ry + ' 0 0 1 ' + (rx * -1) + ' ' + (ry * -1) :
      ''
    ) +
    // Down the left side
    'v' + ((height  - (ry * 2)) * -1) +
    // Draw bottom right corner
    ( rx || ry ?
      'a ' + rx + ' ' + ry + ' 0 0 1 ' + rx + ' ' + (ry * -1) :
      ''
    ) +
    // Close path
    'z';
}

// Required modules
var Path = require("path")
  , Stream = require("readable-stream")
  , Sax = require("sax")
  , SVGPathData = require("svg-pathdata")
;

function svgicons2svgfont(glyphs, options) {
  options = options || {};
  options.fontName = options.fontName || 'iconfont';
  options.fixedWidth = options.fixedWidth || false;
  options.descent = options.descent || 0;
  options.round = options.round || 10e12;
  var outputStream = new Stream.PassThrough()
    , log = (options.log || console.log.bind(console))
    , error = options.error || console.error.bind(console);
  glyphs = glyphs.forEach(function (glyph, index, glyphs) {
    // Parsing each icons asynchronously
    var saxStream = Sax.createStream(true)
      , parents = []
    ;
    saxStream.on('closetag', function(tag) {
      parents.pop();
    });
    saxStream.on('opentag', function(tag) {
      parents.push(tag);
      // Checking if any parent rendering is disabled and exit if so
      if(parents.some(function(tag) {
        if('undefined' != typeof tag.attributes.display
            && 'none' == tag.attributes.display.toLowerCase()) {
          return true;
        }
        if('undefined' != typeof tag.attributes.width
            && 0 === parseFloat(tag.attributes.width, 0)) {
          return true;
        }
        if('undefined' != typeof tag.attributes.height
            && 0 === parseFloat(tag.attributes.height, 0)) {
          return true;
        }
        if('undefined' != typeof tag.attributes.viewBox) {
          var values = tag.attributes.viewBox.split(/\s*,*\s|\s,*\s*|,/);
          if(0 === parseFloat(values[2]) || 0 === parseFloat(values[3])) {
            return true;
          }
        }
      })) {
        return;
      }
      // Save the view size
      if('svg' === tag.name) {
        glyph.dX = 0;
        glyph.dY = 0;
        if('viewBox' in tag.attributes) {
          var values = tag.attributes.viewBox.split(/\s*,*\s|\s,*\s*|,/);
          glyph.dX = parseFloat(values[0], 10);
          glyph.dY = parseFloat(values[1], 10);
          glyph.width = parseFloat(values[2], 10);
          glyph.height = parseFloat(values[3], 10);
        }
        if('width' in tag.attributes) {
          glyph.width = parseFloat(tag.attributes.width, 10);
        }
        if('height' in tag.attributes) {
          glyph.height = parseFloat(tag.attributes.height, 10);
        }
        if(!glyph.width || !glyph.height) {
          log('Glyph "' + glyph.name + '" has no size attribute on which to'
            + ' get the gylph dimensions (heigh and width or viewBox'
            + ' attributes)');
          glyph.width = 150;
          glyph.height = 150;
        }
      // Clipping path unsupported
      } else if('clipPath' === tag.name) {
        log('Found a clipPath element in the icon "' + glyph.name + '" the'
          + 'result may be different than expected.');
      // Change rect elements to the corresponding path
      } else if('rect' === tag.name && 'none' !== tag.attributes.fill) {
        glyph.d.push(applyTransforms(rectToPath(tag.attributes), parents));
      } else if('line' === tag.name && 'none' !== tag.attributes.fill) {
        log('Found a line element in the icon "' + glyph.name + '" the result'
          +' could be different than expected.');
        glyph.d.push(applyTransforms(
          // Move to the line start
          'M' + (parseFloat(tag.attributes.x1,10)||0).toString(10)
          + ' ' + (parseFloat(tag.attributes.y1,10)||0).toString(10)
          + ' ' + ((parseFloat(tag.attributes.x1,10)||0)+1).toString(10)
          + ' ' + ((parseFloat(tag.attributes.y1,10)||0)+1).toString(10)
          + ' ' + ((parseFloat(tag.attributes.x2,10)||0)+1).toString(10)
          + ' ' + ((parseFloat(tag.attributes.y2,10)||0)+1).toString(10)
          + ' ' + (parseFloat(tag.attributes.x2,10)||0).toString(10)
          + ' ' + (parseFloat(tag.attributes.y2,10)||0).toString(10)
          + 'Z', parents
        ));
      } else if('polyline' === tag.name && 'none' !== tag.attributes.fill) {
        log('Found a polyline element in the icon "' + glyph.name + '" the'
          +' result could be different than expected.');
        glyph.d.push(applyTransforms(
          'M' + tag.attributes.points, parents
        ));
      } else if('polygon' === tag.name && 'none' !== tag.attributes.fill) {
        glyph.d.push(applyTransforms(
          'M' + tag.attributes.points + 'Z', parents
        ));
      } else if('circle' === tag.name || 'ellipse' === tag.name &&
        'none' !== tag.attributes.fill) {
        var cx = parseFloat(tag.attributes.cx,10)
          , cy = parseFloat(tag.attributes.cy,10)
          , rx = 'undefined' !== typeof tag.attributes.rx ?
              parseFloat(tag.attributes.rx,10) : parseFloat(tag.attributes.r,10)
          , ry = 'undefined' !== typeof tag.attributes.ry ?
              parseFloat(tag.attributes.ry,10) : parseFloat(tag.attributes.r,10);
        glyph.d.push(applyTransforms(
          'M' + (cx - rx) + ',' + cy
          + 'C' + (cx - rx) + ',' + (cy + ry*KAPPA)
          + ' ' + (cx - rx*KAPPA) + ',' + (cy + ry)
          + ' ' + cx + ',' + (cy + ry)
          + 'C' + (cx + rx*KAPPA) + ',' + (cy+ry)
          + ' ' + (cx + rx) + ',' + (cy + ry*KAPPA)
          + ' ' + (cx + rx) + ',' + cy
          + 'C' + (cx + rx) + ',' + (cy - ry*KAPPA)
          + ' ' + (cx + rx*KAPPA) + ',' + (cy - ry)
          + ' ' + cx + ',' + (cy - ry)
          + 'C' + (cx - rx*KAPPA) + ',' + (cy - ry)
          + ' ' + (cx - rx) + ',' + (cy - ry*KAPPA)
          + ' ' + (cx - rx) + ',' + cy
          + 'Z', parents
        ));
      } else if('path' === tag.name && tag.attributes.d &&
        'none' !== tag.attributes.fill) {
        glyph.d.push(applyTransforms(tag.attributes.d, parents));
      }
    });
    saxStream.on('end', function() {
      glyph.running = false;
      if(glyphs.every(function(glyph) {
        return !glyph.running;
      })) {
        var fontWidth = (glyphs.length > 1 ? glyphs.reduce(function (curMax, glyph) {
              return Math.max(curMax, glyph.width);
            }, 0) : glyphs[0].width)
          , fontHeight = options.fontHeight ||
            (glyphs.length > 1 ? glyphs.reduce(function (curMax, glyph) {
              return Math.max(curMax, glyph.height);
            }, 0) : glyphs[0].height);
        if((!options.normalize)
          && fontHeight>(glyphs.length > 1 ? glyphs.reduce(function (curMin, glyph) {
          return Math.min(curMin, glyph.height);
        }, Infinity) : glyphs[0].height)) {
          log('The provided icons does not have the same height it could lead'
            +' to unexpected results. Using the normalize option could'
            +' solve the problem.');
        }
        // Output the SVG file
        // (find a SAX parser that allows modifying SVG on the fly)
        outputStream.write('<?xml version="1.0" standalone="no"?> \n\
<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd" >\n\
<svg xmlns="http://www.w3.org/2000/svg">\n\
<defs>\n\
  <font id="' + options.fontName + '" horiz-adv-x="' + fontWidth + '">\n\
    <font-face font-family="' + options.fontName + '"\n\
      units-per-em="' + fontHeight + '" ascent="' + (fontHeight - options.descent) + '"\n\
      descent="' + options.descent + '" />\n\
    <missing-glyph horiz-adv-x="0" />\n');
        glyphs.forEach(function(glyph) {
          var ratio = fontHeight / glyph.height
            , d = '';
          if(options.fixedWidth) {
            glyph.width = fontWidth;
          }
          if(options.normalize) {
            glyph.height = fontHeight;
            if(!options.fixedWidth) {
              glyph.width *= ratio;
            }
          }
          glyph.d.forEach(function(cD) {
            d+=' '+new SVGPathData(cD)
                .toAbs()
                .translate(-glyph.dX, -glyph.dY)
                .scale(
                  options.normalize ? ratio : 1,
                  options.normalize ? ratio : 1)
                .ySymetry(glyph.height - options.descent)
                .round(options.round)
                .encode();
          });
          if(options.centerHorizontally) {
            // Naive bounds calculation (should draw, then calculate bounds...)
            var pathData = new SVGPathData(d);
            var bounds = {
              x1:Infinity,
              y1:Infinity,
              x2:0,
              y2:0
            };
            pathData.toAbs().commands.forEach(function(command) {
              bounds.x1 = 'undefined' != typeof command.x && command.x < bounds.x1 ? command.x : bounds.x1;
              bounds.y1 = 'undefined' != typeof command.y && command.y < bounds.y1 ? command.y : bounds.y1;
              bounds.x2 = 'undefined' != typeof command.x && command.x > bounds.x2 ? command.x : bounds.x2;
              bounds.y2 = 'undefined' != typeof command.y && command.y > bounds.y2 ? command.y : bounds.y2;
            });
            d = pathData
              .translate(((glyph.width - (bounds.x2 - bounds.x1)) / 2) - bounds.x1)
              .round(options.round)
              .encode();
          }
          delete glyph.d;
          delete glyph.running;
          outputStream.write('\
    <glyph glyph-name="' + glyph.name + '"\n\
      unicode="&#x' + (glyph.codepoint.toString(16)).toUpperCase() + ';"\n\
      horiz-adv-x="' + glyph.width + '" d="' + d +'" />\n');
        });
        outputStream.write('\
  </font>\n\
</defs>\n\
</svg>\n');
        outputStream.on('finish', function() {
          log("Font created");
          'function' === (typeof options.callback) && (options.callback)(glyphs);
        });
        outputStream.end();
      }
    });
    if('string' !== typeof glyph.name) {
      throw new Error('Please provide a name for the glyph at index ' + index);
    }
    if(glyphs.some(function(g) {
      return (g !== glyph && g.name === glyph.name);
    })) {
      throw new Error('The glyph name "' + glyph.name + '" must be unique.');
    }
    if('number' !== typeof glyph.codepoint) {
      throw new Error('Please provide a codepoint for the glyph "' + glyph.name + '"');
    }
    if(glyphs.some(function(g) {
      return (g !== glyph && g.codepoint === glyph.codepoint);
    })) {
      throw new Error('The glyph "' + glyph.name
        + '" codepoint seems to be used already elsewhere.');
    }
    glyph.running = true;
    glyph.d = [];
    glyph.stream.pipe(saxStream);
  });
  return outputStream;
}

module.exports = svgicons2svgfont;

'use strict';

var _ = require('lodash');

var Font = function () {
  this.ascent = 850;
  this.copyright = '';
  this.createdDate = new Date();
  this.glyphs = [];
  this.isFixedPitch = 0;
  this.italicAngle = 0;
  this.familyClass = 0; // No Classification
  this.familyName = '';
  this.fsSelection = 0x40; // Characters are in the standard weight/style for the font.
  this.fsType = 8; // No subsetting: When this bit is set, the font may not be subsetted prior to embedding.
  this.lineGap = 90;
  this.lowestRecPPEM = 8;
  this.macStyle = 0;
  this.modifiedDate = new Date();
  this.panose = {
    familyType: 2, // Latin Text
    serifStyle: 0, // any
    weight: 5, // book
    proportion: 3, //modern
    contrast: 0, //any
    strokeVariation: 0, //any,
    armStyle: 0, //any,
    letterform: 0, //any,
    midline: 0, //any,
    xHeight: 0 //any,
  };
  this.revision = 1;
  this.sfntNames = [];
  this.underlineThickness = 0;
  this.unitsPerEm = 1000;
  this.weightClass = 400; // normal
  this.width = 1000;
  this.widthClass = 5; // Medium (normal)
  this.ySubscriptXOffset = 0;
  this.ySuperscriptXOffset = 0;
  this.int_descent = -150;

//getters and setters

  Object.defineProperty(this, 'descent', {
    get: function () {
      return this.int_descent;
    },
    set: function (value) {
      this.int_descent = parseInt(Math.round(-Math.abs(value)), 10);
    }
  });

  this.__defineGetter__('avgCharWidth', function () {
    if (this.glyphs.length === 0) {
      return 0;
    }
    var widths = _.map(this.glyphs, 'width');
    return parseInt(widths.reduce(function (prev, cur) {
      return prev + cur;
    }) / widths.length, 10);
  });

  Object.defineProperty(this, 'ySubscriptXSize', {
    get: function () {
      return parseInt(this.int_ySubscriptXSize !== undefined ? this.int_ySubscriptXSize : (this.width * 0.6347), 10);
    },
    set: function (value) {
      this.int_ySubscriptXSize = value;
    }
  });

  Object.defineProperty(this, 'ySubscriptYSize', {
    get: function () {
      return parseInt(this.int_ySubscriptYSize !== undefined ? this.int_ySubscriptYSize : ((this.ascent - this.descent) * 0.7), 10);
    },
    set: function (value) {
      this.int_ySubscriptYSize = value;
    }
  });

  Object.defineProperty(this, 'ySubscriptYOffset', {
    get: function () {
      return parseInt(this.int_ySubscriptYOffset !== undefined ? this.int_ySubscriptYOffset : ((this.ascent - this.descent) * 0.14), 10);
    },
    set: function (value) {
      this.int_ySubscriptYOffset = value;
    }
  });

  Object.defineProperty(this, 'ySuperscriptXSize', {
    get: function () {
      return parseInt(this.int_ySuperscriptXSize !== undefined ? this.int_ySuperscriptXSize : (this.width * 0.6347), 10);
    },
    set: function (value) {
      this.int_ySuperscriptXSize = value;
    }
  });

  Object.defineProperty(this, 'ySuperscriptYSize', {
    get: function () {
      return parseInt(this.int_ySuperscriptYSize !== undefined ? this.int_ySuperscriptYSize : ((this.ascent - this.descent) * 0.7), 10);
    },
    set: function (value) {
      this.int_ySuperscriptYSize = value;
    }
  });

  Object.defineProperty(this, 'ySuperscriptYOffset', {
    get: function () {
      return parseInt(this.int_ySuperscriptYOffset !== undefined ? this.int_ySuperscriptYOffset : ((this.ascent - this.descent) * 0.48), 10);
    },
    set: function (value) {
      this.int_ySuperscriptYOffset = value;
    }
  });

  Object.defineProperty(this, 'yStrikeoutSize', {
    get: function () {
      return parseInt(this.int_yStrikeoutSize !== undefined ? this.int_yStrikeoutSize : ((this.ascent - this.descent) * 0.049), 10);
    },
    set: function (value) {
      this.int_yStrikeoutSize = value;
    }
  });

  Object.defineProperty(this, 'yStrikeoutPosition', {
    get: function () {
      return parseInt(this.int_yStrikeoutPosition !== undefined ? this.int_yStrikeoutPosition : ((this.ascent - this.descent) * 0.258), 10);
    },
    set: function (value) {
      this.int_yStrikeoutPosition = value;
    }
  });

  Object.defineProperty(this, 'minLsb', {
    get: function () {
      return parseInt(_.min(_.pluck(this.glyphs, 'lsb')), 10);
    }
  });

  Object.defineProperty(this, 'minRsb', {
    get: function () {
      var minRsb = _.reduce(this.glyphs, function (minRsb, glyph) {
        return Math.min(minRsb || 0, glyph.width - glyph.lsb - (glyph.xMax - glyph.xMin));
      }, minRsb);
      return minRsb !== undefined ? minRsb : this.width;
    }
  });

  Object.defineProperty(this, 'xMin', {
    get: function () {
      var xMin = _.reduce(this.glyphs, function (xMin, glyph) {
        return Math.min(xMin || 0, glyph.xMin);
      }, xMin);
      return xMin !== undefined ? xMin : this.width;
    }
  });

  Object.defineProperty(this, 'yMin', {
    get: function () {
      var yMin = _.reduce(this.glyphs, function (yMin, glyph) {
        return Math.min(yMin || 0, glyph.yMin);
      }, yMin);
      return yMin !== undefined ? yMin : this.width;
    }
  });

  Object.defineProperty(this, 'xMax', {
    get: function () {
      var xMax = _.reduce(this.glyphs, function (xMax, glyph) {
        return Math.max(xMax || 0, glyph.xMax);
      }, xMax);
      return xMax !== undefined ? xMax : this.width;
    }
  });

  Object.defineProperty(this, 'yMax', {
    get: function () {
      var yMax = _.reduce(this.glyphs, function (yMax, glyph) {
        return Math.max(yMax || 0, glyph.yMax);
      }, yMax);
      return yMax !== undefined ? yMax : this.width;
    }
  });

  Object.defineProperty(this, 'avgWidth', {
    get: function () {
      var len = this.glyphs.length;
      if (len === 0) {
        return this.width;
      }
      var sumWidth = _.reduce(this.glyphs, function (sumWidth, glyph) {
        return (sumWidth || 0) + glyph.width;
      }, sumWidth);
      return Math.round(sumWidth / len);
    }
  });

  Object.defineProperty(this, 'maxWidth', {
    get: function () {
      var maxWidth = _.reduce(this.glyphs, function (maxWidth, glyph) {
        return Math.max(maxWidth || 0, glyph.width);
      }, maxWidth);
      return maxWidth !== undefined ? maxWidth : this.width;
    }
  });

  Object.defineProperty(this, 'maxExtent', {
    get: function () {
      var maxExtent = _.reduce(this.glyphs, function (maxExtent, glyph) {
        return Math.max(maxExtent || 0, glyph.lsb + glyph.xMax - glyph.xMin);
      }, maxExtent);
      return maxExtent !== undefined ? maxExtent : this.width;
    }
  });

  Object.defineProperty(this, 'lineGap', {
    get: function () {
      return parseInt(this.int_lineGap !== undefined ? this.lineGap : ((this.ascent - this.descent) * 0.09), 10);
    },
    set: function (value) {
      this.int_lineGap = value;
    }
  });

  Object.defineProperty(this, 'underlinePosition', {
    get: function () {
      return parseInt(this.int_underlinePosition !== undefined ? this.underlinePosition : ((this.ascent - this.descent) * 0.01), 10);
    },
    set: function (value) {
      this.int_underlinePosition = value;
    }
  });
};


var Glyph = function () {
  this.contours = [];
  this.id = '';
  this.height = 0;
  this.lsb = 0;
  this.name = '';
  this.size = 0;
  this.unicode = 0;
  this.width = 0;
};

Object.defineProperty(Glyph.prototype, 'xMin', {
  get: function () {
    var xMin = 0;
    var hasPoints = false;
    _.forEach(this.contours, function (contour) {
      _.forEach(contour.points, function (point) {
        xMin = Math.min(xMin, Math.floor(point.x));
        hasPoints = true;
      });

    });
    return hasPoints ? xMin : 0;
  }
});

Object.defineProperty(Glyph.prototype, 'xMax', {
  get: function () {
    var xMax = 0;
    var hasPoints = false;
    _.forEach(this.contours, function (contour) {
      _.forEach(contour.points, function (point) {
        xMax = Math.max(xMax, -Math.floor(-point.x));
        hasPoints = true;
      });

    });
    return hasPoints ? xMax : 0;
  }
});

Object.defineProperty(Glyph.prototype, 'yMin', {
  get: function () {
    var yMin = 0;
    var hasPoints = false;
    _.forEach(this.contours, function (contour) {
      _.forEach(contour.points, function (point) {
        yMin = Math.min(yMin, Math.floor(point.y));
        hasPoints = true;
      });

    });
    return hasPoints ? yMin : 0;
  }
});

Object.defineProperty(Glyph.prototype, 'yMax', {
  get: function () {
    var yMax = 0;
    var hasPoints = false;
    _.forEach(this.contours, function (contour) {
      _.forEach(contour.points, function (point) {
        yMax = Math.max(yMax, -Math.floor(-point.y));
        hasPoints = true;
      });

    });
    return hasPoints ? yMax : 0;
  }
});

var Contour = function () {
  this.points = [];
};

var Point = function () {
  this.onCurve = true;
  this.x = 0;
  this.y = 0;
};

var SfntName = function () {
  this.id = 0;
  this.value = '';
};

module.exports.Font = Font;
module.exports.Glyph = Glyph;
module.exports.Contour = Contour;
module.exports.Point = Point;
module.exports.SfntName = SfntName;
module.exports.toTTF = require('./ttf');

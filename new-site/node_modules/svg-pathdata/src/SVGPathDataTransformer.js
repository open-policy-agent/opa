// Transform SVG PathData
// http://www.w3.org/TR/SVG/paths.html#PathDataBNF

// Access to SVGPathData constructor
var SVGPathData = require('./SVGPathData.js')

// TransformStream inherance required modules
  , TransformStream = require('readable-stream').Transform
  , util = require('util')
;

// Inherit of transform stream
util.inherits(SVGPathDataTransformer, TransformStream);

function SVGPathDataTransformer(transformFunction) {
  // Ensure new were used
  if(!(this instanceof SVGPathDataTransformer)) {
    return new (SVGPathDataTransformer.bind.apply(SVGPathDataTransformer,
      [SVGPathDataTransformer].concat([].slice.call(arguments, 0))));
  }

  // Transform function needed
  if('function' !== typeof transformFunction) {
    throw new Error('Please provide a transform callback to receive commands.')
  }
  this._transformer = transformFunction.apply(null, [].slice.call(arguments, 1));
  if('function' !== typeof this._transformer) {
    throw new Error('Please provide a valid transform (returning a function).')
  }

  // Parent constructor
  TransformStream.call(this, {
    objectMode: true
  });
}

SVGPathDataTransformer.prototype._transform = function(commands, encoding, done) {
  if(!(commands instanceof Array)) {
    commands = [commands];
  }
  for(var i=0, j=commands.length; i<j; i++) {
    this.push(this._transformer(commands[i]));
  }
  done();
};

// Predefined transforming functions
// Rounds commands values
SVGPathDataTransformer.ROUND = function roundGenerator(roundVal) {
  roundVal = roundVal || 10e12;
  return function round(command) {
    // x1/y1 values
    if('undefined' !== typeof command.x1) {
      command.x1 = Math.round(command.x1*roundVal)/roundVal;
    }
    if('undefined' !== typeof command.y1) {
      command.y1 = Math.round(command.y1*roundVal)/roundVal;
    }
    // x2/y2 values
    if('undefined' !== typeof command.x2) {
      command.x2 = Math.round(command.x2*roundVal)/roundVal;
    }
    if('undefined' !== typeof command.y2) {
      command.y2 = Math.round(command.y2*roundVal)/roundVal;
    }
    // Finally x/y values
    if('undefined' !== typeof command.x) {
      command.x = Math.round(command.x*roundVal,12)/roundVal;
    }
    if('undefined' !== typeof command.y) {
      command.y = Math.round(command.y*roundVal,12)/roundVal;
    }
    return command;
  };
};

// Relative to absolute commands
SVGPathDataTransformer.TO_ABS = function toAbsGenerator() {
  var prevX = 0, prevY = 0, pathStartX = NaN, pathStartY = NaN;
  return function toAbs(command) {
    if(isNaN(pathStartX) && (command.type&SVGPathData.DRAWING_COMMANDS)) {
      pathStartX = prevX;
      pathStartY = prevY;
    }
    if((command.type&SVGPathData.CLOSE_PATH) && !isNaN(pathStartX)) {
      prevX = isNaN(pathStartX) ? 0 : pathStartX;
      prevY = isNaN(pathStartY) ? 0 : pathStartY;
      pathStartX = NaN;
      pathStartY = NaN;
    }
    if(command.relative) {
      // x1/y1 values
      if('undefined' !== typeof command.x1) {
        command.x1 = prevX + command.x1;
      }
      if('undefined' !== typeof command.y1) {
        command.y1 = prevY + command.y1;
      }
      // x2/y2 values
      if('undefined' !== typeof command.x2) {
        command.x2 = prevX + command.x2;
      }
      if('undefined' !== typeof command.y2) {
        command.y2 = prevY + command.y2;
      }
      // Finally x/y values
      if('undefined' !== typeof command.x) {
        command.x = prevX + command.x;
      }
      if('undefined' !== typeof command.y) {
        command.y = prevY + command.y;
      }
      command.relative = false;
    }
    prevX = ('undefined' !== typeof command.x ? command.x : prevX);
    prevY = ('undefined' !== typeof command.y ? command.y : prevY);
    return command;
  };
};

// Absolute to relative commands
SVGPathDataTransformer.TO_REL = function toRelGenerator() {
  var prevX = 0, prevY = 0;
  return function toRel(command) {
    if(!command.relative) {
      // x1/y1 values
      if('undefined' !== typeof command.x1) {
        command.x1 = command.x1 - prevX;
      }
      if('undefined' !== typeof command.y1) {
        command.y1 = command.y1 - prevY;
      }
      // x2/y2 values
      if('undefined' !== typeof command.x2) {
        command.x2 = command.x2 - prevX;
      }
      if('undefined' !== typeof command.y2) {
        command.y2 = command.y2 - prevY;
      }
      // Finally x/y values
      if('undefined' !== typeof command.x) {
        command.x = command.x - prevX;
      }
      if('undefined' !== typeof command.y) {
        command.y = command.y - prevY;
      }
    command.relative = true;
    }
    prevX = ('undefined' !== typeof command.x ? prevX + command.x : prevX);
    prevY = ('undefined' !== typeof command.y ? prevY + command.y : prevY);
    return command;
  };
};

// SVG Transforms : http://www.w3.org/TR/SVGTiny12/coords.html#TransformList
// Matrix : http://apike.ca/prog_svg_transform.html
SVGPathDataTransformer.MATRIX = function matrixGenerator(a, b, c, d, e, f) {
  var prevX, prevY;
  if('number' !== typeof a, 'number' !== typeof b,
    'number' !== typeof c, 'number' !== typeof d,
    'number' !== typeof e, 'number' !== typeof f) {
    throw new Error('A matrix transformation requires parameters [a,b,c,d,e,f]'
      +' to be set and to be numbers.');
  }
  return function matrix(command) {
    var origX = command.x, origX1 = command.x1, origX2 = command.x2;
    if('undefined' !== typeof command.x) {
      command.x =  command.x * a +
        ('undefined' !== typeof command.y ?
          command.y : (command.relative ? 0 : prevY || 0)
        ) * c
        + (command.relative && 'undefined' !== typeof prevX ? 0 : e);
    }
    if('undefined' !== typeof command.y) {
      command.y = ('undefined' !== typeof origX ?
          origX : (command.relative ? 0 : prevX || 0)
        ) * b
        + command.y * d
        + (command.relative && 'undefined' !== typeof prevY ? 0 : f);
    }
    if('undefined' !== typeof command.x1) {
      command.x1 = command.x1 * a + command.y1 * c
        + (command.relative && 'undefined' !== typeof prevX ? 0 : e);
    }
    if('undefined' !== typeof command.y1) {
      command.y1 = origX1 * b + command.y1 * d
        + (command.relative && 'undefined' !== typeof prevY ? 0 : f);
    }
    if('undefined' !== typeof command.x2) {
      command.x2 = command.x2 * a + command.y2 * c
        + (command.relative && 'undefined' !== typeof prevX ? 0 : e);
    }
    if('undefined' !== typeof command.y2) {
      command.y2 = origX2 * b + command.y2 * d
        + (command.relative && 'undefined' !== typeof prevY ? 0 : f);
    }
    prevX = ('undefined' !== typeof command.x ?
      (command.relative ? (prevX || 0) + command.x : command.x) :
      prevX || 0);
    prevY = ('undefined' !== typeof command.y ?
      (command.relative ? (prevY || 0) + command.y : command.y) :
      prevY || 0);
    return command;
  };
};

// Rotation
SVGPathDataTransformer.ROTATE = function rotateGenerator(a, x, y) {
  if('number' !== typeof a) {
    throw new Error('A rotate transformation requires the parameter a'
      +' to be set and to be a number.');
  }
  return (function(toOrigin, doRotate, fromOrigin) {
    return function rotate(command) {
      return fromOrigin(doRotate(toOrigin(command)));
    };
  })(SVGPathDataTransformer.TRANSLATE(-(x || 0), -(y || 0))
   , SVGPathDataTransformer.MATRIX(Math.cos(a), Math.sin(a),
      -Math.sin(a), Math.cos(a), 0, 0)
   , SVGPathDataTransformer.TRANSLATE(x || 0, y || 0)
  );
};

// Translation
SVGPathDataTransformer.TRANSLATE = function translateGenerator(dX, dY) {
  if('number' !== typeof dX) {
    throw new Error('A translate transformation requires the parameter dX'
      +' to be set and to be a number.');
  }
  return SVGPathDataTransformer.MATRIX(1, 0, 0, 1, dX, dY || 0);
};

// Scaling
SVGPathDataTransformer.SCALE = function scaleGenerator(dX, dY) {
  if('number' !== typeof dX) {
    throw new Error('A scale transformation requires the parameter dX'
      +' to be set and to be a number.');
  }
  return SVGPathDataTransformer.MATRIX(dX, 0, 0, dY || dX, 0, 0);
};

// Skew
SVGPathDataTransformer.SKEW_X = function skewXGenerator(a) {
  if('number' !== typeof a) {
    throw new Error('A skewX transformation requires the parameter x'
      +' to be set and to be a number.');
  }
  return SVGPathDataTransformer.MATRIX(1, 0, Math.atan(a), 1, 0, 0);
}
SVGPathDataTransformer.SKEW_Y = function skewYGenerator(a) {
  if('number' !== typeof a) {
    throw new Error('A skewY transformation requires the parameter y'
      +' to be set and to be a number.');
  }
  return SVGPathDataTransformer.MATRIX(1, Math.atan(a), 0, 1, 0, 0);
}

// Symetry througth the X axis
SVGPathDataTransformer.X_AXIS_SIMETRY = function xSymetryGenerator(xDecal) {
  return (function(toAbs, scale, translate) {
    return function xSymetry(command) {
      return translate(scale(toAbs(command)));
    };
  })(SVGPathDataTransformer.TO_ABS(),
    SVGPathDataTransformer.SCALE(-1, 1),
    SVGPathDataTransformer.TRANSLATE(xDecal || 0, 0)
  );
};

// Symetry througth the Y axis
SVGPathDataTransformer.Y_AXIS_SIMETRY = function ySymetryGenerator(yDecal) {
  return (function(toAbs, scale, translate) {
    return function ySymetry(command) {
      return translate(scale(toAbs(command)));
    };
  })(SVGPathDataTransformer.TO_ABS(),
    SVGPathDataTransformer.SCALE(1, -1),
    SVGPathDataTransformer.TRANSLATE(0, yDecal || 0)
  );
};

// Convert arc commands to curve commands
SVGPathDataTransformer.A_TO_C = function a2CGenerator() {
  var prevX = 0, prevY = 0, args;
  return (function(toAbs) {
    return function a2C(command) {
      var commands = [];
      command = toAbs(command);
      if(command.type === SVGPathData.ARC) {
        args = a2c(prevX, prevY, command.rX, command.rX, command.xRot,
          command.lArcFlag, command.sweepFlag, command.x, command.y);
        prevX = command.x; prevY = command.y;
        for(var i=0, ii=args.length; i<ii; i+=6) {
          commands.push({
            type: SVGPathData.CURVE_TO,
            relative: false,
            x2: args[i],
            y2: args[i+1],
            x1: args[i+2],
            y1: args[i+3],
            x: args[i+4],
            y: args[i+5]
          });
        }
        return commands;
      } else {
        prevX = command.x; prevY = command.y;
        return command;
      }
    };
  })(SVGPathDataTransformer.TO_ABS());
};

function a2c (x1, y1, rx, ry, angle, large_arc_flag, sweep_flag, x2, y2, recursive) {
  var PI = Math.PI
  // Borrowed from https://github.com/PPvG/svg-path/blob/master/lib/Path.js#L208
  // that were borrowed from https://github.com/DmitryBaranovskiy/raphael/blob/4d97d4ff5350bb949b88e6d78b877f76ea8b5e24/raphael.js#L2216-L2304
  // (MIT licensed; http://raphaeljs.com/license.html).
  // --------------------------------------------------------------------------
  // for more information of where this math came from visit:
  // http://www.w3.org/TR/SVG11/implnote.html#ArcImplementationNotes
  var _120 = PI * 120 / 180,
      rad = PI / 180 * (+angle || 0),
      res = [],
      xy,
      rotate = function (x, y, rad) {
        var X = x * Math.cos(rad) - y * Math.sin(rad),
        Y = x * Math.sin(rad) + y * Math.cos(rad);
        return {x: X, y: Y};
      };
  if (!recursive) {
    xy = rotate(x1, y1, -rad);
    x1 = xy.x;
    y1 = xy.y;
    xy = rotate(x2, y2, -rad);
    x2 = xy.x;
    y2 = xy.y;
    var cos = Math.cos(PI / 180 * angle),
        sin = Math.sin(PI / 180 * angle),
        x = (x1 - x2) / 2,
        y = (y1 - y2) / 2;
    var h = (x * x) / (rx * rx) + (y * y) / (ry * ry);
    if (h > 1) {
      h = Math.sqrt(h);
      rx = h * rx;
      ry = h * ry;
    }
    var rx2 = rx * rx,
        ry2 = ry * ry,
        k = (large_arc_flag == sweep_flag ? -1 : 1) *
          Math.sqrt(Math.abs((rx2 * ry2 - rx2 * y * y - ry2 * x * x) /
          (rx2 * y * y + ry2 * x * x))),
        cx = k * rx * y / ry + (x1 + x2) / 2,
        cy = k * -ry * x / rx + (y1 + y2) / 2,
        f1 = Math.asin(((y1 - cy) / ry).toFixed(9)),
        f2 = Math.asin(((y2 - cy) / ry).toFixed(9));

    f1 = x1 < cx ? PI - f1 : f1;
    f2 = x2 < cx ? PI - f2 : f2;
    f1 < 0 && (f1 = PI * 2 + f1);
    f2 < 0 && (f2 = PI * 2 + f2);
    if (sweep_flag && f1 > f2) {
      f1 = f1 - PI * 2;
    }
    if (!sweep_flag && f2 > f1) {
      f2 = f2 - PI * 2;
    }
  } else {
    f1 = recursive[0];
    f2 = recursive[1];
    cx = recursive[2];
    cy = recursive[3];
  }
  var df = f2 - f1;
  if (Math.abs(df) > _120) {
    var f2old = f2,
        x2old = x2,
        y2old = y2;
    f2 = f1 + _120 * (sweep_flag && f2 > f1 ? 1 : -1);
    x2 = cx + rx * Math.cos(f2);
    y2 = cy + ry * Math.sin(f2);
    res = a2c(x2, y2, rx, ry, angle, 0, sweep_flag, x2old, y2old,
      [f2, f2old, cx, cy]);
  }
  df = f2 - f1;
  var c1 = Math.cos(f1),
      s1 = Math.sin(f1),
      c2 = Math.cos(f2),
      s2 = Math.sin(f2),
      t = Math.tan(df / 4),
      hx = 4 / 3 * rx * t,
      hy = 4 / 3 * ry * t,
      m1 = [x1, y1],
      m2 = [x1 + hx * s1, y1 - hy * c1],
      m3 = [x2 + hx * s2, y2 - hy * c2],
      m4 = [x2, y2];
  m2[0] = 2 * m1[0] - m2[0];
  m2[1] = 2 * m1[1] - m2[1];
  if (recursive) {
    return [m2, m3, m4].concat(res);
  } else {
    res = [m2, m3, m4].concat(res).join().split(',');
    var newres = [];
    for (var i = 0, ii = res.length; i < ii; i++) {
      newres[i] = i % 2 ? rotate(res[i - 1], res[i], rad).y :
        rotate(res[i], res[i + 1], rad).x;
    }
    return newres;
  }
}

module.exports = SVGPathDataTransformer;


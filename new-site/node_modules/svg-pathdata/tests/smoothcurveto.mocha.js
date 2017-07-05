var assert = (
    global && global.chai
    ? global.chai.assert
    : require('chai').assert
  )
  , SVGPathData = (
    global && global.SVGPathData
    ? global.SVGPathData
    : require(__dirname + '/../src/SVGPathData.js')
  )
;

describe("Parsing smooth curve to commands", function() {

  beforeEach(function() {
  });

  afterEach(function() {
  });

  it("should not work when badly declarated", function() {
    assert.throw(function() {
      new SVGPathData('S');
    }, SyntaxError, 'Unterminated command at the path end.');
    assert.throw(function() {
      new SVGPathData('S10');
    }, SyntaxError, 'Unterminated command at the path end.');
    assert.throw(function() {
      new SVGPathData('S10 10');
    }, SyntaxError, 'Unterminated command at the path end.');
    assert.throw(function() {
      new SVGPathData('S10 10 10');
    }, SyntaxError, 'Unterminated command at the path end.');
    assert.throw(function() {
      new SVGPathData('S10 10 10 10 10 10');
    }, SyntaxError, 'Unterminated command at the path end.');
    assert.throw(function() {
      new SVGPathData('S10 10 10S10 10 10 10');
    }, SyntaxError, 'Unterminated command at index 9.');
  });

  it("should work with comma separated coordinates", function() {
    var commands = new SVGPathData('S123,456 789,987').commands;
    assert.equal(commands[0].type, SVGPathData.SMOOTH_CURVE_TO);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].x2, '123');
    assert.equal(commands[0].y2, '456');
    assert.equal(commands[0].x, '789');
    assert.equal(commands[0].y, '987');
  });

  it("should work with space separated coordinates", function() {
    var commands = new SVGPathData('S123 456 789 987').commands;
    assert.equal(commands[0].type, SVGPathData.SMOOTH_CURVE_TO);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].x2, '123');
    assert.equal(commands[0].y2, '456');
    assert.equal(commands[0].x, '789');
    assert.equal(commands[0].y, '987');
  });

  it("should work with nested separated complexer coordinate pairs", function() {
    var commands = new SVGPathData('S-10.0032e-5,-20.0032e-5 -30.0032e-5,-40.0032e-5').commands;
    assert.equal(commands[0].type, SVGPathData.SMOOTH_CURVE_TO);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].x2, '-10.0032e-5');
    assert.equal(commands[0].y2, '-20.0032e-5');
    assert.equal(commands[0].x, '-30.0032e-5');
    assert.equal(commands[0].y, '-40.0032e-5');
  });

  it("should work with multiple pairs of coordinates", function() {
    var commands = new SVGPathData('S-10.0032e-5,-20.0032e-5 -30.0032e-5,-40.0032e-5 -10.0032e-5,-20.0032e-5 -30.0032e-5,-40.0032e-5 -10.0032e-5,-20.0032e-5 -30.0032e-5,-40.0032e-5').commands;
    assert.equal(commands[0].type, SVGPathData.SMOOTH_CURVE_TO);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].x2, '-10.0032e-5');
    assert.equal(commands[0].y2, '-20.0032e-5');
    assert.equal(commands[0].x, '-30.0032e-5');
    assert.equal(commands[0].y, '-40.0032e-5');
    assert.equal(commands[1].type, SVGPathData.SMOOTH_CURVE_TO);
    assert.equal(commands[1].relative, false);
    assert.equal(commands[1].x2, '-10.0032e-5');
    assert.equal(commands[1].y2, '-20.0032e-5');
    assert.equal(commands[1].x, '-30.0032e-5');
    assert.equal(commands[1].y, '-40.0032e-5');
    assert.equal(commands[2].type, SVGPathData.SMOOTH_CURVE_TO);
    assert.equal(commands[2].relative, false);
    assert.equal(commands[2].x2, '-10.0032e-5');
    assert.equal(commands[2].y2, '-20.0032e-5');
    assert.equal(commands[2].x, '-30.0032e-5');
    assert.equal(commands[2].y, '-40.0032e-5');
  });

  it("should work with multiple declarated pairs of coordinates", function() {
    var commands = new SVGPathData('S-10.0032e-5,-20.0032e-5 -30.0032e-5,-40.0032e-5 s-10.0032e-5,-20.0032e-5 -30.0032e-5,-40.0032e-5 S-10.0032e-5,-20.0032e-5 -30.0032e-5,-40.0032e-5').commands;
    assert.equal(commands[0].type, SVGPathData.SMOOTH_CURVE_TO);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].x2, '-10.0032e-5');
    assert.equal(commands[0].y2, '-20.0032e-5');
    assert.equal(commands[0].x, '-30.0032e-5');
    assert.equal(commands[0].y, '-40.0032e-5');
    assert.equal(commands[1].type, SVGPathData.SMOOTH_CURVE_TO);
    assert.equal(commands[1].relative, true);
    assert.equal(commands[1].x2, '-10.0032e-5');
    assert.equal(commands[1].y2, '-20.0032e-5');
    assert.equal(commands[1].x, '-30.0032e-5');
    assert.equal(commands[1].y, '-40.0032e-5');
    assert.equal(commands[2].type, SVGPathData.SMOOTH_CURVE_TO);
    assert.equal(commands[2].relative, false);
    assert.equal(commands[2].x2, '-10.0032e-5');
    assert.equal(commands[2].y2, '-20.0032e-5');
    assert.equal(commands[2].x, '-30.0032e-5');
    assert.equal(commands[2].y, '-40.0032e-5');
  });

});

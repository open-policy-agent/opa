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

describe("Parsing curve to commands", function() {

  beforeEach(function() {
  });

  afterEach(function() {
  });

  it("should not work when badly declarated", function() {
    assert.throw(function() {
      new SVGPathData('C');
    }, SyntaxError, 'Unterminated command at the path end.');
    assert.throw(function() {
      new SVGPathData('C10');
    }, SyntaxError, 'Unterminated command at the path end.');
    assert.throw(function() {
      new SVGPathData('C10 10');
    }, SyntaxError, 'Unterminated command at the path end.');
    assert.throw(function() {
      new SVGPathData('C10 10 10');
    }, SyntaxError, 'Unterminated command at the path end.');
    assert.throw(function() {
      new SVGPathData('C10 10 10 10');
    }, SyntaxError, 'Unterminated command at the path end.');
    assert.throw(function() {
      new SVGPathData('C10 10 10 10 10');
    }, SyntaxError, 'Unterminated command at the path end.');
    assert.throw(function() {
      new SVGPathData('C10 10 10 10 10 10 10 10');
    }, SyntaxError, 'Unterminated command at the path end.');
    assert.throw(function() {
      new SVGPathData('C10 10 10C10 10 10 10 10 10');
    }, SyntaxError, 'Unterminated command at index 9.');
  });

  it("should work with comma separated coordinates", function() {
    var commands = new SVGPathData('C123,456 789,987 654,321').commands;
    assert.equal(commands[0].type, SVGPathData.CURVE_TO);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].x2, '123');
    assert.equal(commands[0].y2, '456');
    assert.equal(commands[0].x1, '789');
    assert.equal(commands[0].y1, '987');
    assert.equal(commands[0].x, '654');
    assert.equal(commands[0].y, '321');
  });

  it("should work with space separated coordinates", function() {
    var commands = new SVGPathData('C123 456 789 987 654 321').commands;
    assert.equal(commands[0].type, SVGPathData.CURVE_TO);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].x2, '123');
    assert.equal(commands[0].y2, '456');
    assert.equal(commands[0].x1, '789');
    assert.equal(commands[0].y1, '987');
    assert.equal(commands[0].x, '654');
    assert.equal(commands[0].y, '321');
  });

  it("should work with nested separated complexer coordinate pairs", function() {
    var commands = new SVGPathData(
      'C-10.0032e-5,-20.0032e-5 -30.0032e-5,-40.0032e-5 -50.0032e-5,-60.0032e-5'
    ).commands;
    assert.equal(commands[0].type, SVGPathData.CURVE_TO);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].x2, '-10.0032e-5');
    assert.equal(commands[0].y2, '-20.0032e-5');
    assert.equal(commands[0].x1, '-30.0032e-5');
    assert.equal(commands[0].y1, '-40.0032e-5');
    assert.equal(commands[0].x, '-50.0032e-5');
    assert.equal(commands[0].y, '-60.0032e-5');
  });

  it("should work with multiple pairs of coordinates", function() {
    var commands = new SVGPathData('\
      C-10.0032e-5,-20.0032e-5 -30.0032e-5,-40.0032e-5 -50.0032e-5,-60.0032e-5\
      -10.0032e-5,-20.0032e-5 -30.0032e-5,-40.0032e-5 -50.0032e-5,-60.0032e-5\
      -10.0032e-5,-20.0032e-5 -30.0032e-5,-40.0032e-5 -50.0032e-5,-60.0032e-5'
    ).commands;
    assert.equal(commands[0].type, SVGPathData.CURVE_TO);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].x2, '-10.0032e-5');
    assert.equal(commands[0].y2, '-20.0032e-5');
    assert.equal(commands[0].x1, '-30.0032e-5');
    assert.equal(commands[0].y1, '-40.0032e-5');
    assert.equal(commands[0].x, '-50.0032e-5');
    assert.equal(commands[0].y, '-60.0032e-5');
    assert.equal(commands[1].type, SVGPathData.CURVE_TO);
    assert.equal(commands[1].relative, false);
    assert.equal(commands[1].x2, '-10.0032e-5');
    assert.equal(commands[1].y2, '-20.0032e-5');
    assert.equal(commands[1].x1, '-30.0032e-5');
    assert.equal(commands[1].y1, '-40.0032e-5');
    assert.equal(commands[1].x, '-50.0032e-5');
    assert.equal(commands[1].y, '-60.0032e-5');
    assert.equal(commands[2].type, SVGPathData.CURVE_TO);
    assert.equal(commands[2].relative, false);
    assert.equal(commands[2].x2, '-10.0032e-5');
    assert.equal(commands[2].y2, '-20.0032e-5');
    assert.equal(commands[2].x1, '-30.0032e-5');
    assert.equal(commands[2].y1, '-40.0032e-5');
    assert.equal(commands[2].x, '-50.0032e-5');
    assert.equal(commands[2].y, '-60.0032e-5');
  });

  it("should work with multiple declarated pairs of coordinates", function() {
    var commands = new SVGPathData('\
      C-10.0032e-5,-20.0032e-5 -30.0032e-5,-40.0032e-5 -50.0032e-5,-60.0032e-5\
      c-10.0032e-5,-20.0032e-5 -30.0032e-5,-40.0032e-5 -50.0032e-5,-60.0032e-5\
      C-10.0032e-5,-20.0032e-5 -30.0032e-5,-40.0032e-5 -50.0032e-5,-60.0032e-5'
    ).commands;
    assert.equal(commands[0].type, SVGPathData.CURVE_TO);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].x2, '-10.0032e-5');
    assert.equal(commands[0].y2, '-20.0032e-5');
    assert.equal(commands[0].x1, '-30.0032e-5');
    assert.equal(commands[0].y1, '-40.0032e-5');
    assert.equal(commands[0].x, '-50.0032e-5');
    assert.equal(commands[0].y, '-60.0032e-5');
    assert.equal(commands[1].type, SVGPathData.CURVE_TO);
    assert.equal(commands[1].relative, true);
    assert.equal(commands[1].x2, '-10.0032e-5');
    assert.equal(commands[1].y2, '-20.0032e-5');
    assert.equal(commands[1].x1, '-30.0032e-5');
    assert.equal(commands[1].y1, '-40.0032e-5');
    assert.equal(commands[1].x, '-50.0032e-5');
    assert.equal(commands[1].y, '-60.0032e-5');
    assert.equal(commands[2].type, SVGPathData.CURVE_TO);
    assert.equal(commands[2].relative, false);
    assert.equal(commands[2].x2, '-10.0032e-5');
    assert.equal(commands[2].y2, '-20.0032e-5');
    assert.equal(commands[2].x1, '-30.0032e-5');
    assert.equal(commands[2].y1, '-40.0032e-5');
    assert.equal(commands[2].x, '-50.0032e-5');
    assert.equal(commands[2].y, '-60.0032e-5');
  });

});

describe("Encoding curve to commands", function() {

  it("should work with one command", function() {
      assert.equal(
        new SVGPathData('C-10.0032e-5 -20.0032e-5 -30.0032e-5 -40.0032e-5 -50.0032e-5 -60.0032e-5').encode(),
        'C-0.000100032 -0.000200032 -0.000300032 -0.000400032 -0.000500032 -0.000600032'
      );
  });

  it("should work with several commands", function() {
      assert.equal(
        new SVGPathData('C-10.0032e-5 -20.0032e-5 -30.0032e-5 -40.0032e-5 -50.0032e-5 -60.0032e-5C-10.0032e-5 -20.0032e-5 -30.0032e-5 -40.0032e-5 -50.0032e-5 -60.0032e-5C-10.0032e-5 -20.0032e-5 -30.0032e-5 -40.0032e-5 -50.0032e-5 -60.0032e-5').encode(),
        'C-0.000100032 -0.000200032 -0.000300032 -0.000400032 -0.000500032 -0.000600032C-0.000100032 -0.000200032 -0.000300032 -0.000400032 -0.000500032 -0.000600032C-0.000100032 -0.000200032 -0.000300032 -0.000400032 -0.000500032 -0.000600032'
      );
  });

});

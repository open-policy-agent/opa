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

describe("Parsing eliptical arc commands", function() {

  beforeEach(function() {
  });

  afterEach(function() {
  });

  it("should not work when badly declarated", function() {
    assert.throw(function() {
      new SVGPathData('A');
    }, SyntaxError, 'Unterminated command at the path end.');
    assert.throw(function() {
      new SVGPathData('A 30');
    }, SyntaxError, 'Unterminated command at the path end.');
    assert.throw(function() {
      new SVGPathData('A 30 50');
    }, SyntaxError, 'Unterminated command at the path end.');
    assert.throw(function() {
      new SVGPathData('A 30 50 0');
    }, SyntaxError, 'Unterminated command at the path end.');
    assert.throw(function() {
      new SVGPathData('A 30 50 0 0');
    }, SyntaxError, 'Unterminated command at the path end.');
    assert.throw(function() {
      new SVGPathData('A 30 50 0 0 1');
    }, SyntaxError, 'Unterminated command at the path end.');
    assert.throw(function() {
      new SVGPathData('A 30 50 0 0 1 162.55');
    }, SyntaxError, 'Unterminated command at the path end.');
    assert.throw(function() {
      new SVGPathData('A 30 50 0 0 1 A 30 50 0 0 1 162.55 162.45');
    }, SyntaxError, 'Unterminated command at index 14.');
  });

  it("should not work with bad rX value", function() {
    assert.throw(function() {
      new SVGPathData('A,-30,50,0,0,1,162.55,162.45');
    }, SyntaxError, 'Expected positive number, got "-30" at index "5"');
  });

  it("should not work with bad rY value", function() {
    assert.throw(function() {
      new SVGPathData('A,30,-50,0,0,1,162.55,162.45');
    }, SyntaxError, 'Expected positive number, got "-50" at index "8"');
  });

  it("should not work with bad lArcFlag value", function() {
    assert.throw(function() {
      new SVGPathData('A,30,50,0,15,1,162.55,162.45');
    }, SyntaxError, 'Expected a flag, got "15" at index "12"');
  });

  it("should not work with bad sweepFlag value", function() {
    assert.throw(function() {
      new SVGPathData('A,30,50,0,0,15,162.55,162.45');
    }, SyntaxError, 'Expected a flag, got "15" at index "14"');
  });

  it("should work with comma separated coordinates", function() {
    var commands = new SVGPathData('A,30,50,0,0,1,162.55,162.45').commands;
    assert.equal(commands[0].type, SVGPathData.ARC);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].rX, '30');
    assert.equal(commands[0].rY, '50');
    assert.equal(commands[0].xRot, '0');
    assert.equal(commands[0].lArcFlag, '0');
    assert.equal(commands[0].sweepFlag, '1');
    assert.equal(commands[0].x, '162.55');
    assert.equal(commands[0].y, '162.45');
  });

  it("should work with space separated coordinates", function() {
    var commands = new SVGPathData('A 30 50 0 0 1 162.55 162.45').commands;
    assert.equal(commands[0].type, SVGPathData.ARC);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].rX, '30');
    assert.equal(commands[0].rY, '50');
    assert.equal(commands[0].xRot, '0');
    assert.equal(commands[0].lArcFlag, '0');
    assert.equal(commands[0].sweepFlag, '1');
    assert.equal(commands[0].x, '162.55');
    assert.equal(commands[0].y, '162.45');
  });

  it("should work with nested separated complexer coordinate pairs", function() {
    var commands = new SVGPathData('A 30,50 0 0 1 162.55,162.45').commands;
    assert.equal(commands[0].type, SVGPathData.ARC);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].rX, '30');
    assert.equal(commands[0].rY, '50');
    assert.equal(commands[0].xRot, '0');
    assert.equal(commands[0].lArcFlag, '0');
    assert.equal(commands[0].sweepFlag, '1');
    assert.equal(commands[0].x, '162.55');
    assert.equal(commands[0].y, '162.45');
  });

  it("should work with multiple pairs of coordinates", function() {
    var commands = new SVGPathData(
      'A 10.0032e-5,20.0032e-5 0 0 1 -30.0032e-5,-40.0032e-5\
      50.0032e-5,60.0032e-5 0 1 0 -70.0032e-5,-80.0032e-5\
      90.0032e-5,90.0032e-5 0 0 1 -80.0032e-5,-70.0032e-5').commands;
    assert.equal(commands[0].type, SVGPathData.ARC);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].rX, '10.0032e-5');
    assert.equal(commands[0].rY, '20.0032e-5');
    assert.equal(commands[0].xRot, '0');
    assert.equal(commands[0].lArcFlag, '0');
    assert.equal(commands[0].sweepFlag, '1');
    assert.equal(commands[0].x, '-30.0032e-5');
    assert.equal(commands[0].y, '-40.0032e-5');
    assert.equal(commands[1].type, SVGPathData.ARC);
    assert.equal(commands[1].relative, false);
    assert.equal(commands[1].rX, '50.0032e-5');
    assert.equal(commands[1].rY, '60.0032e-5');
    assert.equal(commands[1].xRot, '0');
    assert.equal(commands[1].lArcFlag, '1');
    assert.equal(commands[1].sweepFlag, '0');
    assert.equal(commands[1].x, '-70.0032e-5');
    assert.equal(commands[1].y, '-80.0032e-5');
    assert.equal(commands[2].type, SVGPathData.ARC);
    assert.equal(commands[2].relative, false);
    assert.equal(commands[2].rX, '90.0032e-5');
    assert.equal(commands[2].rY, '90.0032e-5');
    assert.equal(commands[2].xRot, '0');
    assert.equal(commands[2].lArcFlag, '0');
    assert.equal(commands[2].sweepFlag, '1');
    assert.equal(commands[2].x, '-80.0032e-5');
    assert.equal(commands[2].y, '-70.0032e-5');
  });

  it("should work with multiple declarated pairs of coordinates", function() {
    var commands = new SVGPathData(
      'A 10.0032e-5,20.0032e-5 0 0 1 -30.0032e-5,-40.0032e-5\
      a50.0032e-5,60.0032e-5 0 1 0 -70.0032e-5,-80.0032e-5\
      A90.0032e-5,90.0032e-5 0 0 1 -80.0032e-5,-70.0032e-5').commands;
    assert.equal(commands[0].type, SVGPathData.ARC);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].rX, '10.0032e-5');
    assert.equal(commands[0].rY, '20.0032e-5');
    assert.equal(commands[0].xRot, '0');
    assert.equal(commands[0].lArcFlag, '0');
    assert.equal(commands[0].sweepFlag, '1');
    assert.equal(commands[0].x, '-30.0032e-5');
    assert.equal(commands[0].y, '-40.0032e-5');
    assert.equal(commands[1].type, SVGPathData.ARC);
    assert.equal(commands[1].relative, true);
    assert.equal(commands[1].rX, '50.0032e-5');
    assert.equal(commands[1].rY, '60.0032e-5');
    assert.equal(commands[1].xRot, '0');
    assert.equal(commands[1].lArcFlag, '1');
    assert.equal(commands[1].sweepFlag, '0');
    assert.equal(commands[1].x, '-70.0032e-5');
    assert.equal(commands[1].y, '-80.0032e-5');
    assert.equal(commands[2].type, SVGPathData.ARC);
    assert.equal(commands[2].relative, false);
    assert.equal(commands[2].rX, '90.0032e-5');
    assert.equal(commands[2].rY, '90.0032e-5');
    assert.equal(commands[2].xRot, '0');
    assert.equal(commands[2].lArcFlag, '0');
    assert.equal(commands[2].sweepFlag, '1');
    assert.equal(commands[2].x, '-80.0032e-5');
    assert.equal(commands[2].y, '-70.0032e-5');
  });

});

describe("Encoding eliptical arc commands", function() {

  it("should work with one command", function() {
      assert.equal(
        new SVGPathData('A30 50 0 0 1 162.55 162.45').encode(),
        'A30 50 0 0 1 162.55 162.45'
      );
  });

  it("should work with several commands", function() {
      assert.equal(
        new SVGPathData('A30 50 0 0 1 162.55 162.45A30 50 0 0 1 162.55 162.45A30 50 0 0 1 162.55 162.45').encode(),
        'A30 50 0 0 1 162.55 162.45A30 50 0 0 1 162.55 162.45A30 50 0 0 1 162.55 162.45'
      );
  });

});

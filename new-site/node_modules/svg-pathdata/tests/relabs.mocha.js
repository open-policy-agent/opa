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

describe("Converting relative commands to absolute ones", function() {

  it("should work with m commands", function() {
    var commands = new SVGPathData('m-100,100M10,10m10,10m-1,-1').toAbs().commands;
    assert.equal(commands[0].type, SVGPathData.MOVE_TO);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].x, -100);
    assert.equal(commands[0].y, 100);
    assert.equal(commands[1].type, SVGPathData.MOVE_TO);
    assert.equal(commands[1].relative, false);
    assert.equal(commands[1].x, 10);
    assert.equal(commands[1].y, 10);
    assert.equal(commands[2].type, SVGPathData.MOVE_TO);
    assert.equal(commands[2].relative, false);
    assert.equal(commands[2].x, 20);
    assert.equal(commands[2].y, 20);
    assert.equal(commands[3].type, SVGPathData.MOVE_TO);
    assert.equal(commands[3].relative, false);
    assert.equal(commands[3].x, 19);
    assert.equal(commands[3].y, 19);
  });

  it("should work with h commands", function() {
    var commands = new SVGPathData('h100H10h10h-5').toAbs().commands;
    assert.equal(commands[0].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].x, 100);
    assert.equal(commands[1].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[1].relative, false);
    assert.equal(commands[1].x, 10);
    assert.equal(commands[2].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[2].relative, false);
    assert.equal(commands[2].x, 20);
    assert.equal(commands[3].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[3].relative, false);
    assert.equal(commands[3].x, 15);
  });

  it("should work with v commands", function() {
    var commands = new SVGPathData('v100V10v5v5').toAbs().commands;
    assert.equal(commands[0].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].y, 100);
    assert.equal(commands[1].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[1].relative, false);
    assert.equal(commands[1].y, 10);
    assert.equal(commands[2].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[2].relative, false);
    assert.equal(commands[2].y, 15);
    assert.equal(commands[3].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[3].relative, false);
    assert.equal(commands[3].y, 20);
  });

  it("should work with l commands", function() {
    var commands = new SVGPathData('l100,-100L1,0l2,2l-1,-1').toAbs().commands;
    assert.equal(commands[0].type, SVGPathData.LINE_TO);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].x, 100);
    assert.equal(commands[0].y, -100);
    assert.equal(commands[1].type, SVGPathData.LINE_TO);
    assert.equal(commands[1].relative, false);
    assert.equal(commands[1].x, 1);
    assert.equal(commands[1].y, 0);
    assert.equal(commands[2].type, SVGPathData.LINE_TO);
    assert.equal(commands[2].relative, false);
    assert.equal(commands[2].x, 3);
    assert.equal(commands[2].y, 2);
    assert.equal(commands[3].type, SVGPathData.LINE_TO);
    assert.equal(commands[3].relative, false);
    assert.equal(commands[3].x, 2);
    assert.equal(commands[3].y, 1);
  });

  it("should work with c commands", function() {
    var commands = new SVGPathData('c100,100 100,100 100,100\
      c100,100 100,100 100,100\
      c100,100 100,100 100,100\
      c100,100 100,100 100,100').toAbs().commands;
    assert.equal(commands[0].type, SVGPathData.CURVE_TO);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].x, 100);
    assert.equal(commands[0].y, 100);
    assert.equal(commands[0].x1, 100);
    assert.equal(commands[0].y1, 100);
    assert.equal(commands[0].x2, 100);
    assert.equal(commands[0].y2, 100);
    assert.equal(commands[1].type, SVGPathData.CURVE_TO);
    assert.equal(commands[1].relative, false);
    assert.equal(commands[1].x, 200);
    assert.equal(commands[1].y, 200);
    assert.equal(commands[1].x1, 200);
    assert.equal(commands[1].y1, 200);
    assert.equal(commands[1].x2, 200);
    assert.equal(commands[1].y2, 200);
    assert.equal(commands[2].type, SVGPathData.CURVE_TO);
    assert.equal(commands[2].relative, false);
    assert.equal(commands[2].x, 300);
    assert.equal(commands[2].y, 300);
    assert.equal(commands[2].x1, 300);
    assert.equal(commands[2].y1, 300);
    assert.equal(commands[2].x2, 300);
    assert.equal(commands[2].y2, 300);
    assert.equal(commands[3].type, SVGPathData.CURVE_TO);
    assert.equal(commands[3].relative, false);
    assert.equal(commands[3].x, 400);
    assert.equal(commands[3].y, 400);
    assert.equal(commands[3].x1, 400);
    assert.equal(commands[3].y1, 400);
    assert.equal(commands[3].x2, 400);
    assert.equal(commands[3].y2, 400);
  });

  it("should work with s commands", function() {
    var commands = new SVGPathData('s100,100 100,100\
      s100,100 100,100s100,100 100,100s100,100 100,100').toAbs().commands;
    assert.equal(commands[0].type, SVGPathData.SMOOTH_CURVE_TO);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].x, 100);
    assert.equal(commands[0].y, 100);
    assert.equal(commands[0].x2, 100);
    assert.equal(commands[0].y2, 100);
    assert.equal(commands[1].type, SVGPathData.SMOOTH_CURVE_TO);
    assert.equal(commands[1].relative, false);
    assert.equal(commands[1].x, 200);
    assert.equal(commands[1].y, 200);
    assert.equal(commands[1].x2, 200);
    assert.equal(commands[1].y2, 200);
    assert.equal(commands[2].type, SVGPathData.SMOOTH_CURVE_TO);
    assert.equal(commands[2].relative, false);
    assert.equal(commands[2].x, 300);
    assert.equal(commands[2].y, 300);
    assert.equal(commands[2].x2, 300);
    assert.equal(commands[2].y2, 300);
    assert.equal(commands[3].type, SVGPathData.SMOOTH_CURVE_TO);
    assert.equal(commands[3].relative, false);
    assert.equal(commands[3].x, 400);
    assert.equal(commands[3].y, 400);
    assert.equal(commands[3].x2, 400);
    assert.equal(commands[3].y2, 400);
  });

  it("should work with q commands", function() {
    var commands = new SVGPathData('q-100,100 -100,100q-100,100 -100,100\
      q-100,100 -100,100q-100,100 -100,100').toAbs().commands;
    assert.equal(commands[0].type, SVGPathData.QUAD_TO);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].x, -100);
    assert.equal(commands[0].y, 100);
    assert.equal(commands[0].x1, -100);
    assert.equal(commands[0].y1, 100);
    assert.equal(commands[1].type, SVGPathData.QUAD_TO);
    assert.equal(commands[1].relative, false);
    assert.equal(commands[1].x, -200);
    assert.equal(commands[1].y, 200);
    assert.equal(commands[1].x1, -200);
    assert.equal(commands[1].y1, 200);
    assert.equal(commands[2].type, SVGPathData.QUAD_TO);
    assert.equal(commands[2].relative, false);
    assert.equal(commands[2].x, -300);
    assert.equal(commands[2].y, 300);
    assert.equal(commands[2].x1, -300);
    assert.equal(commands[2].y1, 300);
    assert.equal(commands[3].type, SVGPathData.QUAD_TO);
    assert.equal(commands[3].relative, false);
    assert.equal(commands[3].x, -400);
    assert.equal(commands[3].y, 400);
    assert.equal(commands[3].x1, -400);
    assert.equal(commands[3].y1, 400);
  });

  it("should work with t commands", function() {
    var commands = new SVGPathData('t-100,100t-100,100t10,10t10,10').toAbs().commands;
    assert.equal(commands[0].type, SVGPathData.SMOOTH_QUAD_TO);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].x, -100);
    assert.equal(commands[0].y, 100);
    assert.equal(commands[1].type, SVGPathData.SMOOTH_QUAD_TO);
    assert.equal(commands[1].relative, false);
    assert.equal(commands[1].x, -200);
    assert.equal(commands[1].y, 200);
    assert.equal(commands[2].type, SVGPathData.SMOOTH_QUAD_TO);
    assert.equal(commands[2].relative, false);
    assert.equal(commands[2].x, -190);
    assert.equal(commands[2].y, 210);
    assert.equal(commands[3].type, SVGPathData.SMOOTH_QUAD_TO);
    assert.equal(commands[3].relative, false);
    assert.equal(commands[3].x, -180);
    assert.equal(commands[3].y, 220);
  });

  it("should work with a commands", function() {
    var commands = new SVGPathData('a20,20 180 1 0 -100,100\
      a20,20 180 1 0 -100,100a20,20 180 1 0 -100,100\
      a20,20 180 1 0 -100,100').toAbs().commands;
    assert.equal(commands[0].type, SVGPathData.ARC);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].rX, 20);
    assert.equal(commands[0].rY, 20);
    assert.equal(commands[0].xRot, 180);
    assert.equal(commands[0].lArcFlag, 1);
    assert.equal(commands[0].sweepFlag, 0);
    assert.equal(commands[0].x, -100);
    assert.equal(commands[0].y, 100);
    assert.equal(commands[1].type, SVGPathData.ARC);
    assert.equal(commands[1].relative, false);
    assert.equal(commands[1].rX, 20);
    assert.equal(commands[1].rY, 20);
    assert.equal(commands[1].xRot, 180);
    assert.equal(commands[1].lArcFlag, 1);
    assert.equal(commands[1].sweepFlag, 0);
    assert.equal(commands[1].x, -200);
    assert.equal(commands[1].y, 200);
    assert.equal(commands[2].type, SVGPathData.ARC);
    assert.equal(commands[2].relative, false);
    assert.equal(commands[2].rX, 20);
    assert.equal(commands[2].rY, 20);
    assert.equal(commands[2].xRot, 180);
    assert.equal(commands[2].lArcFlag, 1);
    assert.equal(commands[2].sweepFlag, 0);
    assert.equal(commands[2].x, -300);
    assert.equal(commands[2].y, 300);
    assert.equal(commands[3].type, SVGPathData.ARC);
    assert.equal(commands[3].relative, false);
    assert.equal(commands[3].rX, 20);
    assert.equal(commands[3].rY, 20);
    assert.equal(commands[3].xRot, 180);
    assert.equal(commands[3].lArcFlag, 1);
    assert.equal(commands[3].sweepFlag, 0);
    assert.equal(commands[3].x, -400);
    assert.equal(commands[3].y, 400);
  });

  it("should work with nested commands", function() {
    var commands = new SVGPathData('a20,20 180 1 0 -100,100h10v10l10,10\
      c10,10 20,20 100,100').toAbs().commands;
    assert.equal(commands[0].type, SVGPathData.ARC);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].rX, 20);
    assert.equal(commands[0].rY, 20);
    assert.equal(commands[0].xRot, 180);
    assert.equal(commands[0].lArcFlag, 1);
    assert.equal(commands[0].sweepFlag, 0);
    assert.equal(commands[0].x, -100);
    assert.equal(commands[0].y, 100);
    assert.equal(commands[1].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[1].relative, false);
    assert.equal(commands[1].x, -90);
    assert.equal(commands[2].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[2].relative, false);
    assert.equal(commands[2].y, 110);
    assert.equal(commands[3].type, SVGPathData.LINE_TO);
    assert.equal(commands[3].relative, false);
    assert.equal(commands[3].x, -80);
    assert.equal(commands[3].y, 120);
    assert.equal(commands[4].type, SVGPathData.CURVE_TO);
    assert.equal(commands[4].relative, false);
    assert.equal(commands[4].x, 20);
    assert.equal(commands[4].y, 220);
    assert.equal(commands[4].x1, -60);
    assert.equal(commands[4].y1, 140);
    assert.equal(commands[4].x2, -70);
    assert.equal(commands[4].y2, 130);
  });

});

describe("Converting absolute commands to relative ones", function() {

  it("should work with M commands", function() {
    var commands = new SVGPathData('M100,100M110,90M120,80M130,70').toRel().commands;
    assert.equal(commands[0].type, SVGPathData.MOVE_TO);
    assert.equal(commands[0].relative, true);
    assert.equal(commands[0].x, 100);
    assert.equal(commands[0].y, 100);
    assert.equal(commands[1].type, SVGPathData.MOVE_TO);
    assert.equal(commands[1].relative, true);
    assert.equal(commands[1].x, 10);
    assert.equal(commands[1].y, -10);
    assert.equal(commands[2].type, SVGPathData.MOVE_TO);
    assert.equal(commands[2].relative, true);
    assert.equal(commands[2].x, 10);
    assert.equal(commands[2].y, -10);
    assert.equal(commands[3].type, SVGPathData.MOVE_TO);
    assert.equal(commands[3].relative, true);
    assert.equal(commands[3].x, 10);
    assert.equal(commands[3].y, -10);
  });

  it("should work with M commands", function() {
    var commands = new SVGPathData('M-100,100m90,-90M20,20M19,19').toRel().commands;
    assert.equal(commands[0].type, SVGPathData.MOVE_TO);
    assert.equal(commands[0].relative, true);
    assert.equal(commands[0].x, -100);
    assert.equal(commands[0].y, 100);
    assert.equal(commands[1].type, SVGPathData.MOVE_TO);
    assert.equal(commands[1].relative, true);
    assert.equal(commands[1].x, 90);
    assert.equal(commands[1].y, -90);
    assert.equal(commands[2].type, SVGPathData.MOVE_TO);
    assert.equal(commands[2].relative, true);
    assert.equal(commands[2].x, 30);
    assert.equal(commands[2].y, 10);
    assert.equal(commands[3].type, SVGPathData.MOVE_TO);
    assert.equal(commands[3].relative, true);
    assert.equal(commands[3].x, -1);
    assert.equal(commands[3].y, -1);
  });

  it("should work with H commands", function() {
    var commands = new SVGPathData('H100H10H20H15').toRel().commands;
    assert.equal(commands[0].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[0].relative, true);
    assert.equal(commands[0].x, 100);
    assert.equal(commands[1].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[1].relative, true);
    assert.equal(commands[1].x, -90);
    assert.equal(commands[2].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[2].relative, true);
    assert.equal(commands[2].x, 10);
    assert.equal(commands[3].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[3].relative, true);
    assert.equal(commands[3].x, -5);
  });

  it("should work with V commands", function() {
    var commands = new SVGPathData('V100V10V15V20').toRel().commands;
    assert.equal(commands[0].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[0].relative, true);
    assert.equal(commands[0].y, 100);
    assert.equal(commands[1].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[1].relative, true);
    assert.equal(commands[1].y, -90);
    assert.equal(commands[2].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[2].relative, true);
    assert.equal(commands[2].y, 5);
    assert.equal(commands[3].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[3].relative, true);
    assert.equal(commands[3].y, 5);
  });

  it("should work with L commands", function() {
    var commands = new SVGPathData('L100,-100L1,0L3,2L2,1').toRel().commands;
    assert.equal(commands[0].type, SVGPathData.LINE_TO);
    assert.equal(commands[0].relative, true);
    assert.equal(commands[0].x, 100);
    assert.equal(commands[0].y, -100);
    assert.equal(commands[1].type, SVGPathData.LINE_TO);
    assert.equal(commands[1].relative, true);
    assert.equal(commands[1].x, -99);
    assert.equal(commands[1].y, 100);
    assert.equal(commands[2].type, SVGPathData.LINE_TO);
    assert.equal(commands[2].relative, true);
    assert.equal(commands[2].x, 2);
    assert.equal(commands[2].y, 2);
    assert.equal(commands[3].type, SVGPathData.LINE_TO);
    assert.equal(commands[3].relative, true);
    assert.equal(commands[3].x, -1);
    assert.equal(commands[3].y, -1);
  });

  it("should work with C commands", function() {
    var commands = new SVGPathData('C100,100 100,100 100,100\
      C200,200 200,200 200,200\
      C300,300 300,300 300,300\
      C400,400 400,400 400,400').toRel().commands;
    assert.equal(commands[0].type, SVGPathData.CURVE_TO);
    assert.equal(commands[0].relative, true);
    assert.equal(commands[0].x, 100);
    assert.equal(commands[0].y, 100);
    assert.equal(commands[0].x1, 100);
    assert.equal(commands[0].y1, 100);
    assert.equal(commands[0].x2, 100);
    assert.equal(commands[0].y2, 100);
    assert.equal(commands[1].type, SVGPathData.CURVE_TO);
    assert.equal(commands[1].relative, true);
    assert.equal(commands[1].x, 100);
    assert.equal(commands[1].y, 100);
    assert.equal(commands[1].x1, 100);
    assert.equal(commands[1].y1, 100);
    assert.equal(commands[1].x2, 100);
    assert.equal(commands[1].y2, 100);
    assert.equal(commands[2].type, SVGPathData.CURVE_TO);
    assert.equal(commands[2].relative, true);
    assert.equal(commands[2].x, 100);
    assert.equal(commands[2].y, 100);
    assert.equal(commands[2].x1, 100);
    assert.equal(commands[2].y1, 100);
    assert.equal(commands[2].x2, 100);
    assert.equal(commands[2].y2, 100);
    assert.equal(commands[3].type, SVGPathData.CURVE_TO);
    assert.equal(commands[3].relative, true);
    assert.equal(commands[3].x, 100);
    assert.equal(commands[3].y, 100);
    assert.equal(commands[3].x1, 100);
    assert.equal(commands[3].y1, 100);
    assert.equal(commands[3].x2, 100);
    assert.equal(commands[3].y2, 100);
  });

  it("should work with S commands", function() {
    var commands = new SVGPathData('S100,100 100,100\
      S200,200 200,200S300,300 300,300S400,400 400,400').toRel().commands;
    assert.equal(commands[0].type, SVGPathData.SMOOTH_CURVE_TO);
    assert.equal(commands[0].relative, true);
    assert.equal(commands[0].x, 100);
    assert.equal(commands[0].y, 100);
    assert.equal(commands[0].x2, 100);
    assert.equal(commands[0].y2, 100);
    assert.equal(commands[1].type, SVGPathData.SMOOTH_CURVE_TO);
    assert.equal(commands[1].relative, true);
    assert.equal(commands[1].x, 100);
    assert.equal(commands[1].y, 100);
    assert.equal(commands[1].x2, 100);
    assert.equal(commands[1].y2, 100);
    assert.equal(commands[2].type, SVGPathData.SMOOTH_CURVE_TO);
    assert.equal(commands[2].relative, true);
    assert.equal(commands[2].x, 100);
    assert.equal(commands[2].y, 100);
    assert.equal(commands[2].x2, 100);
    assert.equal(commands[2].y2, 100);
    assert.equal(commands[3].type, SVGPathData.SMOOTH_CURVE_TO);
    assert.equal(commands[3].relative, true);
    assert.equal(commands[3].x, 100);
    assert.equal(commands[3].y, 100);
    assert.equal(commands[3].x2, 100);
    assert.equal(commands[3].y2, 100);
  });

  it("should work with Q commands", function() {
    var commands = new SVGPathData('Q-100,100 -100,100Q-200,200 -200,200\
      Q-300,300 -300,300Q-400,400 -400,400').toRel().commands;
    assert.equal(commands[0].type, SVGPathData.QUAD_TO);
    assert.equal(commands[0].relative, true);
    assert.equal(commands[0].x, -100);
    assert.equal(commands[0].y, 100);
    assert.equal(commands[0].x1, -100);
    assert.equal(commands[0].y1, 100);
    assert.equal(commands[1].type, SVGPathData.QUAD_TO);
    assert.equal(commands[1].relative, true);
    assert.equal(commands[1].x, -100);
    assert.equal(commands[1].y, 100);
    assert.equal(commands[1].x1, -100);
    assert.equal(commands[1].y1, 100);
    assert.equal(commands[2].type, SVGPathData.QUAD_TO);
    assert.equal(commands[2].relative, true);
    assert.equal(commands[2].x, -100);
    assert.equal(commands[2].y, 100);
    assert.equal(commands[2].x1, -100);
    assert.equal(commands[2].y1, 100);
    assert.equal(commands[3].type, SVGPathData.QUAD_TO);
    assert.equal(commands[3].relative, true);
    assert.equal(commands[3].x, -100);
    assert.equal(commands[3].y, 100);
    assert.equal(commands[3].x1, -100);
    assert.equal(commands[3].y1, 100);
  });

  it("should work with T commands", function() {
    var commands = new SVGPathData('T-100,100T-200,200T-190,210T-180,220').toRel().commands;
    assert.equal(commands[0].type, SVGPathData.SMOOTH_QUAD_TO);
    assert.equal(commands[0].relative, true);
    assert.equal(commands[0].x, -100);
    assert.equal(commands[0].y, 100);
    assert.equal(commands[1].type, SVGPathData.SMOOTH_QUAD_TO);
    assert.equal(commands[1].relative, true);
    assert.equal(commands[1].x, -100);
    assert.equal(commands[1].y, 100);
    assert.equal(commands[2].type, SVGPathData.SMOOTH_QUAD_TO);
    assert.equal(commands[2].relative, true);
    assert.equal(commands[2].x, 10);
    assert.equal(commands[2].y, 10);
    assert.equal(commands[3].type, SVGPathData.SMOOTH_QUAD_TO);
    assert.equal(commands[3].relative, true);
    assert.equal(commands[3].x, 10);
    assert.equal(commands[3].y, 10);
  });

  it("should work with A commands", function() {
    var commands = new SVGPathData('A20,20 180 1 0 -100,100\
      A20,20 180 1 0 -200,200A20,20 180 1 0 -300,300\
      A20,20 180 1 0 -400,400').toRel().commands;
    assert.equal(commands[0].type, SVGPathData.ARC);
    assert.equal(commands[0].relative, true);
    assert.equal(commands[0].rX, 20);
    assert.equal(commands[0].rY, 20);
    assert.equal(commands[0].xRot, 180);
    assert.equal(commands[0].lArcFlag, 1);
    assert.equal(commands[0].sweepFlag, 0);
    assert.equal(commands[0].x, -100);
    assert.equal(commands[0].y, 100);
    assert.equal(commands[1].type, SVGPathData.ARC);
    assert.equal(commands[1].relative, true);
    assert.equal(commands[1].rX, 20);
    assert.equal(commands[1].rY, 20);
    assert.equal(commands[1].xRot, 180);
    assert.equal(commands[1].lArcFlag, 1);
    assert.equal(commands[1].sweepFlag, 0);
    assert.equal(commands[1].x, -100);
    assert.equal(commands[1].y, 100);
    assert.equal(commands[2].type, SVGPathData.ARC);
    assert.equal(commands[2].relative, true);
    assert.equal(commands[2].rX, 20);
    assert.equal(commands[2].rY, 20);
    assert.equal(commands[2].xRot, 180);
    assert.equal(commands[2].lArcFlag, 1);
    assert.equal(commands[2].sweepFlag, 0);
    assert.equal(commands[2].x, -100);
    assert.equal(commands[2].y, 100);
    assert.equal(commands[3].type, SVGPathData.ARC);
    assert.equal(commands[3].relative, true);
    assert.equal(commands[3].rX, 20);
    assert.equal(commands[3].rY, 20);
    assert.equal(commands[3].xRot, 180);
    assert.equal(commands[3].lArcFlag, 1);
    assert.equal(commands[3].sweepFlag, 0);
    assert.equal(commands[3].x, -100);
    assert.equal(commands[3].y, 100);
  });

  it("should work with nested commands", function() {
    var commands = new SVGPathData('A20,20 180 1 0 -100,100H-90V110L20,220\
      C10,10 20,20 20,220').toRel().commands;
    assert.equal(commands[0].type, SVGPathData.ARC);
    assert.equal(commands[0].relative, true);
    assert.equal(commands[0].rX, 20);
    assert.equal(commands[0].rY, 20);
    assert.equal(commands[0].xRot, 180);
    assert.equal(commands[0].lArcFlag, 1);
    assert.equal(commands[0].sweepFlag, 0);
    assert.equal(commands[0].x, -100);
    assert.equal(commands[0].y, 100);
    assert.equal(commands[1].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[1].relative, true);
    assert.equal(commands[1].x, 10);
    assert.equal(commands[2].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[2].relative, true);
    assert.equal(commands[2].y, 10);
    assert.equal(commands[3].type, SVGPathData.LINE_TO);
    assert.equal(commands[3].relative, true);
    assert.equal(commands[3].x, 110);
    assert.equal(commands[3].y, 110);
    assert.equal(commands[4].type, SVGPathData.CURVE_TO);
    assert.equal(commands[4].relative, true);
    assert.equal(commands[4].x, 0);
    assert.equal(commands[4].y, 0);
    assert.equal(commands[4].x1, 0);
    assert.equal(commands[4].y1, -200);
    assert.equal(commands[4].x2, -10);
    assert.equal(commands[4].y2, -210);
  });

});

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

describe("Parsing horizontal commands", function() {

  beforeEach(function() {
  });

  afterEach(function() {
  });

  it("should work with single coordinate", function() {
    var commands = new SVGPathData('H100').commands;
    assert.equal(commands[0].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].x, '100');
  });

  it("should work with single complexer coordinate", function() {
    var commands = new SVGPathData('H-10e-5').commands;
    assert.equal(commands[0].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].x, '-10e-5');
  });

  it("should work with single even more complexer coordinate", function() {
    var commands = new SVGPathData('H-10.0032e-5').commands;
    assert.equal(commands[0].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].x, '-10.0032e-5');
  });

  it("should work with single relative coordinate", function() {
    var commands = new SVGPathData('h100').commands;
    assert.equal(commands[0].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[0].relative, true);
    assert.equal(commands[0].x, '100');
  });

  it("should work with comma separated coordinates", function() {
    var commands = new SVGPathData('H123,456,7890,9876').commands;
    assert.equal(commands[0].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[0].x, '123');
    assert.equal(commands[1].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[1].x, '456');
    assert.equal(commands[2].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[2].x, '7890');
    assert.equal(commands[3].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[3].x, '9876');
  });

  it("should work with space separated coordinates", function() {
    var commands = new SVGPathData('H123 456 7890 9876').commands;
    assert.equal(commands[0].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[0].x, '123');
    assert.equal(commands[1].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[1].x, '456');
    assert.equal(commands[2].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[2].x, '7890');
    assert.equal(commands[3].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[3].x, '9876');
  });

  it("should work with nested separated coordinates", function() {
    var commands = new SVGPathData('H123 ,  456  \t,\n7890 \r\n 9876').commands;
    assert.equal(commands[0].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[0].x, '123');
    assert.equal(commands[1].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[1].x, '456');
    assert.equal(commands[2].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[2].x, '7890');
    assert.equal(commands[3].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[3].x, '9876');
  });

  it("should work with multiple command declarations", function() {
    var commands = new SVGPathData('H123 ,  456  \t,\n7890 \r\n 9876H123 , \
       456  \t,\n7890 \r\n 9876').commands;
    assert.equal(commands[0].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[0].x, '123');
    assert.equal(commands[1].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[1].x, '456');
    assert.equal(commands[2].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[2].x, '7890');
    assert.equal(commands[3].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[3].x, '9876');
    assert.equal(commands[4].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[4].x, '123');
    assert.equal(commands[5].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[5].x, '456');
    assert.equal(commands[6].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[6].x, '7890');
    assert.equal(commands[7].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[7].x, '9876');
  });

});

describe("Parsing vertical commands", function() {

  beforeEach(function() {
  });

  afterEach(function() {
  });

  it("should work with single coordinate", function() {
    var commands = new SVGPathData('V100').commands;
    assert.equal(commands[0].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].y, '100');
  });

  it("should work with single complexer coordinate", function() {
    var commands = new SVGPathData('V-10e-5').commands;
    assert.equal(commands[0].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].y, '-10e-5');
  });

  it("should work with single even more complexer coordinate", function() {
    var commands = new SVGPathData('V-10.0032e-5').commands;
    assert.equal(commands[0].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].y, '-10.0032e-5');
  });

  it("should work with single relative coordinate", function() {
    var commands = new SVGPathData('v100').commands;
    assert.equal(commands[0].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[0].relative, true);
    assert.equal(commands[0].y, '100');
  });

  it("should work with comma separated coordinates", function() {
    var commands = new SVGPathData('V123,456,7890,9876').commands;
    assert.equal(commands[0].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[0].y, '123');
    assert.equal(commands[1].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[1].y, '456');
    assert.equal(commands[2].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[2].y, '7890');
    assert.equal(commands[3].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[3].y, '9876');
  });

  it("should work with space separated coordinates", function() {
    var commands = new SVGPathData('V123 456 7890 9876').commands;
    assert.equal(commands[0].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[0].y, '123');
    assert.equal(commands[1].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[1].y, '456');
    assert.equal(commands[2].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[2].y, '7890');
    assert.equal(commands[3].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[3].y, '9876');
  });

  it("should work with nested separated coordinates", function() {
    var commands = new SVGPathData('V123 ,  456  \t,\n7890 \r\n 9876').commands;
    assert.equal(commands[0].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[0].y, '123');
    assert.equal(commands[1].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[1].y, '456');
    assert.equal(commands[2].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[2].y, '7890');
    assert.equal(commands[3].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[3].y, '9876');
  });

  it("should work with multiple command declarations", function() {
    var commands = new SVGPathData('V123 ,  456  \t,\n7890 \r\n\
     9876V123 ,  456  \t,\n7890 \r\n 9876').commands;
    assert.equal(commands[0].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[0].y, '123');
    assert.equal(commands[1].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[1].y, '456');
    assert.equal(commands[2].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[2].y, '7890');
    assert.equal(commands[3].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[3].y, '9876');
    assert.equal(commands[4].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[4].y, '123');
    assert.equal(commands[5].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[5].y, '456');
    assert.equal(commands[6].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[6].y, '7890');
    assert.equal(commands[7].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[7].y, '9876');
  });

});

describe("Parsing nested vertical/horizontal commands", function() {

  beforeEach(function() {
  });

  afterEach(function() {
  });

  it("should work", function() {
    var commands = new SVGPathData(
      'V100H100v0.12h0.12,V100,h100v-10e-5 H-10e-5').commands;
    assert.equal(commands[0].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[0].relative, false);
    assert.equal(commands[0].y, '100');
    assert.equal(commands[1].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[1].relative, false);
    assert.equal(commands[1].x, '100');
    assert.equal(commands[2].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[2].relative, true);
    assert.equal(commands[2].y, '0.12');
    assert.equal(commands[3].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[3].relative, true);
    assert.equal(commands[3].x, '0.12');
    assert.equal(commands[4].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[4].relative, false);
    assert.equal(commands[4].y, '100');
    assert.equal(commands[5].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[5].relative, true);
    assert.equal(commands[5].x, '100');
    assert.equal(commands[6].type, SVGPathData.VERT_LINE_TO);
    assert.equal(commands[6].relative, true);
    assert.equal(commands[6].y, '-10e-5');
    assert.equal(commands[7].type, SVGPathData.HORIZ_LINE_TO);
    assert.equal(commands[7].relative, false);
    assert.equal(commands[7].x, '-10e-5');
  });

});

describe("Encoding nested vertical/horizontal commands", function() {

  beforeEach(function() {
  });

  afterEach(function() {
  });

  it("should work", function() {
    assert.equal(
      new SVGPathData('V100H100v0.12h0.12V100h100v-10e-5H-10e-5').encode(),
      'V100H100v0.12h0.12V100h100v-0.0001H-0.0001'
    );
  });

});


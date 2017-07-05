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

describe("Parsing close path commands", function() {

  it("should work", function() {
    var commands = new SVGPathData('Z').commands;
    assert.equal(commands[0].type, SVGPathData.CLOSE_PATH);
  });

  it("should work with spaces before", function() {
    var commands = new SVGPathData('   Z').commands;
    assert.equal(commands[0].type, SVGPathData.CLOSE_PATH);
  });

  it("should work with spaces after", function() {
    var commands = new SVGPathData('Z    ').commands;
    assert.equal(commands[0].type, SVGPathData.CLOSE_PATH);
  });

  it("should work before a command sequence", function() {
    var commands = new SVGPathData(' Z M10,10 L10,10, H10, V10').commands;
    assert.equal(commands[0].type, SVGPathData.CLOSE_PATH);
  });

  it("should work after a command sequence", function() {
    var commands = new SVGPathData('M10,10 L10,10, H10, V10 Z').commands;
    assert.equal(commands[4].type, SVGPathData.CLOSE_PATH);
  });

  it("should work in a command sequence", function() {
    var commands = new SVGPathData('M10,10 L10,10, H10, V10 Z M10,10 L10,10, H10, V10').commands;
    assert.equal(commands[4].type, SVGPathData.CLOSE_PATH);
  });

});

describe("Encoding close path commands", function() {

  it("should work with one command", function() {
      assert.equal(
        new SVGPathData('z').encode(),
        'z'
      );
  });

  it("should work with several commands", function() {
      assert.equal(
        new SVGPathData('zzzz').encode(),
        'zzzz'
      );
  });

});

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

describe("SVGPathDataEncoder", function() {

  it("should still work when the new operator is forgotten", function() {
    assert.doesNotThrow(function() {
      SVGPathData.Encoder();
    });
  });

  it("should fail when a bad command is given", function() {
    assert.throws(function() {
      var encoder = new SVGPathData.Encoder();
      encoder.write({
        type: 'plop',
        x: 0,
        y: 0
      });
    }, 'Unexpected command type "plop" at index 0.');
  });

});


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

describe("SVGPathDataTransformer", function() {
  it("should fail with bad args", function() {
    assert.throws(function() {
      new SVGPathData.Transformer();
    }, 'Please provide a transform callback to receive commands.');
  });

  it("should fail with bad transform function", function() {
    assert.throws(function() {
      new SVGPathData.Transformer(function() {});
    }, 'Please provide a valid transform (returning a function).');
  });

  it("should still work when the new operator is forgotten", function() {
    assert.doesNotThrow(function() {
      SVGPathData.Transformer(SVGPathData.Transformer.SCALE, 1, 1);
    });
  });

  it("should work in streaming mode", function() {
      var encoder = new SVGPathData.Transformer(SVGPathData.Transformer.SCALE, 1, 1);
      encoder.write({
        type: SVGPathData.Parser.LINE_TO,
        relative: true,
        x: 10,
        y: 10
      });
      encoder.end();
  });

});


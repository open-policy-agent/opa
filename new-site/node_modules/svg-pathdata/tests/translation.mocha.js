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

describe("Possitive translation", function() {

  it("should fail with no args", function() {
    assert.throws(function() {
      new SVGPathData(
        'm20,30l10,10z'
      ).translate().encode();
    }, 'A translate transformation requires the parameter dX'
      +' to be set and to be a number.');
  });

  it("should work with relative path", function() {
    assert.equal(new SVGPathData(
      'm20,30c0 0 10 20 15 30s10 20 15 30q10 20 15 30t10 10l10 10h10v10a10 10 5 1 0 10 10z'
    ).translate(10, 10).encode(),
      'm30 40c0 0 10 20 15 30s10 20 15 30q10 20 15 30t10 10l10 10h10v10a10 10 5 1 0 10 10z');
  });

  it("should work with absolute path", function() {
    assert.equal(new SVGPathData(
      'M20,30C0 0 10 20 15 30S10 20 15 30Q10 20 15 30T10 10L10 10H10V10A10 10 5 1 0 10 10z'
    ).translate(10, 10).encode(),
      'M30 40C10 10 20 30 25 40S20 30 25 40Q20 30 25 40T20 20L20 20H20V20A10 10 5 1 0 20 20z');
  });

});

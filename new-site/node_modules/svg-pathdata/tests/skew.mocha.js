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

describe("X axis skew", function() {
  it("should fail with bad args", function() {
    assert.throws(function() {
      new SVGPathData(
        'm20,30l10,10z'
      ).skewX().encode();
    }, 'A skewX transformation requires the parameter x'
      +' to be set and to be a number.');
  });

  it("should work with relative path", function() {
    assert.equal(new SVGPathData(
      'm100 75l-50 -45l0 90z'
    ).skewX(Math.PI/2).encode(),
      'm175.29136163904155 75l-95.17481698342493 -45l90.34963396684985 90z');
  });

  it("should work with absolute path", function() {
    assert.equal(new SVGPathData(
      'M 100,75 50,30 50,120 z'
    ).skewX(Math.PI/2).encode(),
      'M175.29136163904155 75L80.11654465561662 30L170.46617862246646 120z');
  });

});

describe("Y axis skew", function() {
  it("should fail with bad args", function() {
    assert.throws(function() {
      new SVGPathData(
        'm20,30l10,10z'
      ).skewY().encode();
    }, 'A skewY transformation requires the parameter y'
      +' to be set and to be a number.');
  });

  it("should work with relative path", function() {
    assert.equal(new SVGPathData(
      'm100 75l-50 -45l0 90z'
    ).skewY(Math.PI/2).encode(),
      'm100 175.3884821853887l-50 -95.19424109269436l0 90z');
  });

  it("should work with absolute path", function() {
    assert.equal(new SVGPathData(
      'M 100,75 50,30 50,120 z'
    ).skewY(Math.PI/2).encode(),
      'M100 175.3884821853887L50 80.19424109269436L50 170.19424109269437z');
  });

});

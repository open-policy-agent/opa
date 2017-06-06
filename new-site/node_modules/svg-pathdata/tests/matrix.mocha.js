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

describe("Matrix transformation should be the same than it's equivalent transformation", function() {

  it("should fail with bad args", function() {
    assert.throws(function() {
      new SVGPathData(
        'm20,30l10,10z'
      ).matrix().encode();
    }, 'A matrix transformation requires parameters [a,b,c,d,e,f]'
      +' to be set and to be numbers.');
    assert.throws(function() {
      new SVGPathData(
        'm20,30l10,10z'
      ).matrix(1).encode();
    }, 'A matrix transformation requires parameters [a,b,c,d,e,f]'
      +' to be set and to be numbers.');
    assert.throws(function() {
      new SVGPathData(
        'm20,30l10,10z'
      ).matrix(1, 1).encode();
    }, 'A matrix transformation requires parameters [a,b,c,d,e,f]'
      +' to be set and to be numbers.');
    assert.throws(function() {
      new SVGPathData(
        'm20,30l10,10z'
      ).matrix(1, 1, 1).encode();
    }, 'A matrix transformation requires parameters [a,b,c,d,e,f]'
      +' to be set and to be numbers.');
    assert.throws(function() {
      new SVGPathData(
        'm20,30l10,10z'
      ).matrix(1, 1, 1, 1).encode();
    }, 'A matrix transformation requires parameters [a,b,c,d,e,f]'
      +' to be set and to be numbers.');
    assert.throws(function() {
      new SVGPathData(
        'm20,30l10,10z'
      ).matrix(1, 1, 1, 1, 1).encode();
    }, 'A matrix transformation requires parameters [a,b,c,d,e,f]'
      +' to be set and to be numbers.');
  });

  it("for scale", function() {
    assert.equal(
      new SVGPathData('m20 30c0 0 10 20 15 30z').scale(10, 10).encode(),
      new SVGPathData('m20 30c0 0 10 20 15 30z').matrix(10, 0, 0, 10, 0, 0).encode()
    );
  });

});


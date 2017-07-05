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

// Sample pathes from MDN
// https://developer.mozilla.org/en-US/docs/Web/SVG/Tutorial/Paths
// Here we have to round output before testing since there is some lil
// differences across browsers.

describe("Converting eliptical arc commands to curves", function() {

  it("should work sweepFlag on 0 and largeArcFlag on 0", function() {
      assert.equal(
        new SVGPathData('M80 80 A 45 45, 0, 0, 0, 125 125 L 125 80 Z').aToC()
          .round().encode(),
        'M80 80C80 104.8528137423857 100.1471862576143 125 125 125L125 80z'
      );
  });

  it("should work sweepFlag on 1 and largeArcFlag on 0", function() {
      assert.equal(
        new SVGPathData('M230 80 A 45 45, 0, 1, 0, 275 125 L 275 80 Z').aToC()
          .round().encode(),
        'M230 80C195.3589838486225 80 173.7083487540115 117.5 191.0288568297003 147.5C208.349364905389 177.5 251.650635094611 177.5 268.9711431702998 147.5C272.9207180979216 140.6591355570587 275 132.8991498552438 275 125L275 80z'
      );
  });

  it("should work sweepFlag on 0 and largeArcFlag on 1", function() {
      assert.equal(
        new SVGPathData('M80 230 A 45 45, 0, 0, 1, 125 275 L 125 230 Z').aToC()
          .round().encode(),
        'M80 230C104.8528137423857 230 125 250.1471862576143 125 275L125 230z'
      );
  });

  it("should work sweepFlag on 1 and largeArcFlag on 1", function() {
      assert.equal(
        new SVGPathData('M230 230 A 45 45, 0, 1, 1, 275 275 L 275 230 Z').aToC()
          .round().encode(),
        'M230 230C230 195.3589838486225 267.5 173.7083487540115 297.5 191.0288568297003C327.5 208.349364905389 327.5 251.650635094611 297.5 268.9711431702998C290.6591355570588 272.9207180979216 282.8991498552438 275 275 275L275 230z'
      );
  });

});


var assert = require('assert')
  , svgicons2svgfont = require(__dirname + '/../src/index.js')
  , Fs = require('fs')
  , StringDecoder = require('string_decoder').StringDecoder
  , Path = require("path");

// Helpers
function generateFontToFile(options, done, fileSuffix) {
  var codepoint = 0xE001
    , dest = __dirname + '/results/' + options.fontName
      + (fileSuffix || '') + '.svg'
    , stream = svgicons2svgfont(Fs.readdirSync(__dirname + '/fixtures/' + options.fontName)
      .map(function(file) {
        var matches = file.match(/^(?:u([0-9a-f]{4})\-)?(.*).svg$/i);
        return {
          codepoint: (matches[1] ? parseInt(matches[1], 16) : codepoint++),
          name: matches[2],
          stream: Fs.createReadStream(__dirname + '/fixtures/' + options.fontName + '/' + file)
        };
      }), options);
  stream.pipe(Fs.createWriteStream(dest)).on('finish', function() {
    assert.equal(
      Fs.readFileSync(__dirname + '/expected/' + options.fontName
        + (fileSuffix || '') + '.svg',
        {encoding: 'utf8'}),
      Fs.readFileSync(dest,
        {encoding: 'utf8'})
    );
    done();
  });
}

function generateFontToMemory(options, done) {
  var content = ''
    , decoder = new StringDecoder('utf8')
    , codepoint = 0xE001
    , stream = svgicons2svgfont(Fs.readdirSync(__dirname + '/fixtures/' + options.fontName)
      .map(function(file) {
        var matches = file.match(/^(?:u([0-9a-f]{4})\-)?(.*).svg$/i);
        return {
          codepoint: (matches[1] ? parseInt(matches[1], 16) : codepoint++),
          name: matches[2],
          stream: Fs.createReadStream(__dirname + '/fixtures/' + options.fontName + '/' + file)
        };
      }), options);
  stream.on('data', function(chunk) {
    content += decoder.write(chunk);
  });
  stream.on('finish', function() {
    assert.equal(
      Fs.readFileSync(__dirname + '/expected/' + options.fontName + '.svg',
        {encoding: 'utf8'}),
      content
    );
    done();
  });
}

// Tests
describe('Generating fonts to files', function() {

	it("should work for simple SVG", function(done) {
    generateFontToFile({
      fontName: 'originalicons'
    }, done);
	});

	it("should work for simple fixedWidth and normalize option", function(done) {
    generateFontToFile({
      fontName: 'originalicons',
      fixedWidth: true,
      normalize: true
    }, done, 'n');
	});

	it("should work for simple SVG", function(done) {
    generateFontToFile({
      fontName: 'cleanicons'
    }, done);
	});

	it("should work for codepoint mapped SVG icons", function(done) {
    generateFontToFile({
      fontName: 'prefixedicons',
      callback: function(){}
    }, done);
	});

	it("should work with multipath SVG icons", function(done) {
    generateFontToFile({
      fontName: 'multipathicons'
    }, done);
	});

	it("should work with simple shapes SVG icons", function(done) {
    generateFontToFile({
      fontName: 'shapeicons'
    }, done);
	});

	it("should work with variable height icons", function(done) {
    generateFontToFile({
      fontName: 'variableheighticons'
    }, done);
	});

	it("should work with variable height icons and the normalize option", function(done) {
    generateFontToFile({
      fontName: 'variableheighticons',
      normalize: true
    }, done, 'n');
	});

	it("should work with variable width icons", function(done) {
    generateFontToFile({
      fontName: 'variablewidthicons'
    }, done);
	});

	it("should work with centered variable width icons and the fixed width option", function(done) {
    generateFontToFile({
      fontName: 'variablewidthicons',
      fixedWidth: true,
      centerHorizontally: true
    }, done, 'n');
	});

	it("should not display hidden pathes", function(done) {
    generateFontToFile({
      fontName: 'hiddenpathesicons'
    }, done);
	});

	it("should work with real world icons", function(done) {
    generateFontToFile({
      fontName: 'realicons'
    }, done);
	});

  it("should work with rendering test SVG icons", function(done) {
    generateFontToFile({
      fontName: 'rendricons'
    }, done);
  });

  it("should work with a single SVG icon", function(done) {
    generateFontToFile({
      fontName: 'singleicon'
    }, done);
  });

  it("should work with transformed SVG icons", function(done) {
    generateFontToFile({
      fontName: 'transformedicons'
    }, done);
  });

  it("should work when horizontally centering SVG icons", function(done) {
    generateFontToFile({
      fontName: 'tocentericons',
      centerHorizontally: true
    }, done);
  });

  it("should work with a icons with path with fill none", function(done) {
    generateFontToFile({
      fontName: 'pathfillnone'
    }, done);
  });

  it("should work with shapes with rounded corners", function(done) {
    generateFontToFile({
      fontName: 'roundedcorners'
    }, done);
  });

});

describe('Generating fonts to memory', function() {

  it("should work for simple SVG", function(done) {
    generateFontToMemory({
      fontName: 'originalicons'
    }, done);
  });

  it("should work for simple SVG", function(done) {
    generateFontToMemory({
      fontName: 'cleanicons'
    }, done);
  });

  it("should work for codepoint mapped SVG icons", function(done) {
    generateFontToMemory({
      fontName: 'prefixedicons'
    }, done);
  });

  it("should work with multipath SVG icons", function(done) {
    generateFontToMemory({
      fontName: 'multipathicons'
    }, done);
  });

  it("should work with simple shapes SVG icons", function(done) {
    generateFontToMemory({
      fontName: 'shapeicons'
    }, done);
  });

});

describe('Using options', function() {

  it("should work with fixedWidth option set to true", function(done) {
    generateFontToFile({
      fontName: 'originalicons',
      fixedWidth: true
    }, done, '2');
  });

  it("should work with custom fontHeight option", function(done) {
    generateFontToFile({
      fontName: 'originalicons',
      fontHeight: 800
    }, done, '3');
  });

  it("should work with custom descent option", function(done) {
    generateFontToFile({
      fontName: 'originalicons',
      descent: 200
    }, done, '4');
  });

  it("should work with fixedWidth set to true and with custom fontHeight option", function(done) {
    generateFontToFile({
      fontName: 'originalicons',
      fontHeight: 800,
      fixedWidth: true
    }, done, '5');
  });

  it("should work with fixedWidth and centerHorizontally set to true and with custom fontHeight option", function(done) {
    generateFontToFile({
      fontName: 'originalicons',
      fontHeight: 800,
      fixedWidth: true,
      centerHorizontally: true
    }, done, '6');
  });

  it("should work with fixedWidth, normalize and centerHorizontally set to true and with custom fontHeight option", function(done) {
    generateFontToFile({
      fontName: 'originalicons',
      fontHeight: 800,
      normalize: true,
      fixedWidth: true,
      centerHorizontally: true
    }, done, '7');
  });

});

describe('Testing CLI', function() {

  it("should work for simple SVG", function(done) {
    (require('child_process').exec)(
      'node '+__dirname+'../bin/svgicons2svgfont.js '
      + __dirname + '/expected/originalicons.svg '
      + __dirname + '/results/originalicons.svg',
      function() {
        assert.equal(
          Fs.readFileSync(__dirname + '/expected/originalicons.svg',
            {encoding: 'utf8'}),
          Fs.readFileSync(__dirname + '/results/originalicons.svg',
            {encoding: 'utf8'})
        );
        done();
      }
    );
  });

});

describe('Providing bad glyphs', function() {

	it("should fail when not providing glyph name", function() {
	  var hadError = false;
    try {
      svgicons2svgfont([{
	      stream: Fs.createReadStream('/dev/null'),
	      codepoint: 0xE001
      }]);
    } catch(err) {
	    assert.equal(err instanceof Error, true);
	    assert.equal(err.message, 'Please provide a name for the glyph at index 0');
	    hadError = true;
    }
    assert.equal(hadError, true);
	});

	it("should fail when not providing codepoints", function() {
	  var hadError = false;
    try {
      svgicons2svgfont([{
	      stream: Fs.createReadStream('/dev/null'),
	      name: 'test'
      }]);
    } catch(err) {
	    assert.equal(err instanceof Error, true);
	    assert.equal(err.message, 'Please provide a codepoint for the glyph "test"');
	    hadError = true;
    }
    assert.equal(hadError, true);
	});

	it("should fail when providing the same codepoint twice", function() {
	  var hadError = false;
    try {
      svgicons2svgfont([{
	      stream: Fs.createReadStream('/dev/null'),
	      name: 'test',
	      codepoint: 0xE001
      },{
	      stream: Fs.createReadStream('/dev/null'),
	      name: 'test2',
	      codepoint: 0xE001
      }]);
    } catch(err) {
	    assert.equal(err instanceof Error, true);
	    assert.equal(err.message, 'The glyph "test" codepoint seems to be used already elsewhere.');
	    hadError = true;
    }
    assert.equal(hadError, true);
	});

	it("should fail when providing the same name twice", function() {
	  var hadError = false;
    try {
      svgicons2svgfont([{
	      stream: Fs.createReadStream('/dev/null'),
	      name: 'test',
	      codepoint: 0xE001
      },{
	      stream: Fs.createReadStream('/dev/null'),
	      name: 'test',
	      codepoint: 0xE002
      }]);
    } catch(err) {
	    assert.equal(err instanceof Error, true);
	    assert.equal(err.message, 'The glyph name "test" must be unique.');
	    hadError = true;
    }
    assert.equal(hadError, true);
	});

});

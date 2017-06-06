var gutil = require('gulp-util')
  , duplexer = require('plexer')
  , svgicons2svgfont = require('gulp-svgicons2svgfont')
  , svg2ttf = require('gulp-svg2ttf')
  , ttf2eot = require('gulp-ttf2eot')
  , ttf2woff = require('gulp-ttf2woff')
;

function gulpFontIcon(options) {
  // Generating SVG font and saving her
  var inStream = svgicons2svgfont(options);
  // Generating TTF font and saving her
  var outStream = inStream.pipe(svg2ttf({clone: true}))
  // Generating EOT font
    .pipe(ttf2eot({clone: true}))
  // Generating WOFF font
    .pipe(ttf2woff({clone: true}));

  var duplex = duplexer({objectMode: true}, inStream, outStream);

  // Re-emit codepoint mapping event
  inStream.on('codepoints', function(codepoints) {
    duplex.emit('codepoints', codepoints, options);
  });

  // Re-emit glyph mapping event
  inStream.on('glyph', function(glyph) {
    duplex.emit('glyph', glyph, options);
  });

  return duplex;
}

module.exports = gulpFontIcon;

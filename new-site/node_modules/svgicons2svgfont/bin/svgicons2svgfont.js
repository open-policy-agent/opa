#! /usr/bin/env node

var svgicons2svgfont = require(__dirname + '/../src/index.js')
  , Fs = require('fs')
  , codepoint = 0xE001
;

svgicons2svgfont(
  Fs.readdirSync(process.argv[2]).map(function(file) {
    return {
      name: 'glyph' + codepoint,
      codepoint: codepoint++,
      stream: Fs.createReadStream(
        process.argv[2] + '/' + file
      )
    };
  }), {
    fontName: process.argv[4] || ''
  }
).pipe(Fs.createWriteStream(process.argv[3]));


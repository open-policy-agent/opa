#!/usr/bin/env node
/*
 Author: Sergey Batishchev <sergej.batishchev@gmail.com>

 Written for fontello.com project.
 */

'use strict';


var fs = require('fs');
var ArgumentParser = require('argparse').ArgumentParser;

var svg2ttf = require('./index.js');


var parser = new ArgumentParser({
  version: require('./package.json').version,
  addHelp: true,
  description: 'SVG to TTF font converter'
});

parser.addArgument(
  [ '-c', '--copyright' ],
  {
    help: 'Copyright text',
    required: false
  }
);

parser.addArgument(
  [ 'infile' ],
  {
    nargs: 1,
    help: 'Input file'
  }
);

parser.addArgument(
  [ 'outfile' ],
  {
    nargs: 1,
    help: 'Output file'
  }
);


var args = parser.parseArgs();
var svg;
var options = {};

try {
  svg = fs.readFileSync(args.infile[0], 'utf-8');
} catch(e) {
  console.error("Can't open input file (%s)", args.infile[0]);
  process.exit(1);
}

if (args.copyright) {
  options.copyright = args.copyright;
}

fs.writeFileSync(args.outfile[0], new Buffer(svg2ttf(svg, options).buffer));

#!/usr/bin/env node
/*
    Author: Viktor Semykin <thesame.ml@gmail.com>

    Written for fontello.com project.
*/

'use strict';

var fs = require('fs');
var ArgumentParser = require('argparse').ArgumentParser;

var ttf2eot = require('./index.js');


var parser = new ArgumentParser ({
  version: require('./package.json').version,
  addHelp: true,
  description: 'TTF to EOT font converter'
});

parser.addArgument (
  [ 'infile' ],
  {
    nargs: '?',
    help: 'Input file (stdin if not defined)'
  }
);

parser.addArgument (
  [ 'outfile' ],
  {
    nargs: '?',
    help: 'Output file (stdout if not defined)'
  }
);

/* eslint-disable no-console */

var args = parser.parseArgs();

var input, size;

try {
  if (args.infile) {
    input = fs.readFileSync(args.infile);
  } else {
    size = fs.fstatSync(process.stdin.fd).size;
    input = new Buffer(size);
    fs.readSync(process.stdin.fd, input, 0, size, 0);
  }
} catch (e) {
  console.error("Can't open input file (%s)", args.infile || 'stdin');
  process.exit(1);
}

var ttf = new Uint8Array(input);
var eot = new Buffer(ttf2eot(ttf).buffer);

if (args.outfile) {
  fs.writeFileSync(args.outfile, eot);
} else {
  process.stdout.write(eot);
}


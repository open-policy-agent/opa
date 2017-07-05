# SVGPathData
> Manipulating SVG path datas (path[d] attribute content) simply and efficiently.

[![NPM version](https://badge.fury.io/js/svg-pathdata.png)](https://npmjs.org/package/svg-pathdata) [![Build status](https://secure.travis-ci.org/nfroidure/SVGPathData.png)](https://travis-ci.org/nfroidure/SVGPathData) [![Dependency Status](https://david-dm.org/nfroidure/SVGPathData.png)](https://david-dm.org/nfroidure/SVGPathData) [![devDependency Status](https://david-dm.org/nfroidure/SVGPathData/dev-status.png)](https://david-dm.org/nfroidure/SVGPathData#info=devDependencies) [![Coverage Status](https://coveralls.io/repos/nfroidure/SVGPathData/badge.png?branch=master)](https://coveralls.io/r/nfroidure/SVGPathData?branch=master)

## Including the library
This library is fully node based (based on current stream implementation) but
 you can also use it in modern browsers with the
 [browserified build](https://github.com/nfroidure/SVGPathData/blob/master/dist/SVGPathData.js)
 or in your own build using Browserify.

## Reading PathDatas
```js
var pathData = new SVGPathData ('\
  M 10 10 \
  H 60 \
  V 60 \
  L 10 60 \
  Z \
');

console.log(pathData.commands);

// {"commands":[{
//    "type": SVGPathData.MOVE_TO,
//    "relative": false,
//    "x": 10, "y": 10
//  },{
//    "type": SVGPathData.HORIZ_LINE_TO,
//    "relative": false,
//    "x": 60
//  },{
//    "type": SVGPathData.VERT_LINE_TO,
//    "relative":false,
//    "y": 60
//  },{
//    "type": SVGPathData.LINE_TO,
//    "relative": false,
//    "x": 10,
//    "y": 60
//  },{
//    "type": SVGPathData.CLOSE_PATH
//  }
// ]}
```

## Reading streamed PathDatas
```js
var parser = new SVGPathData.Parser();
parser.on('data', function(cmd) {
  console.log(cmd);
});

parser.write('   ');
parser.write('M 10');
parser.write(' 10');

// {
//   "type": SVGPathData.MOVE_TO,
//   "relative": false,
//   "x": 10, "y": 10
// }


parser.write('H 60');

// {
//   "type": SVGPathData.HORIZ_LINE_TO,
//   "relative": false,
//   "x": 60
// }


parser.write('V');
parser.write('60');

// {
//   "type": SVGPathData.VERT_LINE_TO,
//   "relative": false,
//   "y": 60
// }


parser.write('L 10 60 \
  Z');

// {
//   "type": SVGPathData.LINE_TO,
//   "relative": false,
//   "x": 10,
//   "y": 60
// }
  
// {
//   "type": SVGPathData.CLOSE_PATH
// }

parser.end();
```

## Outputting PathDatas
```js
var pathData = new SVGPathData ('\
  M 10 10 \
  H 60 \
  V 60 \
  L 10 60 \
  Z \
');

console.log(pathData.encode());
// "M10 10H60V60L10 60Z"
```

## Streaming PathDatas out
```js
var encoder = new SVGPathData.Encoder();
encoder.setEncoding('utf8');

encode.on('data', function(str) {
  console.log(str);
});

encoder.write({
   "type": SVGPathData.MOVE_TO,
   "relative": false,
   "x": 10, "y": 10
 });
// "M10 10"

encoder.write({
   "type": SVGPathData.HORIZ_LINE_TO,
   "relative": false,
   "x": 60
});
// "H60"

encoder.write({
   "type": SVGPathData.VERT_LINE_TO,
   "relative": false,
   "y": 60
});
// "V60"

encoder.write({
   "type": SVGPathData.LINE_TO,
   "relative": false,
   "x": 10,
   "y": 60
});
// "L10 60"
  
encoder.write({"type": SVGPathData.CLOSE_PATH});
// "Z"

encode.end();
```

## Transforming PathDatas
This library was made to live decoding/transform/encoding SVG PathDatas. Here is
 [an example of that kind of use](https://github.com/nfroidure/svgicons2svgfont/blob/aa6df0211419e9d61c417c63bcc353f0cb2ea0c8/src/index.js#L192).

### The synchronous way
```js
console.log(
  new SVGPathData ('\
   m 10,10 \
   h 60 \
   v 60 \
   l 10,60 \
   z'
  )
  .toAbs()
  .encode()
);
// "M10,10 H70 V70 L80,130 Z"
```

### The streaming/asynchronous way
Here, we take SVGPathDatas from stdin and output it transformed to stdout.
```js
// stdin to parser
process.stdin.pipe(new SVGPathData.Parser())
// parser to transformer to absolute
  .pipe(new SVGPathData.Transformer(SVGPathData.Transformer.TO_ABS))
// transformer to encoder
  .pipe(new SVGPathData.Encoder())
// encoder to stdout
  .pipe(process.stdout);
```

## Supported transformations
You can find every supported transformations in
 [this file](https://github.com/nfroidure/SVGPathData/blob/master/src/SVGPathDataTransformer.js#L47)
 of course, you can create yours by using this format:
```js
function SET_X_TO(xValue) {
  xValue = xValue || 10; // Provide default values or throw errors for options
  function(command) {
    command.x = xValue; // transform command objects and return them
    return command;
  };
};

// Synchronous usage
new SVGPathData('...')
  .transform(SET_X_TO, 25)
  .encode();

// Streaming usage
process.stdin.pipe(new SVGPathData.Parser())
  .pipe(new SVGPathData.Transformer(SET_X_TO, 25))
  .pipe(new SVGPathData.Encoder())
  .pipe(process.stdout);
```

## Stats

[![NPM](https://nodei.co/npm/svg-pathdata.png?downloads=true&stars=true)](https://nodei.co/npm/svg-pathdata/)
[![NPM](https://nodei.co/npm-dl/svg-pathdata.png)](https://nodei.co/npm/svg-pathdata/)

## Contributing
Clone this project, run:
```sh
npm install; grunt test&
```

node-temp -- temporary files and directories for node.js
========================================================

Handles generating a unique file/directory name under the appropriate
system temporary directory, changing the file to an appropriate mode,
and supports automatic removal.

`temp` has a similar API to the `fs` module.

Synopsis
--------

You can create temporary files with `open` and `openSync`, temporary
directories with `mkdir` and `mkdirSync`, or you can get a unique name
in the system temporary directory with `path`.

Working copies of the following examples can be found under the
`examples` directory.

### Temporary Files

To create a temporary file use `open` or `openSync`, passing
them an optional prefix, suffix, or both (see below for details on
affixes). The object passed to the callback (or returned) has
`path` and `fd` keys:

```javascript
{ path: "/path/to/file",
, fd: theFileDescriptor
}
```

In this example we write to a temporary file and call out to `grep` and
`wc -l` to determine the number of time `foo` occurs in the text.  The
temporary file is chmod'd `0600` and cleaned up automatically when the
process at exit:

```javascript
var temp = require('temp'),
    fs   = require('fs'),
    util  = require('util'),
    exec = require('child_process').exec;

// Fake data
var myData = "foo\nbar\nfoo\nbaz";

// Process the data (note: error handling omitted)
temp.open('myprefix', function(err, info) {
  fs.write(info.fd, myData);
  fs.close(info.fd, function(err) {
    exec("grep foo '" + info.path + "' | wc -l", function(err, stdout) {
      util.puts(stdout.trim());
    });
  });
});
```

### Temporary Directories

To create a temporary directory, use `mkdir` or `mkdirSync`, passing
it an optional prefix, suffix, or both (see below for details on affixes).

In this example we create a temporary directory, write to a file
within it, call out to an external program to create a PDF, and read
the result.  While the external process creates a lot of additional
files, the temporary directory is removed automatically at exit:

```javascript
var temp = require('../lib/temp'),
    fs   = require('fs'),
    util = require('util'),
    path = require('path'),
    exec = require('child_process').exec;

// For use with ConTeXt, http://wiki.contextgarden.net
var myData = "\\starttext\nHello World\n\\stoptext";

temp.mkdir('pdfcreator', function(err, dirPath) {
  var inputPath = path.join(dirPath, 'input.tex')
  fs.writeFile(inputPath, myData, function(err) {
    if (err) throw err;
    process.chdir(dirPath);
    exec("texexec '" + inputPath + "'", function(err) {
      if (err) throw err;
      fs.readFile(path.join(dirPath, 'input.pdf'), function(err, data) {
        if (err) throw err;
        sys.print(data);
      });
    });
  });
});
```

### Temporary Streams

To create a temporary WriteStream, use 'createWriteStream', which sits
on top of `fs.createWriteStream`. The return value is a
`fs.WriteStream` whose `path` is registered for removal when
`temp.cleanup` is called.

```javascript
var temp = require('temp');

var stream = temp.createWriteStream();
stream.write("Some data");
// Maybe do some other things
stream.end();
```

### Affixes

You can provide custom prefixes and suffixes when creating temporary
files and directories. If you provide a string, it is used as the prefix
for the temporary name. If you provide an object with `prefix` and
`suffix` keys, they are used for the temporary name.

Here are some examples:

* `"aprefix"`: A simple prefix, prepended to the filename; this is
  shorthand for:
* `{prefix: "aprefix"}`: A simple prefix, prepended to the filename
* `{suffix: ".asuffix"}`: A suffix, appended to the filename
  (especially useful when the file needs to be named with specific
  extension for use with an external program).
* `{prefix: "myprefix", suffix: "mysuffix"}`: Customize both affixes
* `null`: Use the defaults for files and directories (prefixes `"f-"`
  and `"d-"`, respectively, no suffixes).

In this simple example we read a `pdf`, write it to a temporary file with
a `.pdf` extension, and close it.

```javascript
var fs   = require('fs'),
    temp = require('temp');

fs.readFile('/path/to/source.pdf', function(err, data) {
  temp.open({suffix: '.pdf'}, function(err, info) {
    if (err) throw err;
    fs.write(info.fd, contents);
    fs.close(info.fd, function(err) {
      if (err) throw err;
      // Do something with the file
    });
  });
});
```

### Just a path, please

If you just want a unique name in your temporary directory, use
`path`:

```javascript
var fs = require('fs');
var tempName = temp.path({suffix: '.pdf'});
// Do something with tempName
```
    
Note: The file isn't created for you, and the  mode is not changed  -- and it
will not be removed automatically at exit.  If you use `path`, it's
all up to you.

Compatibility
-------------

Works with Node.js v0.6.0 on OS X.  Please let me know if you have
problems running it on a later version of Node.js or
have platform problems.

Installation
------------

Install it using [npm](http://github.com/isaacs/npm):

    $ npm install temp

Or get it directly from:
http://github.com/bruce/node-temp

Testing
-------

For now, run `test/test-temp.js`:

    $ node test/test-temp.js

Contributing
------------

You can find the repository at:
http://github.com/bruce/node-temp

Issues/Feature Requests can be submitted at:
http://github.com/bruce/node-temp/issues

I'd really like to hear your feedback, and I'd love to receive your
pull-requests!

Copyright
---------

Copyright (c) 2010-2012 Bruce Williams. See LICENSE for details.

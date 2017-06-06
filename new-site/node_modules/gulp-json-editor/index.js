/* jshint node: true */

var jsbeautify   = require('js-beautify').js_beautify;
var merge        = require('deepmerge');
var through      = require('through2');
var PluginError  = require('gulp-util').PluginError;
var detectIndent = require('detect-indent');

module.exports = function (editor, jsbeautifyOptions) {

  /*
   create 'editBy' function from 'editor'
   */
  var editBy;
  if (typeof editor === 'function') {
    // edit JSON object by user specific function
    editBy = function(json) { return editor(json); };
  }
  else if (typeof editor === 'object') {
    // edit JSON object by merging with user specific object
    editBy = function(json) { return merge(json, editor); };
  }
  else if (typeof editor === 'undefined') {
    throw new PluginError('gulp-json-editor', 'missing "editor" option');
  }
  else {
    throw new PluginError('gulp-json-editor', '"editor" option must be a function or object');
  }

  /*
   js-beautify option
   */
  jsbeautifyOptions = jsbeautifyOptions || {};

  // always beautify output
  var beautify = true;

  /*
   create through object and return it
   */
  return through.obj(function (file, encoding, callback) {

    // ignore it
    if (file.isNull()) {
      this.push(file);
      return callback();
    }

    // stream is not supported
    if (file.isStream()) {
      this.emit('error', new PluginError('gulp-json-editor', 'Streaming is not supported'));
      return callback();
    }

    try {
      // try to get current indentation
      var indent = detectIndent(file.contents.toString('utf8'));

      // beautify options for this particular file
      var beautifyOptions = merge({}, jsbeautifyOptions); // make copy
      beautifyOptions.indent_size = beautifyOptions.indent_size || indent.amount || 2;
      beautifyOptions.indent_char = beautifyOptions.indent_char || (indent.type === 'tab' ? '\t' : ' ');

      // edit JSON object and get it as string notation
      var json = JSON.stringify(editBy(JSON.parse(file.contents.toString('utf8'))), null, indent.indent);

      // beautify JSON
      if (beautify) {
        json = jsbeautify(json, beautifyOptions);
      }

      // write it to file
      file.contents = new Buffer(json);
    }
    catch (err) {
      this.emit('error', new PluginError('gulp-json-editor', err));
    }

    this.push(file);
    callback();

  });

};

'use strict';
var rimraf = require('rimraf');
var through2 = require('through2');
var gutil = require('gulp-util');
var path = require('path');

module.exports = function (options) {
  return through2.obj(function (file, enc, cb) {
    // Paths are resolved by gulp
    var filepath = file.path;
    var cwd = file.cwd;
    var relative = path.relative(cwd, filepath);

    // Prevent mistakes with paths
    if (!(relative.substr(0, 2) === '..') && relative !== '' || (options ? (options.force && typeof options.force === 'boolean') : false)) {
      rimraf(filepath, function (error) {
        if (error) {
          this.emit('error', new gutil.PluginError('gulp-clean', 'Unable to delete "' + filepath + '" file (' + error.message + ').'));
        }
        this.push(file);
        cb();
      }.bind(this));
    } else if (relative === '') {
      var msgCurrent = 'Cannot delete current working directory. (' + filepath + '). Use option force.';
      gutil.log('gulp-clean: ' + msgCurrent);
      this.emit('error', new gutil.PluginError('gulp-clean', msgCurrent));
      this.push(file);
      cb();
    } else {
      var msgOutside = 'Cannot delete files outside the current working directory. (' + filepath + '). Use option force.';
      gutil.log('gulp-clean: ' + msgOutside);
      this.emit('error', new gutil.PluginError('gulp-clean', msgOutside));
      this.push(file);
      cb();
    }
  });
};

'use strict';

var assign   = require('../object/assign')
  , isObject = require('../object/is-object')

  , captureStackTrace = Error.captureStackTrace;

exports = module.exports = function (message/*, code, ext*/) {
	var err = new Error(message), code = arguments[1], ext = arguments[2];
	if (ext == null) {
		if (isObject(code)) {
			ext = code;
			code = null;
		}
	}
	if (ext != null) assign(err, ext);
	if (code != null) err.code = code;
	if (captureStackTrace) captureStackTrace(err, exports);
	return err;
};

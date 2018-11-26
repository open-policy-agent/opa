"use strict";

/* From Twitter's Hogan.js */

var rAmp = /&/g,
	rLt = /</g,
	rGt = />/g,
	rApos =/\'/g,
	rQuot = /\"/g,
	hChars =/[&<>\"\']/;

function coerceToString(val) {
	return String((val === null || val === undefined) ? '' : val);
}

module.exports = function(str) {
	str = coerceToString(str);

	return hChars.test(str)
		? str
			.replace(rAmp,'&amp;')
			.replace(rLt,'&lt;')
			.replace(rGt,'&gt;')
			.replace(rApos,'&#39;')
			.replace(rQuot, '&quot;')
		: str;
};

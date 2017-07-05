'use strict';

var ensureString = require('../../object/validate-stringifiable-value');

module.exports = function (search) {
	var string = ensureString(this), count = 0, index = 0;

	search = ensureString(search);
	if (!search) throw new TypeError("Search string cannot be empty");
	while (true) {
		index = string.indexOf(search, index);
		if (index === -1) break;
		++count;
		index += search.length;
	}
	return count;
};

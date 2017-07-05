/**
 * @fileoverview Forbid target='_blank' attribute
 * @author Kevin Miller
 */
'use strict';

// ------------------------------------------------------------------------------
// Rule Definition
// ------------------------------------------------------------------------------

module.exports = function(context) {
  return {
    JSXAttribute: function(node) {
      if (node.name.name === 'target' && node.value.value === '_blank') {
        var relFound = false;
        var attrs = node.parent.attributes;
        for (var idx in attrs) {
          if (attrs[idx].name.name === 'rel') {
            var tags = attrs[idx].value.value.split(' ');
            if (tags.indexOf('noopener') >= 0 && tags.indexOf('noreferrer') >= 0) {
              relFound = true;
              break;
            }
          }
        }
        if (!relFound) {
          context.report(node, 'Using target="_blank" without rel="noopener noreferrer" ' +
          'is a security risk: see https://mathiasbynens.github.io/rel-noopener');
        }
      }
    }
  };
};

module.exports.schema = [];

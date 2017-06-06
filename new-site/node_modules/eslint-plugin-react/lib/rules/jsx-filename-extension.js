/**
 * @fileoverview Restrict file extensions that may contain JSX
 * @author Joe Lencioni
 */
'use strict';

var path = require('path');

// ------------------------------------------------------------------------------
// Constants
// ------------------------------------------------------------------------------

var DEFAULTS = {
  extensions: ['.jsx']
};

// ------------------------------------------------------------------------------
// Rule Definition
// ------------------------------------------------------------------------------

module.exports = function(context) {


  function getExtensionsConfig() {
    return context.options[0] && context.options[0].extensions || DEFAULTS.extensions;
  }

  // --------------------------------------------------------------------------
  // Public
  // --------------------------------------------------------------------------

  return {

    JSXElement: function(node) {
      var allowedExtensions = getExtensionsConfig();
      var filename = context.getFilename();

      var isAllowedExtension = allowedExtensions.some(function (extension) {
        return filename.slice(-extension.length) === extension;
      });

      if (isAllowedExtension) {
        return;
      }

      var extension = path.extname(filename);

      context.report({
        node: node,
        message: 'JSX not allowed in files with extension \'' + extension + '\''
      });
    }
  };

};

module.exports.schema = [{
  type: 'object',
  properties: {
    extensions: {
      type: 'array',
      items: {
        type: 'string'
      }
    }
  },
  additionalProperties: false
}];

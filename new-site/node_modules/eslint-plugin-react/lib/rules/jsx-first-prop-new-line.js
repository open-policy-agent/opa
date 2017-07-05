/**
 * @fileoverview Ensure proper position of the first property in JSX
 * @author Joachim Seminck
 */
'use strict';

// ------------------------------------------------------------------------------
// Rule Definition
// ------------------------------------------------------------------------------

module.exports = function (context) {
  var configuration = context.options[0];

  function isMultilineJSX(jsxNode) {
    return jsxNode.loc.start.line < jsxNode.loc.end.line;
  }

  return {
    JSXOpeningElement: function (node) {
      if ((configuration === 'multiline' && isMultilineJSX(node)) || (configuration === 'always')) {
        node.attributes.forEach(function(decl) {
          if (decl.loc.start.line === node.loc.start.line) {
            context.report({
              node: decl,
              message: 'Property should be placed on a new line'
            });
          }
        });
      } else if (configuration === 'never' && node.attributes.length > 0) {
        var firstNode = node.attributes[0];
        if (node.loc.start.line < firstNode.loc.start.line) {
          context.report({
            node: firstNode,
            message: 'Property should be placed on the same line as the component declaration'
          });
          return;
        }
      }
      return;
    }
  };
};

module.exports.schema = [{
  enum: ['always', 'never', 'multiline']
}];

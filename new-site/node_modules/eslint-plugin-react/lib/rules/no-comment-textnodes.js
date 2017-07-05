/**
 * @fileoverview Comments inside children section of tag should be placed inside braces.
 * @author Ben Vinegar
 */
'use strict';

// ------------------------------------------------------------------------------
// Rule Definition
// ------------------------------------------------------------------------------

module.exports = function(context) {
  function reportLiteralNode(node) {
    context.report(node, 'Comments inside children section of tag should be placed inside braces');
  }

  // --------------------------------------------------------------------------
  // Public
  // --------------------------------------------------------------------------

  return {
    Literal: function(node) {
      if (/\s*\/(\/|\*)/.test(node.value)) {
        // inside component, e.g. <div>literal</div>
        if (node.parent.type !== 'JSXAttribute' &&
            node.parent.type !== 'JSXExpressionContainer' &&
            node.parent.type.indexOf('JSX') !== -1) {
          reportLiteralNode(node);
        }
      }
    }
  };
};

module.exports.schema = [{
  type: 'object',
  properties: {},
  additionalProperties: false
}];

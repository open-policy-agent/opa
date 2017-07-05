/**
 * @fileoverview Enforce React components to have a shouldComponentUpdate method
 * @author Evgueni Naverniouk
 */
'use strict';

var Components = require('../util/Components');

module.exports = Components.detect(function (context, components) {
  var MISSING_MESSAGE = 'Component is not optimized. Please add a shouldComponentUpdate method.';

  /**
   * Checks to see if our component is decorated by PureRenderMixin via reactMixin
   * @param {ASTNode} node The AST node being checked.
   * @returns {Boolean} True if node is decorated with a PureRenderMixin, false if not.
   */
  var hasPureRenderDecorator = function (node) {
    if (node.decorators && node.decorators.length) {
      for (var i = 0, l = node.decorators.length; i < l; i++) {
        if (
          node.decorators[i].expression &&
          node.decorators[i].expression.callee &&
          node.decorators[i].expression.callee.object &&
          node.decorators[i].expression.callee.object.name === 'reactMixin' &&
          node.decorators[i].expression.callee.property &&
          node.decorators[i].expression.callee.property.name === 'decorate' &&
          node.decorators[i].expression.arguments &&
          node.decorators[i].expression.arguments.length &&
          node.decorators[i].expression.arguments[0].name === 'PureRenderMixin'
        ) {
          return true;
        }
      }
    }

    return false;
  };

  /**
   * Checks if we are declaring a shouldComponentUpdate method
   * @param {ASTNode} node The AST node being checked.
   * @returns {Boolean} True if we are declaring a shouldComponentUpdate method, false if not.
   */
  var isSCUDeclarеd = function (node) {
    return Boolean(
      node &&
      node.name === 'shouldComponentUpdate'
    );
  };

  /**
   * Checks if we are declaring a PureRenderMixin mixin
   * @param {ASTNode} node The AST node being checked.
   * @returns {Boolean} True if we are declaring a PureRenderMixin method, false if not.
   */
  var isPureRenderDeclared = function (node) {
    var hasPR = false;
    if (node.value && node.value.elements) {
      for (var i = 0, l = node.value.elements.length; i < l; i++) {
        if (node.value.elements[i].name === 'PureRenderMixin') {
          hasPR = true;
          break;
        }
      }
    }

    return Boolean(
        node &&
        node.key.name === 'mixins' &&
        hasPR
      );
  };

  /**
   * Mark shouldComponentUpdate as declared
   * @param {ASTNode} node The AST node being checked.
   */
  var markSCUAsDeclared = function (node) {
    components.set(node, {
      hasSCU: true
    });
  };

  /**
   * Reports missing optimization for a given component
   * @param {Object} component The component to process
   */
  var reportMissingOptimization = function (component) {
    context.report({
      node: component.node,
      message: MISSING_MESSAGE,
      data: {
        component: component.name
      }
    });
  };

  return {
    ArrowFunctionExpression: function (node) {
      // Stateless Functional Components cannot be optimized (yet)
      markSCUAsDeclared(node);
    },

    ClassDeclaration: function (node) {
      if (!hasPureRenderDecorator(node)) {
        return;
      }
      markSCUAsDeclared(node);
    },

    FunctionExpression: function (node) {
      // Stateless Functional Components cannot be optimized (yet)
      markSCUAsDeclared(node);
    },

    MethodDefinition: function (node) {
      if (!isSCUDeclarеd(node.key)) {
        return;
      }
      markSCUAsDeclared(node);
    },

    ObjectExpression: function (node) {
      // Search for the shouldComponentUpdate declaration
      for (var i = 0, l = node.properties.length; i < l; i++) {
        if (
          !node.properties[i].key || (
            !isSCUDeclarеd(node.properties[i].key) &&
            !isPureRenderDeclared(node.properties[i])
          )
        ) {
          continue;
        }
        markSCUAsDeclared(node);
      }
    },

    'Program:exit': function () {
      var list = components.list();

      // Report missing shouldComponentUpdate for all components
      for (var component in list) {
        if (!list.hasOwnProperty(component) || list[component].hasSCU) {
          continue;
        }
        reportMissingOptimization(list[component]);
      }
    }
  };
});

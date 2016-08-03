opa = typeof opa === 'undefined' ? {} : opa;

opa.collapse = (function () {
  'use strict';

  var classNames =
    OPA_COLLAPSE_CONFIG.baseClassName + ' ' +
    OPA_COLLAPSE_CONFIG.collapsedClassName;
  var collapsedClassName = OPA_COLLAPSE_CONFIG.collapsedClassName;
  var elements = opa.dom.getElementsByClassName(OPA_COLLAPSE_CONFIG.classNames);
  var ignoreClassName = OPA_COLLAPSE_CONFIG.ignoreClassName;

  function makeToggleCollapseHandler(element) {
    return function (event) {
      if (event.type === 'click' || event.keyCode === /* enter */ 13) {
        if (opa.dom.hasClassName(element, collapsedClassName)) {
          opa.dom.removeClassName(element, collapsedClassName);
        } else {
          opa.dom.addClassName(element, collapsedClassName);
        }
      }
    };
  }

  for (var i = elements.length; i--;) {
    var element = elements[i]
    var ignore =
      opa.dom.hasClassName(element, ignoreClassName) ||
      opa.dom.hasClassName(element.parentNode, ignoreClassName) ||
      element.offsetHeight <= OPA_COLLAPSE_CONFIG.maxHeight;

    if (ignore) {
      continue;
    }

    var target = document.createElement('a');
    target.setAttribute('tabindex', '0');
    target.className = OPA_COLLAPSE_CONFIG.targetClassName;

    element.appendChild(target);

    opa.dom.addClassName(element, classNames);
    opa.dom.on(target, 'click keydown', makeToggleCollapseHandler(element));
  }
}());

opa = typeof opa === 'undefined' ? {} : opa;

opa.dom = (function () {
  'use strict';

  var addEventListener = 'addEventListener';
  var eventTypePrefix = '';

  if (!document.addEventListener) {
    var addEventListener = 'attachEvent';
    var eventTypePrefix = 'on';
  }

  /**
   * Retrieves a collection of DOM elements from the descendents of `element`
   * that include all `classNames`.
   *
   * @param {string} classNames - A space-separated list of class names.
   * @param {Element=} [element=document] - The parent element whose
   *     descendents will be inspected for matching class names.
   *
   * @return {Array<Element>} A list of descendents of `element` that include
   *     all `classNames`.
   */
  function getElementsByClassName(classNames, element) {
    var elements = (element || document).getElementsByTagName('*');
    var output = [];

    for (var i = elements.length; i--;) {
      if (hasClassName(elements[i], classNames)) {
        output.push(elements[i]);
      }
    }

    return output;
  }

  /**
   * Adds all `classNames` to `element`.
   *
   * @param {Element} element - The DOM element to modify.
   * @param {string} classNames - A space-separated list of class names.
   */
  function addClassName(element, classNames) {
    element.className += (element.className ? ' ' : '') + classNames;
  }

  /**
   * Removes all `classNames` from `element`.
   *
   * @param {Element} element - The DOM element to modify.
   * @param {string} classNames - A space-separated list of class names.
   */
  function removeClassName(element, classNames) {
    var inputClassNames = classNames.split(/\s/);
    var elementClassNames = element.className.split(/\s/);

    for (var i = inputClassNames.length; i--;) {
      for (var j = elementClassNames.length; j--;) {
        if (inputClassNames[i] === elementClassNames[j]) {
          elementClassNames.splice(j, 1);
          break;
        }
      }
    }

    element.className = elementClassNames.join(' ');
  }

  /**
   * Determines if `element` includes all `classNames`.
   *
   * @param {Element} element - The DOM element to examine.
   * @param {string} classNames - A space-separated list of class names.
   *
   * @return {boolean} `true` if `element` includes all `classNames`.
   */
  function hasClassName(element, classNames) {
    var count = 0;
    var inputClassNames = classNames.split(/\s/);
    var elementClassNames = element.className.split(/\s/);

    for (var i = inputClassNames.length; i--;) {
      for (var j = elementClassNames.length; j--;) {
        if (inputClassNames[i] === elementClassNames[j]) {
          count++;
          break;
        }
      }
    }

    return count === inputClassNames.length;
  }

  /**
   * Registers `callback` for events of `type` on `target`.
   *
   * @param {EventTarget|Array<EventTarget>} target - The object or that should
   *     respond to events.
   * @param {string} type - The event type (for example, 'click') to listen for.
   * @param {function(Event)} callback - The function to call when an event of
   *     the specified type occurs.
   */
  function on(target, type, callback) {
    var types = type.split(/\s/);

    for (var i = types.length; i--;) {
      target[addEventListener](eventTypePrefix + types[i], callback);
    }
  }

  return {
    addClassName: addClassName,
    getElementsByClassName: getElementsByClassName,
    hasClassName: hasClassName,
    on: on,
    removeClassName: removeClassName
  };
}());

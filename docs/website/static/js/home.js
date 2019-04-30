(function () {

  function calculateScrollTop(el) {
    var y = 0
    var node = el

    while (node) {
      y += node.offsetTop
      node = node.offsetParent
    }

    return  y - 120 // margin of each "example/ use-case" is 160px
  }

  function animateScrollTop(endPosition) {
    let initialPosition = window.pageYOffset // Cross-compatible. Do not use window.scrollY
    let difference = Math.abs(endPosition - initialPosition)

    let factor = 7
    let scrollIncrement = 4
    let afterIncrement = endPosition > initialPosition ? initialPosition + scrollIncrement : initialPosition - scrollIncrement
    let time = 0

    if (endPosition > initialPosition) { // downward scroll
      for (let i = afterIncrement; i <= endPosition + scrollIncrement; i += scrollIncrement) {
        afterIncrement += scrollIncrement

        if (afterIncrement >= endPosition) {
          afterIncrement = endPosition
        }

        (function(afterIncrement, time) {
          setTimeout( () => {
            window.scrollTo(0, afterIncrement)
          }, factor * time)
        })(afterIncrement, time)

        time++
        scrollIncrement++
      }
      return;
    }

    for (let i = initialPosition; i > endPosition; i -= scrollIncrement) { // upward scroll
      afterIncrement -= scrollIncrement

      if (afterIncrement < endPosition){
        afterIncrement = endPosition
      }

        (function(afterIncrement, time) {
          setTimeout(() => {
            window.scrollTo(0, afterIncrement)
          }, time * factor)
        })(afterIncrement, time)

        time++
        scrollIncrement++
    }
  }


  function scrollIntoView(event) {
    var el = document.getElementById(event.currentTarget.getAttribute('href').substr(1))
    animateScrollTop(calculateScrollTop(el))

    // XXX: When Chrome 70.0.3538.77 navigates to an internal anchor (#),
    // it breaks the page layout so we don’t want to do that.
    event.preventDefault()
  }

  function scrollUp(event) {
    animateScrollTop(calculateScrollTop(document.getElementById('use-cases')))
    event.preventDefault()
  }

  function ready() {
    var page = document.documentElement

    if (typeof requestAnimationFrame === 'function') {
      clearFrame = cancelAnimationFrame
      setFrame = requestAnimationFrame
    }

    var anchors = document.getElementsByTagName('a')

    for (let i = 0, n = anchors.length; i < n; ++i) {
      var anchor = anchors[i]
      var href = anchor.getAttribute('href')
      var classes = (anchor.className || '').split(/\s+/)

      if (href && href[0] === '#') {
        anchor.addEventListener('click', scrollIntoView)
      } else if (classes.indexOf('action--scroll-up') > -1) {
        anchor.addEventListener('click', scrollUp)
      }
    }
  }

  (function check() { /complete/.test(document.readyState) ? ready() : setTimeout(check) }())
}())

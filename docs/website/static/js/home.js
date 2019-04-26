(function () {
  var scrollIncrement = 4
  var scrollTop = 0
  var page

  function clearSchedule(x) {
    clearTimeout(x)
  }

  function setSchedule(x) {
    setTimeout(x, 16)
  }

  function schedule(callback) {
    var frameId

    (function () {
      clearSchedule(frameId)
      frameId = setSchedule(callback)
    }())
  }

  function calculateScrollTop(el) {
    var y = 0
    var node = el

    while (node) {
      y += node.offsetTop
      node = node.offsetParent
    }

    return y
  }

  var scrollIncrement = 4

  function animateScrollTop(top, n) {
    var n = n || 1

    schedule(function () {
      var diff = top - page.scrollTop
      var increment = scrollIncrement * n

      if (diff) {
        if (Math.abs(diff) < increment) {
          page.scrollTop += diff
        } else if (diff > 0) {
          page.scrollTop += increment
        } else {
          page.scrollTop -= increment
        }

        animateScrollTop(top, ++n)
      }
    })
  }

  function scrollIntoView(event) {
    var el = document.getElementById(event.currentTarget.getAttribute('href').substr(1))
    animateScrollTop(calculateScrollTop(el) - 120)

    // XXX: When Chrome 70.0.3538.77 navigates to an internal anchor (#),
    // it breaks the page layout so we don’t want to do that.
    event.preventDefault()
  }

  function scrollUp(event) {
    animateScrollTop(calculateScrollTop(document.getElementById('use-cases')))
    event.preventDefault()
  }

  function ready() {
    page = document.documentElement

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
function initBannerVersionWarningCloseButton() {
  $(".banner-version-warning").find(".delete").click(function() {
    $(".banner-version-warning").remove();
  });
}

function navbarBurger() {
  $(".navbar-burger").click(function() {
    $(".navbar-burger").toggleClass("is-active");
    $("#mobile-menu").toggleClass("is-active");
  });
}

function versionDropdown() {
  $('.dropdown').click(function() {
    $(this).toggleClass('is-active');
  });
}

$(function() {
  navbarBurger();
  versionDropdown();
});

document.addEventListener("DOMContentLoaded", function(event) {
  anchors.add();

  initBannerVersionWarningCloseButton();

  // TODO: We should probably look into updating the tocbot library
  // but for now we can pad the bottom of the content to make
  // sure you can scroll into each section of the ToC.
  var content = $('.content')
  var lastHeading = content.children().filter(':header').sort(function (a, b) {
    var aTop = a.offsetTop;
    var bTop = b.offsetTop;
    return (aTop < bTop) ? -1 : (aTop > bTop) ? 1 : 0;
  }).last().get(0)
  var fullHeight = content.outerHeight(true) + content.offset().top
  var delta = fullHeight - lastHeading.offsetTop
  var padding = window.innerHeight - delta
  $('.toc-padding').css('paddingBottom', padding + 'px');

  tocbot.init({
    tocSelector: '.toc',
    contentSelector: '.content',
    headingSelector: 'h1, h2, h3, h4, h5',
    scrollSmooth: false,
    scrollContainer: ".dashboard-main",
    scrollEndCallback: function(e) {
      // Make sure the current ToC item we are on is visible in the nav bar
      $('.docs-nav-item.is-active')[0].scrollIntoView();
    }
  });

});


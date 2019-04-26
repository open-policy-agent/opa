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
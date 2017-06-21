$(document).ready(function(){

  $('._homepage--hamburger-menu-container').css('display', 'none');
  $(this).removeClass('is-active');
  $('._homepage--body').css('overflow', 'auto');

  $('.hamburger--spring').click(function(){

    if($(this).hasClass('is-active')){
      $('._homepage--hamburger-menu-container').css('display', 'none');
      $(this).removeClass('is-active');
      $('._homepage--body').css('overflow', 'auto');

    }
    else{
      $('._homepage--hamburger-menu-container').show();
      $(this).addClass('is-active');
      $('._homepage--body').css('overflow', 'hidden');
    }

  });



});

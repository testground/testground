  if(window.location.hash) {
      var hash = window.location.hash;

      $(hash).addClass("table-active")

      $([document.documentElement, document.body]).animate({
         scrollTop: $(hash).offset().top
      }, 200);
  }

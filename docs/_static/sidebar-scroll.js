// Keeps the left sidebar where it was across page loads, so the entry one
// clicks stays in view instead of the menu jumping back to the top.
(function () {
  var KEY = "emos-docs/sidebar-scroll";

  function container() {
    return document.querySelector(".sy-lside .sy-scrollbar");
  }

  function save() {
    var box = container();
    if (!box) return;
    try { sessionStorage.setItem(KEY, String(box.scrollTop)); } catch (e) {}
  }

  function restore() {
    var box = container();
    if (!box) return;
    var saved = null;
    try { saved = sessionStorage.getItem(KEY); } catch (e) {}
    if (saved !== null) box.scrollTop = parseInt(saved, 10) || 0;
    // Whatever was saved, make sure the current page's entry is visible
    var current = box.querySelector(".globaltoc a.current");
    if (!current) return;
    var boxRect = box.getBoundingClientRect();
    var rect = current.getBoundingClientRect();
    if (rect.top < boxRect.top || rect.bottom > boxRect.bottom) {
      box.scrollTop += rect.top - boxRect.top - boxRect.height / 2 + rect.height / 2;
    }
  }

  document.addEventListener("DOMContentLoaded", function () {
    restore();
    var box = container();
    if (box) box.addEventListener("scroll", save, { passive: true });
    window.addEventListener("pagehide", save);
  });
})();

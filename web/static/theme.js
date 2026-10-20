// The theme switch in the header of the documentation and the playground:
// light, the grey PlayStation, or dark, the black Net Yaroze. The choice is
// kept under one key, so that the two share it, and each page's head sets
// data-theme from it before the first paint. Until the visitor chooses, the
// documentation follows the system's color scheme and the playground's head
// makes it dark.
"use strict";

(() => {
  const KEY = "playground-theme";
  const button = document.getElementById("theme");
  const system = matchMedia("(prefers-color-scheme: dark)");
  const dark = () => {
    const theme = document.documentElement.dataset.theme;
    return theme ? theme === "dark" : system.matches;
  };
  function show() {
    const d = dark();
    button.setAttribute("aria-pressed", String(d));
    button.title = d ? "Switch to the grey PlayStation" : "Switch to the black Net Yaroze";
    for (const meta of document.querySelectorAll('meta[name="theme-color"]')) {
      meta.content = d ? "#1b1b1e" : "#e3e2dd";
      meta.removeAttribute("media");
    }
  }
  if (!button) return;
  button.addEventListener("click", () => {
    const theme = dark() ? "light" : "dark";
    document.documentElement.dataset.theme = theme;
    try { localStorage.setItem(KEY, theme); } catch { /* not kept */ }
    show();
  });
  system.addEventListener("change", show);
  addEventListener("storage", (e) => {
    if (e.key !== KEY) return;
    if (e.newValue) document.documentElement.dataset.theme = e.newValue;
    else delete document.documentElement.dataset.theme;
    show();
  });
  show();
})();

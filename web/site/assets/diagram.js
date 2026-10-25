// A diagram shrunk to fit a narrow column opens at its own size when it is
// tapped, in a dialog over the page that scrolls both ways.
"use strict";

{
  const figures = document.querySelectorAll("article figure.diagram");
  let dialog;

  // A diagram is worth opening when the column shows it at less than
  // three quarters of its size.
  const shrunk = (fig) => fig.getBoundingClientRect().width < 0.75 * parseFloat(fig.style.maxWidth);

  const mark = () => {
    for (const fig of figures) {
      const on = shrunk(fig);
      fig.classList.toggle("zoomable", on);
      if (on) {
        fig.setAttribute("role", "button");
        fig.setAttribute("tabindex", "0");
        fig.setAttribute("aria-label", "Open the diagram at full size");
      } else {
        fig.removeAttribute("role");
        fig.removeAttribute("tabindex");
        fig.removeAttribute("aria-label");
      }
    }
  };

  const open = (fig) => {
    if (!dialog) {
      dialog = document.createElement("dialog");
      dialog.className = "diagram-zoom";
      dialog.innerHTML = `<button class="close" type="button" aria-label="Close">×</button><div class="scroll"></div>`;
      dialog.querySelector(".close").addEventListener("click", () => dialog.close());
      // A tap outside the diagram closes it too.
      dialog.addEventListener("click", (e) => {
        if (!e.target.closest("svg")) dialog.close();
      });
      document.body.append(dialog);
    }
    const svg = fig.querySelector("svg").cloneNode(true);
    svg.style.width = fig.style.maxWidth;
    dialog.querySelector(".scroll").replaceChildren(svg);
    dialog.showModal();
  };

  for (const fig of figures) {
    fig.addEventListener("click", () => {
      if (fig.classList.contains("zoomable")) open(fig);
    });
    fig.addEventListener("keydown", (e) => {
      if ((e.key === "Enter" || e.key === " ") && fig.classList.contains("zoomable")) {
        e.preventDefault();
        open(fig);
      }
    });
  }
  if (figures.length) {
    mark();
    addEventListener("resize", mark);
  }
}

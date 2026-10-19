// A framed player reports its height whenever it changes, and the frame is
// given exactly that, so that the player scrolls with the page and never
// inside itself.
"use strict";

addEventListener("message", (e) => {
  if (!e.data || e.data.type !== "psx-player-height") return;
  for (const frame of document.querySelectorAll("iframe.player")) {
    if (frame.contentWindow === e.source) frame.style.height = `${e.data.height}px`;
  }
});

// On a narrow screen the examples are a row of chips, and the current one
// is brought into view along it.
for (const a of document.querySelectorAll(".described .side a[aria-current]")) {
  const row = a.closest("ul");
  if (row.scrollWidth > row.clientWidth) {
    row.scrollLeft = a.offsetLeft - row.offsetLeft - (row.clientWidth - a.offsetWidth) / 2;
  }
}

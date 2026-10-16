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

// The layouts: the page arranged for the job in hand. Code shows the Go
// files in the editor with the game, its controls and the compiler's
// output beside them. Scenes gives the scene editor the middle, what the
// open scene holds in the sidebar and what is picked in a column on the
// right. Assets does the same for the files on the disc: pictures and
// sprites, models, sounds, archives and video memory. Opening something
// goes to its layout (files.js), and the switch in the top bar goes back
// to what was last open in each.
//
// Outside Code the game goes on in a dock under the right column: the
// emulator's own screen is moved there, so a change shows on the console
// while its editor is in view. The dock is folded away until something is
// built or running.
//
// Also here: the sheet of keys, opened by its button, by ? anywhere but a
// text field, and by the keys button under the scene editor.
"use strict";

const layout = (() => {
  const body = document.body;
  const buttons = [...document.querySelectorAll("#layouts button")];
  const screen = document.querySelector(".screen");
  const stage = $("stage");
  const dock = $("dock");
  const dockScreen = $("dock-screen");
  const events = new EventTarget();
  let mode = body.dataset.layout || "code";
  let building = false;

  // Each layout shows the sidebar's sections it uses, folding those it
  // only might; a section's head folds or unfolds it, and the layout
  // keeps that while the page lasts.
  const SECTIONS = { code: ["scenes", "files"], scenes: ["scenes", "files"], assets: ["scenes", "files", "vram"] };
  const folded = {
    code: new Set(["scenes"]),
    scenes: new Set(["files"]),
    assets: new Set(["scenes"]),
  };
  function applyFolds() {
    for (const name of ["scenes", "files", "vram"]) {
      const el = $(`side-${name}`);
      el.hidden = !SECTIONS[mode].includes(name);
      const fold = folded[mode].has(name);
      el.classList.toggle("folded", fold);
      el.querySelector(".side-fold").setAttribute("aria-expanded", String(!fold));
    }
  }
  function toggleFold(name) {
    if (folded[mode].has(name)) folded[mode].delete(name);
    else folded[mode].add(name);
    applyFolds();
  }
  for (const head of document.querySelectorAll(".pg-bar .side-head")) {
    const name = head.querySelector(".side-fold").dataset.section;
    head.addEventListener("click", (e) => {
      if (e.target.closest(".side-add")) return;
      toggleFold(name);
    });
  }

  // set arranges the page for a layout.
  function set(next) {
    if (!SECTIONS[next]) return;
    const changed = next !== mode;
    mode = next;
    body.dataset.layout = next;
    for (const b of buttons) b.setAttribute("aria-pressed", String(b.dataset.layout === next));
    applyFolds();
    placeScreen();
    renderDock();
    if (changed) events.dispatchEvent(new CustomEvent("change", { detail: { layout: next } }));
  }
  for (const b of buttons) b.addEventListener("click", () => files.openLayout(b.dataset.layout));
  // With no project there is nothing to lay out.
  projectEvents.addEventListener("open", ({ detail: { project: p } }) => {
    for (const b of buttons) b.disabled = !p;
  });

  // The emulator's screen: under the game's controls in Code, in the dock
  // elsewhere. The canvas the emulator draws on goes with it, and the
  // emulator keeps running while it can be seen.
  function placeScreen() {
    const home = mode === "code" ? stage : dockScreen;
    if (screen.parentElement !== home) home.append(screen);
  }

  // The dock, shown outside Code once a program is building or loaded,
  // with the game's state and its controls, which work the ones under the
  // screen in Code.
  function renderDock() {
    const loaded = !!state.current;
    dock.hidden = mode === "code" || !(loaded || building);
    $("dock-state").textContent = building ? "building" : !loaded ? "" : state.running ? (state.current.id === projects.current?.id ? "live" : "running") : "paused";
    dock.classList.toggle("on", loaded && state.running);
    const pauseButton = $("dock-pause");
    pauseButton.disabled = !loaded;
    $("dock-reset").disabled = !loaded;
    pauseButton.title = state.running ? "Pause" : "Resume";
    pauseButton.setAttribute("aria-label", pauseButton.title);
    pauseButton.querySelector("use").setAttribute("href", state.running ? "#i-pause" : "#i-play");
    const muted = $("sound").getAttribute("aria-pressed") === "true";
    const soundButton = $("dock-sound");
    soundButton.setAttribute("aria-pressed", String(muted));
    soundButton.title = muted ? "Sound is off: turn it on" : "Mute";
    soundButton.querySelector("use").setAttribute("href", muted ? "#i-mute" : "#i-sound");
    soundButton.disabled = $("sound").disabled;
  }
  $("dock-pause").addEventListener("click", () => $("pause").click());
  $("dock-reset").addEventListener("click", () => $("reset").click());
  $("dock-sound").addEventListener("click", () => $("sound").click());
  $("dock-code").addEventListener("click", () => files.openLayout("code"));
  new MutationObserver(renderDock).observe($("sound"), { attributes: true, attributeFilter: ["aria-pressed", "disabled"] });

  // The sheet of keys.
  const keys = $("keys-dialog");
  function showKeys(section) {
    if (!keys.open) keys.showModal();
    for (const s of keys.querySelectorAll("section")) s.classList.toggle("here", s.dataset.keys === section);
    const at = section && keys.querySelector(`section[data-keys="${section}"]`);
    if (at) at.scrollIntoView({ block: "nearest" });
  }
  $("keys-open").addEventListener("click", () => showKeys(mode === "scenes" ? sceneTab.keysSection() : mode === "assets" ? "sprite" : "everywhere"));
  $("keys-close").addEventListener("click", () => keys.close());
  keys.addEventListener("click", (e) => { if (e.target === keys) keys.close(); });
  document.addEventListener("click", (e) => {
    const b = e.target.closest("[data-keys]");
    if (b && b.tagName === "BUTTON") showKeys(b.dataset.keys === "scene" ? sceneTab.keysSection() : b.dataset.keys);
  });
  addEventListener("keydown", (e) => {
    if (e.key !== "?" || e.ctrlKey || e.metaKey || e.altKey || e.defaultPrevented) return;
    if (e.target.closest?.("input, textarea, select, [contenteditable]") || document.querySelector("dialog[open]")) return;
    e.preventDefault();
    $("keys-open").click();
  });

  set(mode);

  return {
    set,
    get: () => mode,
    // building says a build is under way, which shows the dock.
    building(on) {
      building = on;
      renderDock();
    },
    renderDock,
    showKeys,
    events,
  };
})();

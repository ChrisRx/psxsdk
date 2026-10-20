// The playground: a project (project.js), its Go files in the editor and
// its data files beside them (files.js, sidebar.js), built by the compiler
// running in compile-worker.js, and booted like an example by app.js, with
// the compiler's output and the emulator's beside the picture.
//
// The compiler is large, so its worker starts, and fetches it, the first
// time the editor is used rather than when the page loads. The project is
// kept in this browser as it changes.
//
// Run formats the program with gofmt, then builds it, showing the
// compiler's output, and once it builds, boots it and shows the emulator's:
// what the BIOS and the program print, with the emulator's log, at the
// level chosen beside the tabs, interleaved.
"use strict";

let compiler = null;
const builds = new Map();
let nextBuild = 0;

// request sends the compiler's worker a request, starting the worker if it
// has not been, and resolves to its reply; log gets its output meanwhile.
function request(type, data, log) {
  if (!compiler) {
    compiler = new Worker(versioned("compile-worker.js"), { type: "module" });
    compiler.onmessage = (e) => {
      const msg = e.data;
      const build = builds.get(msg.id);
      if (!build) return;
      if (msg.event === "log") {
        build.log(msg.text);
        return;
      }
      builds.delete(msg.id);
      build.resolve(msg);
    };
    compiler.onerror = (e) => {
      for (const build of builds.values()) build.resolve({ ok: false, error: e.message ?? "the compiler failed to load" });
      builds.clear();
      compiler = null;
    };
  }
  const id = ++nextBuild;
  return new Promise((resolve) => {
    builds.set(id, { log, resolve });
    compiler.postMessage({ type, id, ...data });
  });
}

const buildLog = $("build-log");
const buildStatus = $("build-status");

const ttyLog = $("tty");

// The output's tabs: the compiler's and the emulator's. Output arriving in
// the one not shown marks its tab, in red for an error.
const tabs = { build: $("tab-build"), tty: $("tab-tty") };
const panels = { build: buildLog, tty: ttyLog };
let shownTab = "build";

function showTab(name) {
  shownTab = name;
  for (const [key, tab] of Object.entries(tabs)) {
    const on = key === name;
    tab.setAttribute("aria-selected", String(on));
    tab.tabIndex = on ? 0 : -1;
    panels[key].hidden = !on;
    if (on) tab.classList.remove("unread", "has-error");
  }
  document.querySelector(".tabs .level").hidden = name !== "tty";
  $("clear-tty").hidden = name !== "tty";
}

function markUnread(name, error = false) {
  if (shownTab === name) return;
  tabs[name].classList.add("unread");
  if (error) tabs[name].classList.add("has-error");
}

for (const [name, tab] of Object.entries(tabs)) {
  tab.addEventListener("click", () => showTab(name));
  tab.addEventListener("keydown", (e) => {
    if (e.key !== "ArrowLeft" && e.key !== "ArrowRight") return;
    const other = name === "build" ? "tty" : "build";
    showTab(other);
    tabs[other].focus();
  });
}

// appendTo adds to the end of a panel, keeping it scrolled to the end if
// it was there, and no longer than a few thousand nodes.
function appendTo(panel, ...nodes) {
  const atEnd = panel.scrollTop + panel.clientHeight >= panel.scrollHeight - 4;
  panel.append(...nodes);
  while (panel.childNodes.length > 4000) panel.firstChild.remove();
  if (atEnd) panel.scrollTop = panel.scrollHeight;
}

function appendLog(text) {
  appendTo(buildLog, text);
  markUnread("build");
}

// showEmulatorOutput adds what the emulator's worker sends while logging,
// printed text and log records in order, to the emulator's tab; app.js
// calls it. A record starts on a line of its own.
function showEmulatorOutput(chunks) {
  const nodes = [];
  let error = false;
  for (const { level, text } of chunks) {
    if (!level) {
      nodes.push(document.createTextNode(text));
      continue;
    }
    const last = nodes.at(-1)?.textContent ?? ttyLog.lastChild?.textContent ?? "\n";
    if (!last.endsWith("\n")) nodes.push(document.createTextNode("\n"));
    // The record's first line, and its attributes under it, quieter.
    const span = document.createElement("span");
    span.className = `lv-${level}`;
    const br = text.indexOf("\n") + 1;
    span.textContent = br > 0 ? text.slice(0, br) : text;
    nodes.push(span);
    if (br > 0 && br < text.length) {
      const attrs = document.createElement("span");
      attrs.className = "lv-attrs";
      attrs.textContent = text.slice(br);
      nodes.push(attrs);
    }
    if (level === "ERROR") error = true;
  }
  if (!nodes.length) return;
  appendTo(ttyLog, ...nodes);
  markUnread("tty", error);
}

// The level the emulator logs at, kept in this browser.
const LEVEL = "playground-log-level";
const levelSelect = $("log-level");
try { levelSelect.value = localStorage.getItem(LEVEL) ?? "info"; } catch { /* the default */ }
if (!levelSelect.value) levelSelect.value = "info";
call("log", { level: levelSelect.value });
levelSelect.addEventListener("change", () => {
  call("log", { level: levelSelect.value });
  try { localStorage.setItem(LEVEL, levelSelect.value); } catch { /* not kept */ }
});

$("clear-tty").addEventListener("click", () => { ttyLog.textContent = ""; });

// The compiler is fetched once the editor is first used, so it is likely
// ready by the time Run is pressed.
let warming = null;
function warm() {
  warming ??= (async () => {
    buildStatus.textContent = "Loading the compiler…";
    const reply = await request("load", {}, appendLog);
    if (!building) buildStatus.textContent = reply.ok ? "Ready." : `Could not load the compiler: ${reply.error}`;
    if (!reply.ok) warming = null;
  })();
}

// The compiler names a place in the program as FILE.go:LINE:COL.
const ERROR_AT = /(?:^|[\s/])([\w.-]+\.go):(\d+):(\d+)?:?\s*(.*)$/gm;

// showErrors makes each error in the log a link to its line, and marks
// those lines in the editors.
function showErrors(log) {
  const errors = [];
  buildLog.replaceChildren();
  let last = 0;
  for (const m of log.matchAll(ERROR_AT)) {
    const file = m[1];
    const line = Number(m[2]);
    const col = Number(m[3] ?? 1);
    errors.push({ file, line, message: m[4] });
    const start = m.index + m[0].indexOf(file);
    buildLog.append(log.slice(last, start));
    const a = document.createElement("a");
    a.href = "#";
    a.textContent = log.slice(start, m.index + m[0].length);
    a.addEventListener("click", (e) => {
      e.preventDefault();
      files.open(file, line, col);
    });
    buildLog.append(a);
    last = m.index + m[0].length;
  }
  buildLog.append(log.slice(last));
  files.markErrors(errors);
  return errors;
}

let building = false;

// format runs gofmt over the program's Go files and puts the results in
// the project. It returns whether gofmt could parse them all; when it could
// not, its complaints are in the compiler's tab and marked in the editors.
async function format() {
  const reply = await request("format", { files: files.goTexts() }, appendLog);
  if (!reply.ok) {
    showTab("build");
    buildLog.textContent = "";
    const errors = showErrors(reply.error);
    buildStatus.textContent = errors.length ? `${errors.length} syntax error${errors.length > 1 ? "s" : ""}` : `Could not format: ${reply.error}`;
    return false;
  }
  for (const { path, text } of reply.files) projects.current.set(path, text, "format");
  return true;
}

$("format").addEventListener("click", async () => {
  const button = $("format");
  button.disabled = true;
  try {
    if (await format()) {
      files.markErrors([]);
      buildStatus.textContent = "Formatted";
    }
  } finally {
    button.disabled = false;
  }
});

async function buildAndRun() {
  if (building || !projects.current) return;
  building = true;
  $("run").disabled = true;
  showTab("build");
  buildLog.textContent = "";
  buildStatus.textContent = "Formatting…";
  // What was running stops while its successor builds.
  pause(true);
  showOverlay("Building…");
  if (!(await format())) {
    building = false;
    $("run").disabled = false;
    showOverlay("The program does not parse; the compiler's tab says why.");
    return;
  }
  buildStatus.textContent = "Building…";
  const t0 = performance.now();
  let log = "";
  const project = projects.current;
  const result = await request("build", { files: project.list() }, (text) => {
    log += text;
    appendLog(text);
  });
  building = false;
  $("run").disabled = false;
  if (!result.ok) {
    const errors = showErrors(log);
    buildStatus.textContent = errors.length ? `${errors.length} error${errors.length > 1 ? "s" : ""}` : (result.error ?? "The build failed.");
    buildLog.scrollTop = 0;
    showOverlay("The build failed; the compiler's tab says why.");
    return;
  }
  files.markErrors([]);
  buildStatus.textContent = `Built in ${((performance.now() - t0) / 1000).toFixed(1)}s`;
  state.current = { name: project.name, id: project.id };
  state.disc = { cue: "prog.cue", files: result.files, elf: result.elf };
  ttyLog.textContent = "";
  showTab("tty");
  try {
    await boot();
    // Straight to playing: the keys go to the pad until the editor is
    // clicked again.
    document.querySelector(".screen").focus({ preventScroll: true });
  } catch (e) {
    showOverlay(`Could not start ${project.name}: ${e.message}`);
  }
}

$("run").addEventListener("click", () => buildAndRun());
files.onRun(buildAndRun);

// The screen takes the keys as the pad while it has the focus; a click on
// it gives it the focus, as the canvas under it cannot hold it.
document.querySelector(".screen").addEventListener("pointerdown", (e) => {
  if (!e.target.closest("button")) e.currentTarget.focus({ preventScroll: true });
});

// Ctrl+Enter runs, and Ctrl+Shift+F formats, from anywhere on the page,
// not only the editor. Ctrl+Z and Ctrl+Y go to the scene editor's history
// while it shows and the keys are not in a text field, and Ctrl+Z puts
// back what the sidebar removed.
addEventListener("keydown", (e) => {
  if (!(e.ctrlKey || e.metaKey) || e.defaultPrevented) return;
  if (e.key === "Enter") {
    e.preventDefault();
    buildAndRun();
  } else if (e.shiftKey && e.code === "KeyF") {
    e.preventDefault();
    $("format").click();
  } else if ((e.key === "z" || e.key === "Z" || e.key === "y" || e.key === "Y") && !e.target.closest("input, textarea, select, [contenteditable]")) {
    if (sceneTab.shown()) {
      e.preventDefault();
      if (e.key === "y" || e.key === "Y" || e.shiftKey) sceneTab.redo(); else sceneTab.undo();
    } else if (sidebar.canUndo() && (e.key === "z" || e.key === "Z")) {
      e.preventDefault();
      sidebar.undoRemove();
    }
  }
});

// The live pill by Run: lit while the program running is this project's,
// whose scene then follows the scene editor without a rebuild.
const livePill = $("live-pill");
function updateLivePill() {
  const mine = state.current?.id && state.current.id === projects.current?.id;
  livePill.hidden = !mine;
  livePill.classList.toggle("on", !!(mine && state.running));
  livePill.title = mine && state.running ? "This project is running: changes to its scene show on the console at once" : "This project's program is paused";
  projectEvents.dispatchEvent(new CustomEvent("live", { detail: { live: !!(mine && state.running) } }));
}
{
  const base = setRunning;
  setRunning = function (running) {
    base(running);
    updateLivePill();
  };
}
projectEvents.addEventListener("open", updateLivePill);

// The divider between the editor and the emulator: dragged, or moved with
// the arrow keys, it sets how wide the emulator's side is.
const pg = $("pg");
const split = $("split");
const SIDE = "playground-side";

function setSide(px) {
  const max = pg.clientWidth - (pg.querySelector(".pg-bar")?.offsetWidth ?? 0) - 320;
  const width = Math.round(Math.max(300, Math.min(px, max)));
  pg.style.setProperty("--side", `${width}px`);
  return width;
}

try {
  const saved = Number(localStorage.getItem(SIDE));
  if (saved) setSide(saved);
} catch { /* the default */ }

function keepSide(width) {
  try { localStorage.setItem(SIDE, String(width)); } catch { /* not kept */ }
}

split.addEventListener("pointerdown", (e) => {
  split.setPointerCapture(e.pointerId);
  document.body.classList.add("resizing");
  const move = (ev) => setSide(pg.getBoundingClientRect().right - ev.clientX - split.offsetWidth / 2);
  const up = (ev) => {
    split.removeEventListener("pointermove", move);
    split.removeEventListener("pointerup", up);
    document.body.classList.remove("resizing");
    keepSide(setSide(pg.getBoundingClientRect().right - ev.clientX - split.offsetWidth / 2));
  };
  split.addEventListener("pointermove", move);
  split.addEventListener("pointerup", up);
});
split.addEventListener("keydown", (e) => {
  if (e.key !== "ArrowLeft" && e.key !== "ArrowRight") return;
  e.preventDefault();
  const side = $("pg").querySelector(".pg-side").offsetWidth;
  keepSide(setSide(side + (e.key === "ArrowLeft" ? 32 : -32)));
});

for (const type of ["focusin", "pointerdown"]) {
  $("editors").addEventListener(type, warm, { once: true });
}

projects.start(startingFiles).catch((e) => {
  buildStatus.textContent = `Could not load the starting program: ${e.message}`;
});

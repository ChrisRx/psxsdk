// The page around the emulator: it picks an example, fetches its disc and
// a BIOS, and passes the emulator the pad's state as it changes. The
// emulator runs in worker.js, off this thread, keeping its own time and
// drawing onto the canvas itself.
"use strict";

// Pad bits, as emu's Button constants.
const BUTTON = {
  select: 1 << 0, l3: 1 << 1, r3: 1 << 2, start: 1 << 3,
  up: 1 << 4, right: 1 << 5, down: 1 << 6, left: 1 << 7,
  l2: 1 << 8, r2: 1 << 9, l1: 1 << 10, r1: 1 << 11,
  triangle: 1 << 12, circle: 1 << 13, cross: 1 << 14, square: 1 << 15,
};

const KEYS = {
  ArrowUp: BUTTON.up, ArrowDown: BUTTON.down, ArrowLeft: BUTTON.left, ArrowRight: BUTTON.right,
  KeyZ: BUTTON.cross, KeyX: BUTTON.circle, KeyA: BUTTON.square, KeyS: BUTTON.triangle,
  KeyQ: BUTTON.l1, KeyW: BUTTON.r1, Digit1: BUTTON.l2, Digit2: BUTTON.r2,
  Enter: BUTTON.start, Backspace: BUTTON.select,
};

// The standard gamepad layout's buttons, by index.
const PAD_BUTTONS = [
  BUTTON.cross, BUTTON.circle, BUTTON.square, BUTTON.triangle,
  BUTTON.l1, BUTTON.r1, BUTTON.l2, BUTTON.r2,
  BUTTON.select, BUTTON.start, BUTTON.l3, BUTTON.r3,
  BUTTON.up, BUTTON.down, BUTTON.left, BUTTON.right,
];

// What each example is, and what the pad does in it, from its package
// documentation. An example that is not here is listed by name alone.
const INFO = {
  check: {
    about: "Bouncing balls, with music, after the SDK's own first sample.",
    controls: "Up and down change how many balls bounce. L1 pauses. Select quits.",
  },
  chrome: {
    about: "A chrome-mapped torus turning, a Net Yaroze demo of environment mapping.",
    controls: "Start and Select together quit.",
  },
  clone: {
    about: "A first-person maze shooter, three levels deep, a Net Yaroze demo.",
    controls: "Up and down walk, left and right turn, L1 and R1 sidestep. Cross fires, Circle shows the map. Start and Select together quit.",
  },
  dungeon: {
    about: "A random maze of stone walls walked in the first person, a Japanese Net Yaroze demo.",
    controls: "The directions turn and walk, L1 and R1 sidestep, L2 and R2 look up and down, and Circle speeds everything up. Start switches to steering the ship instead; Start and Select together quit.",
  },
  flying: {
    about: "A ship over a plane of tiles, hunted by enemies that track and shoot, a Net Yaroze tutorial.",
    controls: "Up and down move the ship, Square and Circle turn it, and L2 fires. R1 and R2 change the view and L1 puts it back. Start pauses. Start and Select together quit.",
  },
  mdec: {
    about: "Full-color video played through the motion decoder.",
    controls: "Start and Select together quit.",
  },
  cube: {
    about: "A textured cube read off the disc, the program the tutorial writes.",
    controls: "The directions turn the cube.",
  },
  offscr: {
    about: "A sprite painted onto a sphere's texture while it runs, a Net Yaroze tutorial.",
    controls: "The directions, L1 and L2 move the ship, and the symbol buttons move the sprite; hold Select to move faster. Start pauses. Start and Select together quit.",
  },
  tunnel: {
    about: "A ship down an endless tunnel, shooting waves of enemies, a Net Yaroze game demo.",
    controls: "The directions, L1 and L2 move the ship, and R1 fires; hold Select to move faster. Start pauses. Start and Select together quit.",
  },
};

const $ = (id) => document.getElementById(id);
const canvas = $("screen");
const overlay = $("overlay");
const status = $("status");
const tty = $("tty");

const state = {
  examples: [],
  current: null,  // the example running
  disc: null,     // its files, kept to boot again with another BIOS
  bios: null,     // {name, data}, the visitor's own, or null for OpenBIOS
  openBIOS: null,
  running: false, // the visitor wants it running (as against paused)
  shown: !document.hidden, // the page is in a visible tab
  inView: true,   // the screen is within the viewport
  driving: false, // the worker is running frames
  keys: 0,
  touches: new Map(), // pointer id to {dpad, bits}, for each finger on the touch controls
  buttons: 0,     // the buttons last sent to the emulator
};

// The emulator's worker: requests to it, each answered by a reply with the
// same id, and what it reports unasked, its frame rate and the BIOS's
// output, and that it has stopped.
const worker = new Worker("worker.js");
const pending = new Map();
let nextID = 0;

function call(type, data = {}, transfer = []) {
  const id = ++nextID;
  return new Promise((resolve) => {
    pending.set(id, resolve);
    worker.postMessage({ type, id, ...data }, transfer);
  });
}

worker.onmessage = (e) => {
  const msg = e.data;
  switch (msg.event) {
    case "stats":
      if (state.driving) status.textContent = `${msg.fps.toFixed(1)} fps`;
      if (msg.tty) {
        tty.textContent = (tty.textContent + msg.tty).slice(-20000);
        tty.scrollTop = tty.scrollHeight;
      }
      return;
    case "stopped":
      state.driving = false;
      setRunning(false);
      showOverlay(`Stopped: ${msg.error}`);
      return;
  }
  const resolve = pending.get(msg.id);
  pending.delete(msg.id);
  resolve?.(msg);
};

worker.onerror = (e) => {
  setRunning(false);
  showOverlay(`The emulator failed to load: ${e.message ?? "unknown error"}`);
};

// The canvas, handed to the worker to draw on.
const ready = canvas.transferControlToOffscreen
  ? (() => {
      const offscreen = canvas.transferControlToOffscreen();
      return call("init", { canvas: offscreen }, [offscreen]);
    })()
  : Promise.resolve({ error: "this browser cannot draw from a worker (no OffscreenCanvas)" });

// Storing the visitor's BIOS. Without IndexedDB, as in some private
// windows, it lasts until the page is closed.
const store = {
  open() {
    return new Promise((resolve, reject) => {
      const req = indexedDB.open("psxsdk", 1);
      req.onupgradeneeded = () => req.result.createObjectStore("files");
      req.onsuccess = () => resolve(req.result);
      req.onerror = () => reject(req.error);
    });
  },
  async run(mode, fn) {
    const db = await this.open();
    return new Promise((resolve, reject) => {
      const req = fn(db.transaction("files", mode).objectStore("files"));
      req.onsuccess = () => resolve(req.result);
      req.onerror = () => reject(req.error);
    });
  },
  async get(key) {
    try { return await this.run("readonly", (s) => s.get(key)); } catch { return undefined; }
  },
  async put(key, value) {
    try { await this.run("readwrite", (s) => s.put(value, key)); } catch { /* kept in memory only */ }
  },
  async delete(key) {
    try { await this.run("readwrite", (s) => s.delete(key)); } catch { /* nothing stored */ }
  },
};

async function fetchBytes(url) {
  const res = await fetch(url);
  if (!res.ok) throw new Error(`${url}: ${res.status} ${res.statusText}`);
  return new Uint8Array(await res.arrayBuffer());
}

async function biosData() {
  if (state.bios) return state.bios.data;
  state.openBIOS ??= await fetchBytes("openbios.bin");
  return state.openBIOS;
}

// fetchDisc fetches a cue sheet and the files it names, which are relative
// to it, keyed by name relative to the cue sheet.
async function fetchDisc(cueURL) {
  const base = new URL(cueURL, location.href);
  const cue = await fetchBytes(base);
  const name = base.pathname.split("/").pop();
  const files = { [name]: cue };
  const text = new TextDecoder().decode(cue);
  for (const m of text.matchAll(/^\s*FILE\s+(?:"([^"]+)"|(\S+))/gim)) {
    const file = m[1] ?? m[2];
    files[file] = await fetchBytes(new URL(file, base));
  }
  return { cue: name, files };
}

function showOverlay(text) {
  overlay.textContent = text;
  overlay.hidden = !text;
}

function setRunning(running) {
  state.running = running;
  $("pause").textContent = running ? "Pause" : "Resume";
  $("pause").disabled = !state.current;
  $("reset").disabled = !state.current;
}

// drive runs the emulator only while the visitor wants it running and the
// screen can be seen: a hidden tab, or a player scrolled off the page,
// costs nothing. The worker's own boot starts it, so started() records
// that before calling here.
async function drive() {
  const want = state.running && state.shown && state.inView;
  if (want === state.driving) return;
  state.driving = want;
  if (!want) status.textContent = "";
  await call(want ? "resume" : "pause");
}

// pause pauses or resumes the emulator.
async function pause(paused) {
  setRunning(!paused);
  await drive();
}

document.addEventListener("visibilitychange", () => {
  state.shown = !document.hidden;
  drive();
});
new IntersectionObserver((entries) => {
  state.inView = entries.at(-1).isIntersecting;
  drive();
}).observe(canvas);

// started shows the outcome of booting, from the worker's reply.
function started(what, reply) {
  if (reply.error) {
    showOverlay(`Could not ${what}: ${reply.error}`);
    setRunning(false);
    return;
  }
  tty.textContent = "";
  showOverlay("");
  state.driving = true;
  setRunning(true);
  drive();
}

// boot boots the current example's disc with the BIOS chosen. The worker
// gets copies of both, which it keeps for a reset.
async function boot() {
  setRunning(false);
  const init = await ready;
  if (init.error) throw new Error(init.error);
  const reply = await call("boot", { bios: await biosData(), cue: state.disc.cue, files: state.disc.files });
  started(`start ${state.current.name}`, reply);
}

async function start(example) {
  setRunning(false);
  state.current = example;
  for (const a of document.querySelectorAll("#examples a")) {
    const current = a.dataset.name === example.name;
    a.setAttribute("aria-current", String(current));
    // In the row of chips on a narrow screen, bring the current one into view.
    if (current && narrow.matches) a.scrollIntoView({ block: "nearest", inline: "center" });
  }
  $("hint").textContent = INFO[example.name]?.controls ?? "";
  tty.textContent = "";
  status.textContent = "";
  showOverlay(`Loading ${example.name}…`);
  try {
    state.disc = await fetchDisc(example.cue);
    if (state.current !== example) return; // another was picked meanwhile
    await boot();
  } catch (e) {
    showOverlay(`Could not start ${example.name}: ${e.message}`);
  }
}

async function reset() {
  if (!state.current) return;
  setRunning(false);
  started("reset", await call("reset"));
}

function gamepadButtons() {
  let bits = 0;
  for (const pad of navigator.getGamepads?.() ?? []) {
    if (!pad || pad.mapping !== "standard") continue;
    pad.buttons.forEach((b, i) => { if (b.pressed && PAD_BUTTONS[i]) bits |= PAD_BUTTONS[i]; });
  }
  return bits;
}

function touchButtons() {
  let bits = 0;
  for (const t of state.touches.values()) bits |= t.bits;
  return bits;
}

const padButtons = [...document.querySelectorAll(".pad [data-button]")];

// sendButtons tells the emulator the pad's buttons, if they have changed,
// and shows them pressed on the touch controls, whatever pressed them.
function sendButtons() {
  const buttons = state.keys | touchButtons() | gamepadButtons();
  if (buttons === state.buttons) return;
  state.buttons = buttons;
  worker.postMessage({ type: "buttons", buttons });
  for (const el of padButtons) el.classList.toggle("on", (buttons & BUTTON[el.dataset.button]) !== 0);
}

// loop polls the gamepads, which only the page can read, once a frame.
function loop() {
  requestAnimationFrame(loop);
  if (state.running) sendButtons();
}

function inField(e) {
  return e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement;
}

addEventListener("keydown", (e) => {
  const bit = KEYS[e.code];
  if (!bit || inField(e) || !state.current) return;
  state.keys |= bit;
  e.preventDefault();
  sendButtons();
});
addEventListener("keyup", (e) => {
  const bit = KEYS[e.code];
  if (!bit) return;
  state.keys &= ~bit;
  sendButtons();
});
// Losing the page lets everything go, so nothing stays pressed.
function releaseAll() {
  state.keys = 0;
  state.touches.clear();
  sendButtons();
}
addEventListener("blur", releaseAll);
document.addEventListener("visibilitychange", () => { if (document.hidden) releaseAll(); });

// Touch controls. Each half of the pad is one touch area: a finger
// presses whichever button is under it as it moves, so it can roll from
// one button to the next. A finger that lands on the D-pad steers it by
// its direction from the centre until lifted, wherever it strays, with a
// diagonal pressing two directions.
const DPAD_DEAD = 0.2;   // of the D-pad's radius, from its centre
const DPAD_AXIS = 0.38;  // about sin(22.5°): beyond it, a direction is pressed

function dpadButtons(dpad, x, y) {
  const r = dpad.getBoundingClientRect();
  const dx = x - (r.left + r.width / 2);
  const dy = y - (r.top + r.height / 2);
  const d = Math.hypot(dx, dy);
  if (d < (r.width / 2) * DPAD_DEAD) return 0;
  let bits = 0;
  if (dx / d > DPAD_AXIS) bits |= BUTTON.right;
  if (dx / d < -DPAD_AXIS) bits |= BUTTON.left;
  if (dy / d > DPAD_AXIS) bits |= BUTTON.down;
  if (dy / d < -DPAD_AXIS) bits |= BUTTON.up;
  return bits;
}

function touchAt(touch, x, y) {
  if (touch.dpad) return dpadButtons(touch.dpad, x, y);
  const el = document.elementFromPoint(x, y)?.closest(".pad [data-button]");
  return el ? BUTTON[el.dataset.button] : 0;
}

function track(e) {
  const touch = state.touches.get(e.pointerId);
  if (!touch) return;
  const bits = touchAt(touch, e.clientX, e.clientY);
  if (bits & ~touch.bits) navigator.vibrate?.(8);
  touch.bits = bits;
  sendButtons();
}

function release(e) {
  if (!state.touches.delete(e.pointerId)) return;
  sendButtons();
}

for (const pad of document.querySelectorAll(".pad")) {
  pad.addEventListener("pointerdown", (e) => {
    if (e.target.closest("button")) return;
    pad.setPointerCapture(e.pointerId);
    state.touches.set(e.pointerId, { dpad: e.target.closest("[data-dpad]"), bits: 0 });
    track(e);
  });
  pad.addEventListener("pointermove", track);
  pad.addEventListener("contextmenu", (e) => e.preventDefault());
}
// A finger lifting anywhere releases what it pressed, should its pad not
// have captured it.
addEventListener("pointerup", release, { capture: true });
addEventListener("pointercancel", release, { capture: true });

// The touch controls show on a touchscreen, or once one is used.
const coarse = matchMedia("(pointer: coarse)");
const narrow = matchMedia("(max-width: 759px)");
const showTouch = () => document.body.classList.add("touch");
if (coarse.matches) showTouch();
coarse.addEventListener("change", () => { if (coarse.matches) showTouch(); });
addEventListener("pointerdown", (e) => { if (e.pointerType === "touch") showTouch(); }, { capture: true });

// Fullscreen: the browser's own where it has one for the page, and
// otherwise the stage covering the viewport, as on iPhones.
const stage = $("stage");

function showFullscreen(full) {
  stage.classList.toggle("full", full);
  document.body.classList.toggle("has-full", full);
  $("fullscreen").textContent = full ? "Exit fullscreen" : "Fullscreen";
}

async function fullscreen(full) {
  showFullscreen(full);
  try {
    if (full && stage.requestFullscreen && !document.fullscreenElement) {
      await stage.requestFullscreen({ navigationUI: "hide" });
    } else if (!full && document.fullscreenElement) {
      await document.exitFullscreen();
    }
  } catch { /* the stage covers the viewport instead */ }
}

document.addEventListener("fullscreenchange", () => {
  if (!document.fullscreenElement) showFullscreen(false);
});
addEventListener("keydown", (e) => {
  if (e.key === "Escape" && stage.classList.contains("full")) fullscreen(false);
});
$("fullscreen").addEventListener("click", () => fullscreen(!stage.classList.contains("full")));
$("exit-full").addEventListener("click", () => fullscreen(false));

$("pause").addEventListener("click", () => pause(state.running));
$("reset").addEventListener("click", reset);

// Sound: audio.js, an AudioWorklet, plays what the worker sends it, over a
// port between the two that the page sets up. Browsers only start audio
// after the visitor does something on the page, so it is set up at the
// first click or key press, and the choice of on or off is remembered.
const sound = {
  on: (() => { try { return localStorage.getItem("sound") !== "off"; } catch { return true; } })(),
  ctx: null,
  setup: null, // the setting up, once started
};

function showSound() {
  $("sound").textContent = sound.on ? "Mute" : "Unmute";
  $("sound").setAttribute("aria-pressed", String(!sound.on));
}

async function setUpSound() {
  let ctx;
  try {
    // At the SPU's rate, if the browser allows it, so nothing is resampled.
    ctx = new AudioContext({ sampleRate: 44100, latencyHint: "interactive" });
  } catch {
    ctx = new AudioContext({ latencyHint: "interactive" });
  }
  await ctx.audioWorklet.addModule("audio.js");
  const node = new AudioWorkletNode(ctx, "psx-audio", { outputChannelCount: [2] });
  node.connect(ctx.destination);
  const { port1, port2 } = new MessageChannel();
  node.port.postMessage({ port: port1 }, [port1]);
  await call("audio", { on: sound.on, port: port2 }, [port2]);
  sound.ctx = ctx;
}

async function startSound() {
  sound.setup ??= setUpSound().catch((e) => {
    $("sound").disabled = true;
    $("sound").title = `Sound is unavailable: ${e.message}`;
    throw e;
  });
  await sound.setup;
  if (sound.on) await sound.ctx.resume();
}

// Which events count as an interaction differs between browsers, so the
// first of each kind is tried; starting again is harmless.
for (const type of ["pointerdown", "pointerup", "click", "keydown"]) {
  addEventListener(type, () => { if (sound.on) startSound().catch(() => {}); }, { capture: true, once: true });
}

$("sound").addEventListener("click", async () => {
  sound.on = !sound.on;
  try { localStorage.setItem("sound", sound.on ? "on" : "off"); } catch { /* not remembered */ }
  showSound();
  try {
    await startSound();
    await call("audio", { on: sound.on });
    if (!sound.on) await sound.ctx.suspend();
  } catch { /* the button says why */ }
});

showSound();

function showBIOS() {
  $("bios-name").textContent = state.bios ? `Using ${state.bios.name}.` : "Using OpenBIOS.";
  $("bios-clear").hidden = !state.bios;
}

$("bios-file").addEventListener("change", async (e) => {
  const file = e.target.files[0];
  e.target.value = "";
  if (!file) return;
  const data = new Uint8Array(await file.arrayBuffer());
  if (data.length === 0 || (data.length & (data.length - 1)) !== 0) {
    $("bios-name").textContent = `${file.name} is ${data.length} bytes, which is not a BIOS image's size.`;
    return;
  }
  state.bios = { name: file.name, data };
  await store.put("bios", state.bios);
  showBIOS();
  if (state.disc) boot().catch((e) => showOverlay(`Could not start: ${e.message}`));
});

$("bios-clear").addEventListener("click", async () => {
  state.bios = null;
  await store.delete("bios");
  showBIOS();
  if (state.disc) boot().catch((e) => showOverlay(`Could not start: ${e.message}`));
});

function exampleFromHash() {
  const name = decodeURIComponent(location.hash.slice(1));
  return state.examples.find((x) => x.name === name);
}

addEventListener("hashchange", () => {
  const example = exampleFromHash();
  if (example && example !== state.current) start(example);
});

async function init() {
  // Embedded in a page of the documentation, which supplies its own
  // heading and picks the example through the hash, the page is only the
  // player.
  if (new URLSearchParams(location.search).has("embed")) {
    document.documentElement.classList.add("embed");
    // The page around the frame sizes it to fit, so that the player never
    // scrolls inside it: it is told the height whenever that changes, as
    // it does when the touch controller appears.
    const report = () => parent.postMessage({ type: "psx-player-height", height: document.documentElement.scrollHeight }, "*");
    new ResizeObserver(report).observe(document.body);
    report();
  }

  state.bios = (await store.get("bios")) ?? null;
  showBIOS();

  const list = $("examples");
  try {
    state.examples = await (await fetch("examples.json")).json();
  } catch (e) {
    list.innerHTML = `<li class="muted">Could not load the examples: ${e.message}</li>`;
    return;
  }
  list.replaceChildren(...state.examples.map((x) => {
    const li = document.createElement("li");
    const a = document.createElement("a");
    a.href = `#${encodeURIComponent(x.name)}`;
    a.dataset.name = x.name;
    const name = document.createElement("b");
    name.textContent = x.name;
    a.append(name);
    if (INFO[x.name]) {
      const about = document.createElement("small");
      about.textContent = INFO[x.name].about;
      a.append(about);
    }
    li.append(a);
    return li;
  }));

  // With a keyboard, the keys are worth seeing before starting.
  if (!coarse.matches) $("keys").open = true;

  const example = exampleFromHash();
  if (example) start(example);
  requestAnimationFrame(loop);
}

init();

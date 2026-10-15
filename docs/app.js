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
  running: false,
  keys: 0,
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
      if (state.running) status.textContent = `${msg.fps.toFixed(1)} fps`;
      if (msg.tty) {
        tty.textContent = (tty.textContent + msg.tty).slice(-20000);
        tty.scrollTop = tty.scrollHeight;
      }
      return;
    case "stopped":
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

// pause pauses or resumes the emulator.
async function pause(paused) {
  setRunning(!paused);
  await call(paused ? "pause" : "resume");
}

// started shows the outcome of booting, from the worker's reply.
function started(what, reply) {
  if (reply.error) {
    showOverlay(`Could not ${what}: ${reply.error}`);
    setRunning(false);
    return;
  }
  tty.textContent = "";
  showOverlay("");
  setRunning(true);
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
    a.setAttribute("aria-current", String(a.dataset.name === example.name));
  }
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

// sendButtons tells the emulator the pad's buttons, if they have changed.
function sendButtons() {
  const buttons = state.keys | gamepadButtons();
  if (buttons === state.buttons) return;
  state.buttons = buttons;
  worker.postMessage({ type: "buttons", buttons });
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
addEventListener("blur", () => {
  state.keys = 0;
  sendButtons();
});

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
  $("sound").textContent = sound.on ? "Sound on" : "Sound off";
  $("sound").setAttribute("aria-pressed", String(sound.on));
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

for (const type of ["pointerdown", "keydown"]) {
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
    a.textContent = x.name;
    a.dataset.name = x.name;
    li.append(a);
    return li;
  }));

  const example = exampleFromHash();
  if (example) start(example);
  requestAnimationFrame(loop);
}

init();

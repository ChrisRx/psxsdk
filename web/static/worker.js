// The emulator's thread. It runs psx.wasm, which web/main.go builds, keeps
// time, and draws onto the page's canvas, handed over as an OffscreenCanvas,
// so that neither emulating nor drawing holds up the page.
//
// The page sends requests, each with an id that the reply, {id, error},
// carries back:
//
//	{type: "init", canvas}              the canvas to draw on
//	{type: "boot", bios, cue, files, elf, fastStart}
//	                                    boots a disc, as psx.boot does, and
//	                                    runs it, as fast as it goes until
//	                                    the first picture unless fastStart
//	                                    is false
//	{type: "reset"}                     boots the same disc again
//	{type: "pause"}, {type: "resume"}   stops and starts running
//	{type: "buttons", buttons}          the pad's buttons, as they change
//	{type: "audio", on, port}           turns the sound on or off; port,
//	                                    once, is audio.js's, to send it to
//	{type: "symbol", name}              answers {address}, the program's
//	                                    variable of that name, or -1
//	{type: "peek", address, length}     answers {data}, bytes of RAM
//	{type: "poke", address, data}       writes bytes into RAM: the
//	                                    playground's live view of a scene
//	{type: "frame"}                     answers {bitmap}, the last picture
//	                                    drawn, as an ImageBitmap, or null
//	{type: "log", level}                sets the level the emulator logs at,
//	                                    as psx.setLogLevel does
//
// It sends the page, unasked, {event: "stats", fps, fast, tty, output}
// twice a second, fast while the fast start is running, with what the BIOS
// and the program printed meanwhile, as text in tty and, while logging,
// interleaved with the log in output, as psx.takeOutput returns it; and
// {event: "stopped", error} if the emulator stops.
"use strict";

// The build's stamp, which app.js put after this worker's name, carried on
// to the files it loads; see versioned there.
const BUILD = new URL(self.location.href).searchParams.get("v") ?? "";
const versioned = (url) => (BUILD ? `${url}?v=${BUILD}` : url);

importScripts(versioned("wasm_exec.js"));

// A frame is 263 lines of 3413 GPU cycles at 53.693175MHz, as the emulator
// times it: about 59.83 a second. At that rate the SPU makes exactly the
// 44,100 samples a second the sound plays at.
const FRAME_MS = (263 * 3413) / 53693.175;
// Frames run at most before yielding, so requests are not held up; and how
// far behind it may fall before giving up on catching up, after a stall.
const BATCH = 2;
const MAX_BEHIND = FRAME_MS * 4;
const STATS_MS = 500;
// How long a tick of the fast start runs frames for before yielding.
const FAST_MS = 12;

const ready = (async () => {
  const go = new Go();
  const { instance } = await WebAssembly.instantiateStreaming(fetch(versioned("psx.wasm")), go.importObject);
  go.run(instance);
  return self.psx;
})();

let psx = null;
let ctx = null;
let image = null; // the display's pixels, at its current size
let booted = null; // the last boot's arguments, for a reset
let audioPort = null; // where the sound goes, audio.js's worklet
let buttons = 0;

const clock = {
  running: false,
  fast: false,   // in the fast start, until the first picture
  next: 0,       // when the next frame is due
  scheduled: false,
  frames: 0,     // run since the last stats
  since: 0,
  output: [],
};
let logging = false;

// Yielding with a message rather than setTimeout(0), which browsers clamp
// to 4ms once nested.
const wake = new MessageChannel();
wake.port1.onmessage = () => tick();

function schedule() {
  if (clock.scheduled || !clock.running) return;
  clock.scheduled = true;
  const wait = clock.next - performance.now();
  if (wait > 1) setTimeout(() => tick(), wait);
  else wake.port2.postMessage(null);
}

// tick runs the frames that are due, a batch at a time, copies out and
// draws the last of them, sends their sound, and schedules the next.
function tick() {
  clock.scheduled = false;
  if (!clock.running) return;
  if (clock.fast) {
    const res = psx.runFast(FAST_MS);
    if (res.error) {
      stop(res.error);
      return;
    }
    clock.frames += res.frames;
    if (res.shown) {
      // Real time from the first picture on, with no sound from before.
      clock.fast = false;
      clock.next = performance.now();
      psx.takeAudio();
      audioPort?.postMessage("flush");
    }
  }
  const now = performance.now();
  if (now - clock.next > MAX_BEHIND) clock.next = now;

  // Behind, after a stall or on a slow device, more than one frame is due:
  // only the last is worth copying out.
  let due = 0;
  if (!clock.fast && now >= clock.next) due = Math.min(BATCH, Math.floor((now - clock.next) / FRAME_MS) + 1);
  for (let i = 0; i < due; i++) {
    const last = i === due - 1;
    const res = psx.runFrame(last ? image?.data : null);
    if (res.error) {
      stop(res.error);
      return;
    }
    // A frame at a new size has nowhere to go; the next one does.
    if (last && (!image || image.width !== res.width || image.height !== res.height)) {
      image = res.width && res.height ? new ImageData(res.width, res.height) : null;
      if (image && ctx) {
        ctx.canvas.width = res.width;
        ctx.canvas.height = res.height;
      }
    }
    clock.next += FRAME_MS;
  }
  if (due > 0) {
    if (image && ctx) ctx.putImageData(image, 0, 0);
    const sound = psx.takeAudio();
    if (sound && audioPort) audioPort.postMessage(sound.buffer, [sound.buffer]);
    clock.frames += due;
  }

  clock.output.push(...psx.takeOutput());
  if (now - clock.since >= STATS_MS) {
    const fps = (clock.frames * 1000) / (now - clock.since);
    const tty = clock.output.filter((c) => !c.level).map((c) => c.text).join("");
    self.postMessage({ event: "stats", fps, fast: clock.fast, tty, output: logging ? clock.output : undefined });
    clock.frames = 0;
    clock.since = now;
    clock.output = [];
  }
  schedule();
}

function start() {
  clock.running = true;
  clock.next = clock.since = performance.now();
  clock.frames = 0;
  schedule();
}

function stop(error) {
  clock.running = false;
  if (error) self.postMessage({ event: "stopped", error });
}

function boot({ bios, cue, files, elf, fastStart }) {
  stop();
  clock.output = [];
  const error = psx.boot(bios, cue, files, elf ?? null);
  audioPort?.postMessage("flush");
  booted = error ? null : { bios, cue, files, elf, fastStart };
  if (!error) {
    psx.setButtons(buttons);
    clock.fast = fastStart !== false;
    start();
  }
  return { error };
}

self.onmessage = async (e) => {
  const msg = e.data;
  let reply = { error: null };
  try {
    psx = await ready;
    switch (msg.type) {
      case "init":
        ctx = msg.canvas.getContext("2d");
        break;
      case "boot":
        reply = boot(msg);
        break;
      case "reset":
        reply = booted ? boot(booted) : { error: "nothing to reset" };
        break;
      case "pause":
        stop();
        audioPort?.postMessage("flush");
        break;
      case "resume":
        if (booted) start();
        break;
      case "buttons":
        buttons = msg.buttons;
        psx.setButtons(buttons);
        break;
      case "audio":
        if (msg.port) audioPort = msg.port;
        psx.setAudio(msg.on);
        if (!msg.on) audioPort?.postMessage("flush");
        break;
      case "log":
        logging = msg.level !== "off";
        psx.setLogLevel(msg.level);
        break;
      case "symbol":
        reply = { error: null, address: psx.symbol(msg.name) };
        break;
      case "peek":
        reply = { error: null, data: booted ? psx.read(msg.address, msg.length) : null };
        break;
      case "poke":
        reply = { error: booted && psx.write(msg.address, msg.data) ? null : "could not write there" };
        break;
      case "frame": {
        // The last picture drawn, for the page to show elsewhere.
        const bitmap = image ? await createImageBitmap(image) : null;
        self.postMessage({ id: msg.id, error: null, bitmap }, bitmap ? [bitmap] : []);
        return;
      }
      default:
        reply = { error: `unknown request ${msg.type}` };
    }
  } catch (err) {
    reply = { error: err.message ?? String(err) };
  }
  if (msg.id) self.postMessage({ id: msg.id, ...reply });
};

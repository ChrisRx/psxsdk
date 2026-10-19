// The emulator's sound output, an AudioWorklet. The emulator's worker sends
// it the SPU's samples, interleaved stereo 16-bit at 44.1kHz, over a port
// the page hands it, and it plays them at the output's rate.
//
// It keeps a small buffer, since the emulator makes sound a frame at a time
// on its own clock. Short of samples, it plays silence until a few frames'
// worth have arrived again; with too many, it drops the oldest, which keeps
// the delay from growing. Between the two, it holds the level by playing a
// shade faster or slower, since the emulator's clock and the output's
// never quite agree.
"use strict";

const SOURCE_RATE = 44100;
const CAPACITY = SOURCE_RATE;        // a second, as stereo frames
const START = SOURCE_RATE * 0.1;     // buffered before playing (100ms), and held
const RESUME = SOURCE_RATE * 0.05;   // buffered before playing again after running dry
const MAX = SOURCE_RATE * 0.25;      // buffered at most (250ms)
const RATE_MAX = 0.005;              // how far playback speeds up or slows down

class PSXAudio extends AudioWorkletProcessor {
  constructor() {
    super();
    this.left = new Float32Array(CAPACITY);
    this.right = new Float32Array(CAPACITY);
    this.read = 0;      // where the next frame is read, fractionally
    this.count = 0;     // frames buffered
    this.write = 0;     // where the next frame is written
    this.playing = false;
    this.dried = false; // ran dry while playing, so a smaller reserve starts it again
    this.step = SOURCE_RATE / sampleRate;

    this.port.onmessage = (e) => {
      // The page hands over the port the worker sends on.
      e.data.port.onmessage = (m) => this.receive(m.data);
    };
  }

  receive(data) {
    if (data === "flush") {
      this.count = 0;
      this.playing = false;
      this.dried = false;
      return;
    }
    const s = new Int16Array(data);
    for (let i = 0; i + 1 < s.length; i += 2) {
      this.left[this.write] = s[i] / 32768;
      this.right[this.write] = s[i + 1] / 32768;
      this.write = (this.write + 1) % CAPACITY;
    }
    // Too far ahead, skip to the newest START frames. The read position is
    // always count frames behind the write position, plus a fraction.
    this.count = Math.min(this.count + (s.length >> 1), CAPACITY);
    if (this.count > MAX) this.count = START;
    this.read = (this.write - this.count + CAPACITY) % CAPACITY + (this.read % 1);
    // Having run dry, as on a device that cannot keep up, a shorter wait
    // makes for shorter gaps.
    if (!this.playing && this.count >= (this.dried ? RESUME : START)) this.playing = true;
  }

  process(_inputs, outputs) {
    const [outL, outR] = outputs[0];
    // Above START, play faster, and below it slower, by up to RATE_MAX,
    // which is too little to hear but takes up the clocks' drift.
    const level = Math.max(-1, Math.min(1, (this.count - START) / START));
    const step = this.step * (1 + level * RATE_MAX);
    for (let i = 0; i < outL.length; i++) {
      // Linear interpolation between the two frames around the position,
      // which only matters when the output's rate is not 44.1kHz.
      if (!this.playing || this.count < 2) {
        if (this.playing) this.dried = true;
        this.playing = false;
        outL[i] = 0;
        if (outR) outR[i] = 0;
        continue;
      }
      const i0 = Math.floor(this.read);
      const i1 = (i0 + 1) % CAPACITY;
      const t = this.read - i0;
      outL[i] = this.left[i0] + (this.left[i1] - this.left[i0]) * t;
      if (outR) outR[i] = this.right[i0] + (this.right[i1] - this.right[i0]) * t;

      const next = this.read + step;
      const whole = Math.floor(next) - i0;
      this.read = next % CAPACITY;
      this.count -= whole;
    }
    return true;
  }
}

registerProcessor("psx-audio", PSXAudio);

// The sprite editor: a picture under assets/ drawn pixel by pixel with a
// palette, the way the console's sprites and textures are made. The
// picture is kept in the project as an indexed PNG with its palette in
// order, so that png2tim, whose line converts it as any picture under
// assets/, keeps the indices a program may name colors by; color 0 is
// always the transparent one, which the GPU leaves undrawn. Every change
// is saved to the project after a moment and the converter rerun, so the
// TIM the program loads follows the drawing.
//
// Tools: a pencil and an eraser (Shift draws a line from the last point),
// a fill, and a picker; the right button picks with any tool. Ctrl+Z and
// Ctrl+Y undo and redo. The palette holds up to 256 colors; a swatch is
// picked with a click and changed with a double click.
"use strict";

const spriteEditor = (() => {
  const pane = $("sprite-pane");
  const canvas = $("sprite-canvas");
  const ctx = canvas.getContext("2d");
  const paletteEl = $("sprite-palette");
  const info = $("sprite-info");
  const wInput = $("sprite-w");
  const hInput = $("sprite-h");
  const zoomSelect = $("sprite-zoom");
  const gridButton = $("sprite-grid");
  const undoButton = $("sprite-undo");
  const redoButton = $("sprite-redo");
  const colorInput = $("sprite-color");
  const optionsForm = $("sprite-options");
  const optionsInfo = $("sprite-options-info");

  const MAX = 256;    // the widest and tallest a picture may be
  const STEPS = 100;  // the undo history kept

  let path = null;  // the picture shown, under assets/
  let pic = null;   // {w, h, pixels: Uint8Array of palette indices, palette: [[r, g, b, a]]}
  let tool = "pencil";
  let color = 1;
  let zoom = 12;
  let grid = true;
  let undo = [];
  let redo = [];
  let stroke = null; // the pixels before the stroke being drawn, or null
  let last = null;   // the last point drawn, for Shift lines
  let saveTimer = 0;
  let dirty = false;

  function project() { return projects.current; }

  // The palette a new sprite starts with: transparent, then fifteen
  // colors in the way of the old 16-color palettes, sixteen entries in
  // all, which png2tim makes a 4-bit TIM of.
  const STARTER = [
    [0, 0, 0, 0], [20, 12, 28, 255], [68, 36, 52, 255], [48, 52, 109, 255], [78, 74, 78, 255],
    [133, 76, 48, 255], [52, 101, 36, 255], [208, 70, 72, 255], [117, 113, 97, 255], [89, 125, 206, 255],
    [210, 125, 44, 255], [109, 170, 44, 255], [210, 170, 153, 255], [109, 194, 202, 255],
    [218, 212, 94, 255], [222, 238, 214, 255],
  ];

  // PNG, read and written here: a picture comes in through the browser's
  // decoder, whatever its kind, and goes out indexed.
  const crcTable = new Int32Array(256);
  for (let n = 0; n < 256; n++) {
    let c = n;
    for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    crcTable[n] = c;
  }
  function crc32(bytes) {
    let c = -1;
    for (let i = 0; i < bytes.length; i++) c = crcTable[(c ^ bytes[i]) & 0xff] ^ (c >>> 8);
    return (c ^ -1) >>> 0;
  }
  function chunk(type, body) {
    const out = new Uint8Array(12 + body.length);
    const view = new DataView(out.buffer);
    view.setUint32(0, body.length);
    out.set([type.charCodeAt(0), type.charCodeAt(1), type.charCodeAt(2), type.charCodeAt(3)], 4);
    out.set(body, 8);
    view.setUint32(8 + body.length, crc32(out.subarray(4, 8 + body.length)));
    return out;
  }
  async function encodePNG(p) {
    const ihdr = new Uint8Array(13);
    const v = new DataView(ihdr.buffer);
    v.setUint32(0, p.w);
    v.setUint32(4, p.h);
    ihdr.set([8, 3, 0, 0, 0], 8); // 8 bits, indexed
    const plte = new Uint8Array(p.palette.length * 3);
    const trns = new Uint8Array(p.palette.length);
    p.palette.forEach(([r, g, b, a], i) => {
      plte.set([r, g, b], i * 3);
      trns[i] = a;
    });
    const raw = new Uint8Array((p.w + 1) * p.h);
    for (let y = 0; y < p.h; y++) raw.set(p.pixels.subarray(y * p.w, (y + 1) * p.w), y * (p.w + 1) + 1);
    const stream = new Blob([raw]).stream().pipeThrough(new CompressionStream("deflate"));
    const idat = new Uint8Array(await new Response(stream).arrayBuffer());
    const parts = [new Uint8Array([137, 80, 78, 71, 13, 10, 26, 10]), chunk("IHDR", ihdr), chunk("PLTE", plte), chunk("tRNS", trns), chunk("IDAT", idat), chunk("IEND", new Uint8Array(0))];
    const out = new Uint8Array(parts.reduce((n, c) => n + c.length, 0));
    let at = 0;
    for (const c of parts) {
      out.set(c, at);
      at += c.length;
    }
    return out;
  }

  // decode reads any picture the browser can into a palette and indices:
  // a pixel more than half transparent is color 0, and the colors are
  // numbered in the order they are met, up to 256, after which the
  // nearest is taken.
  async function decode(bytes) {
    const bitmap = await createImageBitmap(new Blob([bytes]));
    const w = Math.min(bitmap.width, MAX), h = Math.min(bitmap.height, MAX);
    const off = new OffscreenCanvas(w, h);
    const c = off.getContext("2d", { willReadFrequently: true });
    c.drawImage(bitmap, 0, 0);
    bitmap.close();
    const data = c.getImageData(0, 0, w, h).data;
    const palette = [[0, 0, 0, 0]];
    const index = new Map();
    const pixels = new Uint8Array(w * h);
    for (let i = 0; i < w * h; i++) {
      const r = data[i * 4], g = data[i * 4 + 1], b = data[i * 4 + 2], a = data[i * 4 + 3];
      if (a < 128) continue;
      const key = (r << 16) | (g << 8) | b;
      let n = index.get(key);
      if (n === undefined) {
        if (palette.length < MAX) {
          n = palette.length;
          palette.push([r, g, b, 255]);
        } else {
          n = nearest(palette, r, g, b);
        }
        index.set(key, n);
      }
      pixels[i] = n;
    }
    return { w, h, pixels, palette, cut: bitmap.width > MAX || bitmap.height > MAX };
  }
  function nearest(palette, r, g, b) {
    let best = 1, bestD = Infinity;
    for (let i = 1; i < palette.length; i++) {
      const [pr, pg, pb] = palette[i];
      const d = (pr - r) ** 2 + (pg - g) ** 2 + (pb - b) ** 2;
      if (d < bestD) {
        best = i;
        bestD = d;
      }
    }
    return best;
  }

  // Showing.
  async function show(p) {
    const data = project()?.get(p);
    if (!data) return;
    if (path !== p) {
      flush();
      path = p;
      undo = [];
      redo = [];
      last = null;
      pic = null;
      info.textContent = "Reading…";
    }
    pane.hidden = false;
    if (!pic) {
      try {
        pic = await decode(data);
        if (path !== p) return;
        if (pic.palette.length <= 1) pic.palette = STARTER.slice();
        color = Math.min(color, pic.palette.length - 1) || 1;
        fit();
      } catch (e) {
        info.textContent = `Could not read ${p}: ${e.message}`;
        return;
      }
    }
    render();
    renderOptions();
  }

  function hide() {
    flush();
    pane.hidden = true;
  }

  // fit picks the largest zoom the select offers that shows the whole
  // picture in the room the middle has.
  function fit() {
    const stage = pane.querySelector(".sprite-stage");
    const most = Math.min(Math.floor(Math.max(stage.clientWidth - 40, 200) / pic.w), Math.floor(Math.max(stage.clientHeight - 40, 200) / pic.h));
    const sizes = [...zoomSelect.options].map((o) => Number(o.value));
    zoom = sizes.filter((z) => z <= most).at(-1) ?? sizes[0];
    zoomSelect.value = String(zoom);
  }

  function css([r, g, b, a]) { return a ? `rgb(${r} ${g} ${b})` : "transparent"; }

  function render() {
    if (!pic) return;
    canvas.width = pic.w * zoom;
    canvas.height = pic.h * zoom;
    canvas.style.width = `${pic.w * zoom}px`;
    canvas.style.height = `${pic.h * zoom}px`;
    ctx.clearRect(0, 0, canvas.width, canvas.height);
    for (let y = 0; y < pic.h; y++) {
      for (let x = 0; x < pic.w; x++) {
        const c = pic.palette[pic.pixels[y * pic.w + x]];
        if (!c || !c[3]) continue;
        ctx.fillStyle = css(c);
        ctx.fillRect(x * zoom, y * zoom, zoom, zoom);
      }
    }
    if (grid && zoom >= 6) {
      ctx.strokeStyle = "rgb(128 128 128 / 0.5)";
      ctx.lineWidth = 1;
      ctx.beginPath();
      for (let x = 1; x < pic.w; x++) {
        ctx.moveTo(x * zoom + 0.5, 0);
        ctx.lineTo(x * zoom + 0.5, canvas.height);
      }
      for (let y = 1; y < pic.h; y++) {
        ctx.moveTo(0, y * zoom + 0.5);
        ctx.lineTo(canvas.width, y * zoom + 0.5);
      }
      ctx.stroke();
    }
    wInput.value = String(pic.w);
    hInput.value = String(pic.h);
    const used = new Set(pic.pixels);
    used.delete(0);
    info.textContent = `${pic.w}x${pic.h}, ${pic.palette.length} palette entries with the transparent one, ${used.size} used; a ${pic.palette.length <= 16 ? "4" : "8"}-bit TIM${pic.cut ? `; the picture was wider or taller than ${MAX} and was cut` : ""}`;
    undoButton.disabled = !undo.length;
    redoButton.disabled = !redo.length;
    renderPalette();
    for (const b of document.querySelectorAll("#sprite-tools button")) b.setAttribute("aria-pressed", String(b.dataset.tool === tool));
    gridButton.setAttribute("aria-pressed", String(grid));
  }

  function renderPalette() {
    paletteEl.replaceChildren(...pic.palette.map((c, i) => {
      const b = document.createElement("button");
      b.type = "button";
      b.className = "sprite-swatch";
      b.style.background = css(c);
      b.title = i ? `Color ${i}: ${c[0]}, ${c[1]}, ${c[2]}. Double-click to change it.` : "Color 0: transparent, not drawn";
      b.setAttribute("aria-label", b.title);
      b.setAttribute("aria-pressed", String(i === color));
      if (!i) b.classList.add("transparent");
      b.addEventListener("click", () => {
        color = i;
        renderPalette();
      });
      b.addEventListener("dblclick", () => editColor(i));
      return b;
    }));
    const add = document.createElement("button");
    add.type = "button";
    add.className = "sprite-swatch add";
    add.textContent = "+";
    add.title = "Add a color";
    add.disabled = pic.palette.length >= MAX;
    add.addEventListener("click", () => {
      pic.palette.push([255, 255, 255, 255]);
      color = pic.palette.length - 1;
      changed();
      editColor(color);
    });
    paletteEl.append(add);
  }

  // editColor opens the color input on palette entry i; the change
  // recolors every pixel of that index.
  function editColor(i) {
    if (!i) return;
    const [r, g, b] = pic.palette[i];
    colorInput.value = "#" + [r, g, b].map((v) => v.toString(16).padStart(2, "0")).join("");
    colorInput.oninput = () => {
      const v = colorInput.value;
      pic.palette[i] = [parseInt(v.slice(1, 3), 16), parseInt(v.slice(3, 5), 16), parseInt(v.slice(5, 7), 16), 255];
      changed();
    };
    colorInput.click();
  }

  // Drawing.
  function at(e) {
    const r = canvas.getBoundingClientRect();
    const x = Math.floor((e.clientX - r.left) / zoom), y = Math.floor((e.clientY - r.top) / zoom);
    return x >= 0 && y >= 0 && x < pic.w && y < pic.h ? { x, y } : null;
  }
  function put(x, y, c) {
    if (x < 0 || y < 0 || x >= pic.w || y >= pic.h) return;
    pic.pixels[y * pic.w + x] = c;
  }
  function line(a, b, c) {
    const dx = Math.abs(b.x - a.x), dy = -Math.abs(b.y - a.y);
    const sx = a.x < b.x ? 1 : -1, sy = a.y < b.y ? 1 : -1;
    let err = dx + dy, x = a.x, y = a.y;
    for (;;) {
      put(x, y, c);
      if (x === b.x && y === b.y) break;
      const e2 = 2 * err;
      if (e2 >= dy) { err += dy; x += sx; }
      if (e2 <= dx) { err += dx; y += sy; }
    }
  }
  function fill(x, y, c) {
    const from = pic.pixels[y * pic.w + x];
    if (from === c) return;
    const stack = [[x, y]];
    while (stack.length) {
      const [px, py] = stack.pop();
      if (px < 0 || py < 0 || px >= pic.w || py >= pic.h || pic.pixels[py * pic.w + px] !== from) continue;
      pic.pixels[py * pic.w + px] = c;
      stack.push([px + 1, py], [px - 1, py], [px, py + 1], [px, py - 1]);
    }
  }

  // A snapshot is the pixels with their size, for the history.
  function snapshot() { return { pixels: pic.pixels.slice(), w: pic.w, h: pic.h }; }
  function restore(s) {
    pic.pixels = s.pixels;
    pic.w = s.w;
    pic.h = s.h;
  }
  function keep(s) {
    undo.push(s);
    if (undo.length > STEPS) undo.shift();
    redo = [];
  }

  function begin() {
    stroke = snapshot();
  }
  function end() {
    if (!stroke) return;
    if (stroke.pixels.some((v, i) => v !== pic.pixels[i])) {
      keep(stroke);
      changed();
    }
    stroke = null;
    render();
  }

  canvas.addEventListener("contextmenu", (e) => e.preventDefault());
  canvas.addEventListener("pointerdown", (e) => {
    if (!pic) return;
    const p = at(e);
    if (!p) return;
    canvas.setPointerCapture(e.pointerId);
    if (e.button === 2 || tool === "pick") {
      color = pic.pixels[p.y * pic.w + p.x];
      renderPalette();
      return;
    }
    const c = tool === "eraser" ? 0 : color;
    begin();
    if (tool === "fill") {
      fill(p.x, p.y, c);
      end();
      return;
    }
    if (e.shiftKey && last) line(last, p, c);
    else put(p.x, p.y, c);
    last = p;
    render();
  });
  canvas.addEventListener("pointermove", (e) => {
    if (!stroke || !pic || (tool !== "pencil" && tool !== "eraser")) return;
    const p = at(e);
    if (!p) return;
    line(last ?? p, p, tool === "eraser" ? 0 : color);
    last = p;
    render();
  });
  const up = () => { if (stroke) end(); };
  canvas.addEventListener("pointerup", up);
  canvas.addEventListener("pointercancel", up);

  // Undo and redo swap the pixels with the history's.
  function undoOne() {
    const prev = undo.pop();
    if (!prev) return;
    redo.push(snapshot());
    restore(prev);
    changed();
  }
  function redoOne() {
    const next = redo.pop();
    if (!next) return;
    undo.push(snapshot());
    restore(next);
    changed();
  }
  undoButton.addEventListener("click", undoOne);
  redoButton.addEventListener("click", redoOne);
  addEventListener("keydown", (e) => {
    if (pane.hidden || !pic) return;
    const typing = /^(INPUT|SELECT|TEXTAREA)$/.test(document.activeElement?.tagName);
    if ((e.ctrlKey || e.metaKey) && !typing && e.key.toLowerCase() === "z") {
      e.preventDefault();
      if (e.shiftKey) redoOne(); else undoOne();
    } else if ((e.ctrlKey || e.metaKey) && !typing && e.key.toLowerCase() === "y") {
      e.preventDefault();
      redoOne();
    } else if (!e.ctrlKey && !e.metaKey && !e.altKey && !typing) {
      const t = { b: "pencil", e: "eraser", g: "fill", i: "pick" }[e.key.toLowerCase()];
      if (t) {
        tool = t;
        render();
      }
    }
  });

  // The bar.
  for (const b of document.querySelectorAll("#sprite-tools button")) {
    b.addEventListener("click", () => {
      tool = b.dataset.tool;
      render();
    });
  }
  zoomSelect.addEventListener("change", () => {
    zoom = Number(zoomSelect.value);
    render();
  });
  gridButton.addEventListener("click", () => {
    grid = !grid;
    render();
  });
  $("sprite-resize").addEventListener("click", () => {
    const w = Math.min(Math.max(Number(wInput.value) | 0, 1), MAX), h = Math.min(Math.max(Number(hInput.value) | 0, 1), MAX);
    if (!pic || (w === pic.w && h === pic.h)) return;
    keep(snapshot());
    const pixels = new Uint8Array(w * h);
    for (let y = 0; y < Math.min(h, pic.h); y++) pixels.set(pic.pixels.subarray(y * pic.w, y * pic.w + Math.min(w, pic.w)), y * w);
    restore({ pixels, w, h });
    changed();
  });
  $("sprite-clear").addEventListener("click", async () => {
    if (!pic || !(await dialogs.confirm({ title: "Clear the picture?", text: "Every pixel becomes transparent. Undo brings them back.", ok: "Clear", danger: true }))) return;
    begin();
    pic.pixels.fill(0);
    end();
  });

  // Saving: a change is written to the project after a moment, as an
  // indexed PNG, and the converter rerun to remake its TIM.
  function changed() {
    dirty = true;
    render();
    clearTimeout(saveTimer);
    saveTimer = setTimeout(flush, 300);
  }
  async function flush() {
    clearTimeout(saveTimer);
    if (!dirty || !pic || !path) return;
    dirty = false;
    const p = project();
    const savedPath = path;
    const bytes = await encodePNG(pic);
    if (!p || p !== project() || !p.has(savedPath)) return;
    p.set(savedPath, bytes, "sprite");
    convert.run();
  }

  // The converter's options for the picture, as the viewer shows them.
  function renderOptions() {
    if (!path) return;
    convert.renderOptions(path, optionsForm, optionsInfo);
  }
  let typed = 0;
  projectEvents.addEventListener("change", ({ detail: { path: changedPath, source } }) => {
    if (pane.hidden || !path) return;
    if (changedPath === path && source !== "sprite") {
      // Replaced from outside: read it again.
      pic = null;
      show(path);
    } else if (changedPath.endsWith(".go")) {
      clearTimeout(typed);
      typed = setTimeout(renderOptions, source === "editor" ? 800 : 0);
    }
  });
  projectEvents.addEventListener("open", () => {
    path = null;
    pic = null;
    hide();
  });

  // create makes a new sprite under assets/, 32 by 32 with the starter
  // palette, gives it a png2tim line and returns its path, or null.
  async function create() {
    const p = project();
    if (!p) return null;
    const clean = (v) => v.trim().replace(/\.png$/i, "").replace(/[^A-Za-z0-9_-]+/g, "_");
    const answer = await dialogs.ask({
      title: "New sprite",
      text: "A picture under assets/, drawn here pixel by pixel and converted to a TIM by a png2tim line.",
      value: "sprite.png",
      ok: "Create",
      check: (v) => (clean(v) ? "" : "A name is letters, digits, underscores and dashes."),
    });
    if (!answer) return null;
    const name = clean(answer);
    const file = `assets/${name}.png`;
    if (p.has(file)) return file;
    const bytes = await encodePNG({ w: 32, h: 32, pixels: new Uint8Array(32 * 32), palette: STARTER.slice() });
    p.set(file, bytes, "files");
    convert.added([file]);
    return file;
  }

  return { show, hide, create, shown: () => path };
})();

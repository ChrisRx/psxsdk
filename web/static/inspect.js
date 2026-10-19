// The preview beside the Files tab's tree: one TIM through its palette,
// with the map of video memory under it showing where it sits; one TMD
// turning as the console would draw it; a source under art/ with its
// converter's options; or the whole map of video memory with every TIM in
// the project where its header puts it. A block on the map is dragged
// into place. preview.js reads and draws; this is the panel.
"use strict";

const inspect = (() => {
  const canvas = $("preview-canvas");
  const ctx = canvas.getContext("2d");
  const vramCanvas = $("preview-vram");
  const title = $("preview-title");
  const info = $("preview-info");
  const objectSelect = $("preview-object");
  const rowSelect = $("preview-row");

  let shown = null; // {kind: "tim" | "tmd" | "vram", path}
  let frame = 0;    // the animation frame requested, while a model turns
  const angle = { ry: 0.6, rx: 0.35 };
  let turning = true;

  function project() { return projects.current; }

  // The decoded files, each kept while its bytes are the same.
  const cache = new Map(); // path to {data, value, error}
  function decoded(path, decode) {
    const data = project()?.get(path);
    if (!data) {
      cache.delete(path);
      return null;
    }
    let entry = cache.get(path);
    if (!entry || entry.data !== data) {
      entry = { data };
      try { entry.value = decode(data); } catch (e) { entry.error = e.message; }
      cache.set(path, entry);
    }
    return entry;
  }

  const isTIM = (path) => /\.tim$/i.test(path);
  const isTMD = (path) => /\.tmd$/i.test(path);

  // tims is every TIM in the project that reads, with its path.
  function tims() {
    const out = [];
    for (const path of project()?.onDisc() ?? []) {
      if (!isTIM(path)) continue;
      const e = decoded(path, preview.decodeTIM);
      if (e?.value) out.push({ path, tim: e.value });
    }
    return out;
  }

  function vram() {
    const v = new preview.VRAM();
    for (const { path, tim } of tims()) v.place(path, tim);
    return v;
  }

  function hex(n) { return "0x" + n.toString(16); }

  // show opens the preview of a file, or of video memory for "vram", or
  // the options of a source under art/.
  function show(path) {
    const kind = path === "vram" ? "vram" : isTIM(path) ? "tim" : isTMD(path) ? "tmd" : path.startsWith("art/") ? "source" : null;
    if (!kind) {
      // A file with nothing to draw: what it is and how big.
      hide();
      const data = project()?.get(path);
      if (!data) return;
      title.textContent = path;
      canvas.hidden = vramCanvas.hidden = form.hidden = true;
      objectSelect.hidden = rowSelect.hidden = true;
      const ext = path.replace(/^.*\./, "").toUpperCase();
      const what = { SEQ: "a score for the sound library, played through a bank", VH: "the header of a sound bank, loaded with its VB", VB: "the samples of a sound bank, loaded with its VH", VAB: "a sound bank, header and samples together", VAG: "one sound, for a bank", DAT: "a data file the program reads as it likes", GO: "Go source", JSON: "JSON", ARC: "an archive, read with package arc" }[ext];
      info.textContent = `${formatSize(data.length)}${what ? `: ${what}` : ""}.${path.startsWith("data/") ? " On the disc as it is." : ""}`;
      table.hidden = ext !== "ARC";
      if (ext === "ARC") renderArchive(data);
      return;
    }
    table.hidden = true;
    shown = { kind, path };
    objectSelect.hidden = true;
    rowSelect.hidden = true;
    canvas.className = kind;
    form.hidden = kind !== "source";
    canvas.hidden = false;
    vramCanvas.hidden = kind !== "tim";
    selectedPath = kind === "tim" ? path : null;
    render();
  }

  function hide() {
    shown = null;
    cancelAnimationFrame(frame);
    frame = 0;
  }

  function render() {
    if (!shown) return;
    if (shown.kind === "tim") renderTIM();
    else if (shown.kind === "tmd") renderTMD();
    else if (shown.kind === "source") renderSource();
    else renderVRAM();
  }

  // An archive: its table, each entry with how it is held.
  const table = $("preview-table");
  function renderArchive(data) {
    const entries = convert.readArchive(data);
    const body = table.querySelector("tbody");
    body.replaceChildren();
    if (!entries) {
      info.textContent += " Its table does not read.";
      return;
    }
    let raw = 0, held = 0;
    for (const e of entries) {
      raw += e.size;
      held += e.packed;
      const tr = document.createElement("tr");
      for (const text of [e.name, formatSize(e.size), e.method === 1 ? "deflate" : e.method === 0 ? "stored" : `method ${e.method}`, e.method === 1 ? `${Math.round(100 * e.packed / Math.max(1, e.size))}%` : ""]) {
        const td = document.createElement("td");
        td.textContent = text;
        tr.append(td);
      }
      body.append(tr);
    }
    info.textContent += ` ${entries.length} file${entries.length === 1 ? "" : "s"}, ${formatSize(raw)} unpacked, ${formatSize(held)} held: ${Math.round(100 * held / Math.max(1, raw))}%. Unpacked piece by piece into memory the program gives, at about a third to a whole megabyte a second; a stored file can be used where it lies.`;
  }

  // A source: its options, and the picture itself when it is one.
  const form = $("source-options");
  async function renderSource() {
    const { path } = shown;
    const data = project()?.get(path);
    if (!data) return hide();
    title.textContent = path;
    convert.renderOptions(path, form, info);
    canvas.hidden = true;
    if (!/\.(png|bmp|gif|jpe?g)$/i.test(path)) return;
    try {
      const bitmap = await createImageBitmap(new Blob([data]));
      if (shown?.path !== path) return;
      canvas.width = bitmap.width;
      canvas.height = bitmap.height;
      ctx.drawImage(bitmap, 0, 0);
      const scale = bitmap.width * 2 <= 480 && bitmap.height * 2 <= 256 ? 2 : 1;
      canvas.style.width = `${bitmap.width * scale}px`;
      canvas.className = "tim";
      canvas.hidden = false;
    } catch { /* not a picture the browser reads */ }
  }

  function fail(path, message) {
    title.textContent = path;
    canvas.width = 1;
    canvas.height = 1;
    canvas.style.width = "";
    info.textContent = `Could not read it: ${message}.`;
  }

  // A TIM: the picture, at twice its size when it is small, with where it
  // sits in video memory and how a primitive names it.
  function renderTIM() {
    const { path } = shown;
    const e = decoded(path, preview.decodeTIM);
    if (!e) return hide();
    if (e.error) return fail(path, e.error);
    const tim = e.value;
    title.textContent = path;
    if (tim.clut && tim.clut.h > 1) {
      rowSelect.hidden = false;
      if (rowSelect.options.length !== tim.clut.h) {
        rowSelect.replaceChildren(...Array.from({ length: tim.clut.h }, (_, i) => new Option(`palette ${i}`, String(i))));
      }
    }
    const row = rowSelect.hidden ? 0 : Number(rowSelect.value);
    const img = preview.timImage(tim, row);
    canvas.width = img.width;
    canvas.height = img.height;
    ctx.putImageData(img, 0, 0);
    const scale = img.width * 2 <= 480 && img.height * 2 <= 256 ? 2 : 1;
    canvas.style.width = `${img.width * scale}px`;
    const [u, v] = preview.uv(tim);
    const parts = [
      `${tim.bits}-bit, ${tim.width} by ${tim.height}, at ${tim.pixels.x}, ${tim.pixels.y} in video memory`,
      tim.clut ? `palette of ${tim.clut.w}${tim.clut.h > 1 ? ` by ${tim.clut.h}` : ""} at ${tim.clut.x}, ${tim.clut.y}` : "no palette",
      `texture page ${hex(preview.tpage(tim))}${tim.clut ? `, palette ${hex(preview.cba(tim))}` : ""}, U ${u} V ${v}`,
    ];
    info.textContent = parts.join(". ") + ".";
    renderVRAM(vramCanvas);
  }

  // A TMD: drawn at 320 by 240, turning, textured from the project's TIMs.
  function renderTMD() {
    const { path } = shown;
    const e = decoded(path, preview.decodeTMD);
    if (!e) return hide();
    if (e.error) return fail(path, e.error);
    const tmd = e.value;
    title.textContent = path;
    if (tmd.objects.length > 1) {
      objectSelect.hidden = false;
      if (objectSelect.options.length !== tmd.objects.length) {
        objectSelect.replaceChildren(...tmd.objects.map((_, i) => new Option(`object ${i}`, String(i))));
      }
    }
    const which = objectSelect.hidden ? 0 : Math.min(Number(objectSelect.value), tmd.objects.length - 1);
    canvas.width = preview.W;
    canvas.height = preview.H;
    canvas.style.width = "";
    const s = preview.stats(tmd);
    const o = tmd.objects[which];
    const img = ctx.createImageData(preview.W, preview.H);
    const v = vram();
    const drawn = preview.renderModel(img, o, v, angle.ry, angle.rx);
    ctx.putImageData(img, 0, 0);
    const textured = preview.polygons(o).filter((p) => p.texture).length;
    info.textContent = `${s.objects} object${s.objects === 1 ? "" : "s"}, ${s.verts} vertices, ${s.polys} polygons${s.lines ? `, ${s.lines} lines` : ""}${textured ? `; ${textured} textured, from the TIMs in data/` : ""}. ${drawn} facing the camera. Drag to turn it.`;
    cancelAnimationFrame(frame);
    frame = requestAnimationFrame(turn);
  }

  let last = 0;
  function turn(now) {
    frame = 0;
    if (!shown || shown.kind !== "tmd" || !filesTab.shown() || document.hidden) return;
    if (turning && now - last > 40) {
      last = now;
      angle.ry += 0.02;
      renderTMD();
      return;
    }
    frame = requestAnimationFrame(turn);
  }

  // Turning the model by hand.
  canvas.addEventListener("pointerdown", (e) => {
    if (shown?.kind === "tmd") {
      canvas.setPointerCapture(e.pointerId);
      turning = false;
      let lx = e.clientX, ly = e.clientY;
      const move = (ev) => {
        angle.ry += (ev.clientX - lx) * 0.01;
        angle.rx = Math.max(-1.5, Math.min(1.5, angle.rx + (ev.clientY - ly) * 0.01));
        lx = ev.clientX;
        ly = ev.clientY;
        renderTMD();
      };
      const up = () => {
        canvas.removeEventListener("pointermove", move);
        canvas.removeEventListener("pointerup", up);
        turning = true;
        if (!frame) frame = requestAnimationFrame(turn);
      };
      canvas.addEventListener("pointermove", move);
      canvas.addEventListener("pointerup", up);
      return;
    }
    if (shown?.kind === "vram") dragVRAM(e, canvas);
  });
  vramCanvas.addEventListener("pointerdown", (e) => dragVRAM(e, vramCanvas));

  // Video memory: 1024 by 512 units, each a pixel here, with the display
  // buffers a 320 by 240 program uses marked, and every block of every TIM
  // drawn where it is. A block is dragged to move it, which rewrites the
  // TIM's header.
  let placed = []; // the blocks drawn, in drawing order
  function renderVRAM(target = canvas) {
    const ctx = target.getContext("2d");
    if (target === canvas) {
      title.textContent = "Video memory";
      canvas.width = 1024;
      canvas.height = 512;
      canvas.style.width = "";
    }
    const v = vram();
    placed = v.placed;
    ctx.fillStyle = "#101012";
    ctx.fillRect(0, 0, 1024, 512);
    ctx.fillStyle = "#1c1c24";
    ctx.fillRect(0, 0, 320, 480);
    ctx.strokeStyle = "#3a3a4a";
    ctx.lineWidth = 1;
    ctx.strokeRect(0.5, 0.5, 319, 239);
    ctx.strokeRect(0.5, 240.5, 319, 239);
    ctx.fillStyle = "#6a6a80";
    ctx.font = "12px Barlow, system-ui, sans-serif";
    ctx.fillText("display buffer 0", 6, 14);
    ctx.fillText("display buffer 1", 6, 254);
    ctx.imageSmoothingEnabled = false;
    const tmp = document.createElement("canvas");
    const tctx = tmp.getContext("2d");
    for (const entry of placed) {
      const { tim, block, kind } = entry;
      let img;
      if (kind === "pixels") {
        img = preview.timImage(tim, 0);
      } else {
        img = new ImageData(Math.max(1, block.w), Math.max(1, block.h));
        for (let i = 0; i < block.data.length; i++) {
          const c = block.data[i];
          const o = img.data;
          o[4 * i] = (c & 31) * 255 / 31;
          o[4 * i + 1] = (c >> 5 & 31) * 255 / 31;
          o[4 * i + 2] = (c >> 10 & 31) * 255 / 31;
          o[4 * i + 3] = 255;
        }
      }
      tmp.width = img.width;
      tmp.height = img.height;
      tctx.putImageData(img, 0, 0);
      ctx.drawImage(tmp, block.x, block.y, block.w, block.h);
      const over = block.x < 320 && block.y < 480;
      const selected = selectedPath === entry.path;
      ctx.strokeStyle = over ? "#f26670" : selected ? "#8fa8ff" : kind === "clut" ? "#c2a6ff" : "#9ad48c";
      ctx.strokeRect(block.x + 0.5, block.y + 0.5, Math.max(1, block.w - 1), Math.max(1, block.h - 1));
    }
    if (target !== canvas) return;
    if (!placed.length) info.textContent = "No TIM in data/ yet. Add one and it is drawn here where its header puts it; drag it to move it.";
    else if (!hovered) info.textContent = `${tims().length} TIM${tims().length === 1 ? "" : "s"}. Green is a picture, purple its palette, red one over a display buffer. Drag a block to move it; a model textured from it then needs its texture page and palette changed to match.`;
  }
  let selectedPath = null;
  let hovered = null;

  function blockAt(e, target = canvas) {
    const r = target.getBoundingClientRect();
    const x = (e.clientX - r.left) * target.width / r.width;
    const y = (e.clientY - r.top) * target.height / r.height;
    for (let i = placed.length - 1; i >= 0; i--) {
      const b = placed[i].block;
      if (x >= b.x && x < b.x + b.w && y >= b.y && y < b.y + b.h) return { entry: placed[i], x, y };
    }
    return { entry: null, x, y };
  }

  for (const target of [canvas, vramCanvas]) {
    target.addEventListener("pointermove", (e) => {
      if (dragging || (target === canvas && shown?.kind !== "vram")) return;
      const { entry, x, y } = blockAt(e, target);
      hovered = entry;
      target.style.cursor = entry ? "grab" : "";
      if (target !== canvas) return;
      if (entry) {
        const b = entry.block;
        info.textContent = `${entry.path}: ${entry.kind === "clut" ? "palette" : "picture"}, ${b.w} by ${b.h} units at ${b.x}, ${b.y}.`;
      } else {
        info.textContent = `${Math.floor(x)}, ${Math.floor(y)}`;
      }
    });
    target.addEventListener("pointerleave", () => {
      if (!dragging && (target !== canvas || shown?.kind === "vram")) {
        hovered = null;
        renderVRAM(target);
      }
    });
  }

  let dragging = null;
  function dragVRAM(e, target) {
    const { entry, x, y } = blockAt(e, target);
    if (!entry) return;
    target.setPointerCapture(e.pointerId);
    const b = entry.block;
    dragging = { entry, ox: b.x, oy: b.y, sx: x, sy: y };
    if (target === canvas) selectedPath = entry.path;
    target.style.cursor = "grabbing";
    const move = (ev) => {
      const r = target.getBoundingClientRect();
      const px = (ev.clientX - r.left) * target.width / r.width;
      const py = (ev.clientY - r.top) * target.height / r.height;
      let nx = Math.round(dragging.ox + px - dragging.sx);
      let ny = Math.round(dragging.oy + py - dragging.sy);
      // A palette starts on a 16-unit boundary, as its address counts them.
      if (entry.kind === "clut") nx = Math.round(nx / 16) * 16;
      b.x = Math.max(0, Math.min(1024 - b.w, nx));
      b.y = Math.max(0, Math.min(512 - b.h, ny));
      renderVRAM(target);
      info.textContent = `${entry.path}: ${entry.kind === "clut" ? "palette" : "picture"} to ${b.x}, ${b.y}.`;
    };
    const up = () => {
      target.removeEventListener("pointermove", move);
      target.removeEventListener("pointerup", up);
      target.style.cursor = "grab";
      const moved = b.x !== dragging.ox || b.y !== dragging.oy;
      dragging = null;
      // A TIM a generate line makes is moved by its line, so that the
      // models textured from it follow; any other by its header.
      if (moved && !convert.moveTIM(entry.path, entry.kind, b.x, b.y)) {
        const bytes = preview.moveTIM(project().get(entry.path), b, b.x, b.y);
        project().set(entry.path, bytes, "vram");
      }
      render();
    };
    target.addEventListener("pointermove", move);
    target.addEventListener("pointerup", up);
  }

  objectSelect.addEventListener("change", render);
  rowSelect.addEventListener("change", render);
  document.addEventListener("visibilitychange", () => { if (!document.hidden && shown?.kind === "tmd" && !frame) frame = requestAnimationFrame(turn); });

  projectEvents.addEventListener("open", hide);
  projectEvents.addEventListener("files", () => {
    if (!shown) return;
    if (shown.kind !== "vram" && !project()?.has(shown.path)) hide();
    else render();
  });
  projectEvents.addEventListener("change", ({ detail: { path, source } }) => {
    if (!shown) return;
    if (shown.kind === "source") {
      // The options come from the Go file's lines; the picture from art/.
      if (path === shown.path || (path.endsWith(".go") && source !== "editor")) render();
      return;
    }
    if (!path.startsWith("data/")) return;
    if (shown.kind === "tim" && path === shown.path) render();
    else if (shown.kind === "tmd" && (path === shown.path || isTIM(path))) render();
    else if (shown.kind === "vram" && isTIM(path) && source !== "vram") render();
  });

  // Typing a line by hand in the editor changes the options too, once the
  // typing pauses.
  let typed = 0;
  projectEvents.addEventListener("change", ({ detail: { path, source } }) => {
    if (shown?.kind !== "source" || source !== "editor" || !path.endsWith(".go")) return;
    clearTimeout(typed);
    typed = setTimeout(render, 800);
  });

  // resume restarts a model's turning once the tab shows again.
  function resume() {
    if (shown?.kind === "tmd" && !frame) frame = requestAnimationFrame(turn);
  }

  return { show, hide, resume, shown: () => shown, isTIM, isTMD };
})();

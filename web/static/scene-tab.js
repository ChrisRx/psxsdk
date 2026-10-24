// The Scene tab: the project's scenes under scenes/, edited by placing
// things. The viewport draws it at 320 by 240 the way the console does
// (preview.js), through the scene's own camera, projection and lights,
// with the models textured from the project's TIMs; an object is picked
// by clicking it, moved by dragging it across the view, or into it with
// Shift, and placed exactly in the inspector under the picture; the
// camera orbits what it looks at when the empty picture is dragged, and
// the wheel brings it closer. Every change goes into the scene's file and
// mkscene cooks its data file from it (convert.js), so Run shows it.
//
// The units are the console's: positions in world units, rotations with
// 4096 a whole turn, scales with 4096 as 1.0, sprites in pixels from the
// middle of the screen, as GsInit3D puts the origin.
//
// While the project's program runs in the emulator beside it, each change
// also goes straight into the program's memory, over the scene it loaded,
// and shows on the console without a rebuild (poke, below).
"use strict";

const sceneTab = (() => {
  const pane = $("scene-pane");
  const canvas = $("scene-canvas");
  const ctx = canvas.getContext("2d");
  const inspector = $("scene-inspector");
  const cameraForm = $("scene-camera");
  const lightsForm = $("scene-lights");
  const status = $("scene-status");
  const addModel = $("scene-add-model");
  const addSprite = $("scene-add-sprite");
  const outline = $("scene-outline");
  const snapButton = $("scene-snap");
  let snap = false; // positions to 16 units and turns to 64ths while dragging
  let gizmoMode = "move"; // move, rotate or scale: W, E and R
  let typing = null;      // a typed transform under way: {op, axis, text}
  let flying = false;     // the right button is held: WASD move the camera
  let showConsole = false; // the running program's picture in the viewport
  let consoleFrame = null; // its last picture
  let consoleTimer = 0;
  let search = "";        // the outline's filter
  const problems = new Map(); // object index to what is wrong with it

  const W = preview.W;
  const H = preview.H;
  const img = ctx.createImageData(W, H);
  const ids = new Int16Array(W * H); // what each pixel shows: objects from 1, sprites from -1

  let doc = null;      // the scene being edited, parsed
  let current = null;  // its path, scenes/NAME.json

  // scenePath is the scene being edited: the one picked, or the first.
  function scenePath() {
    const p = project();
    if (!p) return null;
    if (current && p.has(current)) return current;
    current = p.sceneFiles()[0] ?? null;
    return current;
  }
  const stemOf = (path) => path.replace(/^.*\//, "").replace(/\.json$/, "");
  // datPath is the data file mkscene makes from a scene: what its line
  // says, or the name one would be given.
  function datPath(path = scenePath()) {
    if (!path) return null;
    const line = convert.lineForSource(path);
    return (line && (line.values.get("-o") ?? line.values.get("--output"))) || convert.outputFor(path, "mkscene");
  }
  let selected = null; // {kind: "object" | "sprite", index}
  let shown = false;
  let error = "";

  function project() { return projects.current; }

  // The scene.json of a project with none: a camera above and before the
  // origin, one light from over its shoulder, nothing placed.
  function blank() {
    return {
      camera: { from: [0, -300, -900], at: [0, 0, 0] },
      projection: 320,
      ambient: [1024, 1024, 1024],
      lights: [{ direction: [1, 1, 1], color: [192, 192, 192] }],
      textures: [],
      models: [],
      objects: [],
      sprites: [],
    };
  }

  // read parses the scene being edited, filling in what it leaves out.
  function read() {
    const path = scenePath();
    const text = path ? project().text(path) : undefined;
    if (text === undefined) {
      doc = null;
      return;
    }
    try {
      const d = JSON.parse(text);
      const b = blank();
      doc = {
        camera: { from: d.camera?.from ?? b.camera.from, at: d.camera?.at ?? b.camera.at, twist: d.camera?.twist ?? 0 },
        projection: d.projection ?? 320,
        ambient: d.ambient ?? b.ambient,
        lights: (d.lights ?? []).slice(0, 3),
        textures: d.textures ?? [],
        models: d.models ?? [],
        objects: (d.objects ?? []).map((o) => ({ name: o.name ?? "", model: o.model ?? "", index: o.index ?? 0, position: o.position ?? [0, 0, 0], rotation: o.rotation ?? [0, 0, 0], scale: o.scale ?? [0, 0, 0], fields: o.fields ?? [0, 0], hidden: !!o.hidden })),
        sprites: (d.sprites ?? []).map((s) => ({ attribute: 0, x: 0, y: 0, w: 0, h: 0, tpage: 0, u: 0, v: 0, cx: 0, cy: 0, r: 128, g: 128, b: 128, mx: 0, my: 0, scalex: 0, scaley: 0, rotate: 0, ...s })),
        background: null,
      };
      if (d.background?.texture) {
        const bg = d.background;
        const width = Math.max(1, Math.min(64, bg.width | 0 || 20));
        const height = Math.max(1, Math.min(64, bg.height | 0 || 15));
        const map = new Array(width * height).fill(0);
        for (let i = 0; i < map.length && i < (bg.map?.length ?? 0); i++) map[i] = bg.map[i];
        doc.background = { texture: bg.texture, cell: 16, width, height, map, scroll: bg.scroll ?? [0, 0], window: bg.window ?? [0, 0, 0, 0], brightness: bg.brightness ?? [128, 128, 128] };
      }
      error = "";
    } catch (e) {
      doc = null;
      error = `${path} does not parse: ${e.message}`;
    }
    if (selected && !(selected.kind === "object" ? doc?.objects[selected.index] : doc?.sprites[selected.index])) selected = null;
  }

  // The project's TIMs and TMDs, by the paths the disc knows them by.
  // discPath is the name a scene gives a file: its disc path, which for a
  // file packed in an archive is the name the archive looks up, ASSETS/ and
  // the file's name.
  const discPath = (path) => (path.startsWith("pack/") ? "ASSETS/" + path.replace(/^.*\//, "") : path).toUpperCase();
  function dataFiles(re) {
    return (project()?.onDisc() ?? []).filter((p) => re.test(p));
  }
  const cache = new Map(); // path to {data, value}
  function decoded(discName, decode) {
    const path = (project()?.onDisc() ?? []).find((p) => discPath(p) === discName);
    if (!path) return null;
    const data = project().get(path);
    let e = cache.get(path);
    if (!e || e.data !== data) {
      e = { data, value: null };
      try { e.value = decode(data); } catch { /* not readable */ }
      cache.set(path, e);
    }
    return e.value;
  }
  const tmdFor = (name) => decoded(name, preview.decodeTMD);
  const timFor = (name) => decoded(name, preview.decodeTIM);

  let vramCache = { keys: "", vram: null };
  function vram() {
    const paths = dataFiles(/\.tim$/i);
    const keys = paths.map((p) => p + ":" + project().get(p).length).join(",");
    if (vramCache.keys !== keys || !vramCache.vram || paths.some((p) => cache.get(p)?.data !== project().get(p))) {
      const v = new preview.VRAM();
      for (const p of paths) {
        const tim = timFor(discPath(p));
        if (tim) v.place(p, tim);
      }
      vramCache = { keys, vram: v };
    }
    return vramCache.vram;
  }

  // Writing: the lists of models and textures follow what is placed, and
  // the TIMs the models' texture pages name, which the program must load.
  function lists() {
    const models = [...new Set(doc.objects.map((o) => o.model).filter(Boolean))];
    const textures = new Set(doc.sprites.map((s) => s.texture).filter(Boolean));
    if (doc.background?.texture) textures.add(doc.background.texture);
    const pages = new Set();
    for (const m of models) {
      const tmd = tmdFor(m);
      if (!tmd) continue;
      for (const o of tmd.objects) for (const p of preview.polygons(o)) if (p.texture) pages.add(p.texture.tpage & 0x19f);
    }
    for (const path of dataFiles(/\.tim$/i)) {
      const tim = timFor(discPath(path));
      if (tim && pages.has(preview.tpage(tim))) textures.add(discPath(path));
    }
    doc.models = models;
    doc.textures = [...textures];
  }

  let writeTimer = 0;
  let cookTimer = 0;
  function changed(now = false) {
    render();
    renderOutline();
    clearTimeout(writeTimer);
    writeTimer = setTimeout(write, now ? 0 : 150);
  }

  // Undo and redo: the scene is one text, so each step is the text before
  // a change. A drag, or a run of typing, is one step: while an edit is
  // under way (editing, from pointerdown to pointerup) or within a moment
  // of the last step, nothing new is pushed.
  const history = { undo: [], redo: [] };
  let editing = null; // {pushed} while a drag is on
  let lastStep = 0;
  function snapshot() {
    const prev = current ? project()?.text(current) : undefined;
    if (prev === undefined) return;
    const now = Date.now();
    const fresh = editing ? !editing.pushed : now - lastStep > 800;
    if (fresh) {
      history.undo.push(prev);
      if (history.undo.length > 200) history.undo.shift();
      history.redo.length = 0;
      if (editing) editing.pushed = true;
    }
    lastStep = now;
    renderHistory();
  }
  function beginEdit() { editing = { pushed: false }; }
  function endEdit() { editing = null; }
  function undo() { step(history.undo, history.redo); }
  function redo() { step(history.redo, history.undo); }
  function step(from, to) {
    if (!from.length || !project()) return;
    clearTimeout(writeTimer);
    const now = project().text(current);
    const text = from.pop();
    to.push(now);
    lastStep = 0;
    project().set(current, text, "scene-history");
    read();
    renderAll();
    cookSoon();
  }
  function renderHistory() {
    $("scene-undo").disabled = !history.undo.length;
    $("scene-redo").disabled = !history.redo.length;
  }
  function cookSoon() {
    clearTimeout(cookTimer);
    cookTimer = setTimeout(async () => {
      if (await convert.run()) await poke();
    }, 400);
  }

  function write() {
    if (!doc || !project() || !current) return;
    snapshot();
    lists();
    const out = { ...doc };
    out.objects = doc.objects.map((o) => {
      const r = { name: o.name, model: o.model, index: o.index, position: o.position, rotation: o.rotation };
      if (o.scale.some((c) => c)) r.scale = o.scale;
      if (o.fields.some((c) => c)) r.fields = o.fields;
      if (o.hidden) r.hidden = true; // the editor's own; mkscene leaves it
      return r;
    });
    if (!doc.camera.twist) out.camera = { from: doc.camera.from, at: doc.camera.at };
    if (!doc.background) delete out.background;
    else {
      const bg = doc.background;
      out.background = { texture: bg.texture, cell: 16, width: bg.width, height: bg.height, scroll: bg.scroll, brightness: bg.brightness, map: "\0map" };
      if (bg.window.some((c) => c)) out.background.window = bg.window;
    }
    // The map goes a row to a line, which is readable where one number a
    // line is not.
    let text = JSON.stringify(out, null, 2);
    if (doc.background) {
      const bg = doc.background;
      const rows = [];
      for (let y = 0; y < bg.height; y++) rows.push("      " + bg.map.slice(y * bg.width, (y + 1) * bg.width).join(", "));
      text = text.replace('"\\u0000map"', "[\n" + rows.join(",\n") + "\n    ]");
    }
    project().set(current, text + "\n", "scene");
    cookSoon();
  }

  // The live view: when this project's program is running, the cooked
  // scene is written into its memory, over the file it loaded, and the
  // scene package reads it again at its next frame (its live variable
  // says where the file is and takes the flag). A scene grown past the
  // memory the program loaded it into needs a Run.
  const liveStatus = $("scene-live");
  const LIVE = "go.chrisrx.dev/psx/scene.live";
  async function poke() {
    const p = project();
    if (!p || !state.running || state.current?.id !== p.id) {
      liveStatus.textContent = "";
      return;
    }
    const dat = p.get(datPath());
    if (!dat) return;
    const { address } = await call("symbol", { name: LIVE });
    if (!(address >= 0)) {
      liveStatus.textContent = "The running program does not read a scene.";
      return;
    }
    const head = await call("peek", { address, length: 48 });
    if (!head.data) return;
    const view = new DataView(head.data.buffer, head.data.byteOffset, head.data.byteLength);
    const ptr = view.getUint32(0, true), words = view.getUint32(4, true);
    if (!ptr || !words) {
      liveStatus.textContent = "The running program has not loaded a scene yet.";
      return;
    }
    // The program says which file it loaded; another scene's changes
    // wait until it loads that one.
    const loaded = new TextDecoder().decode(head.data.subarray(16, 48)).replace(/\0[^]*$/, "").replace(/^.*\//, "").replace(/;\d+$/, "");
    const want = datPath().replace(/^.*\//, "");
    if (loaded && loaded.toUpperCase() !== want.toUpperCase()) {
      liveStatus.textContent = `The program is showing ${loaded}, not ${want}: changes here show when it loads ${want}.`;
      return;
    }
    if (dat.length > words * 4) {
      liveStatus.textContent = "Run to see it: the scene has grown past the memory the program loaded it into.";
      return;
    }
    const wrote = await call("poke", { address: ptr, data: dat });
    if (wrote.error) {
      liveStatus.textContent = `Could not update the running program: ${wrote.error}`;
      return;
    }
    await call("poke", { address: address + 8, data: new Uint8Array([1, 0, 0, 0]) });
    liveStatus.textContent = "";
  }

  // The camera: the basis of the view, from where it is and what it looks
  // at, with the console's axes, Y down and Z away.
  function camera() {
    const [fx, fy, fz] = doc.camera.from;
    const [ax, ay, az] = doc.camera.at;
    let f = [ax - fx, ay - fy, az - fz];
    let len = Math.hypot(...f) || 1;
    f = f.map((c) => c / len);
    let r = [f[2], 0, -f[0]]; // down × forward
    len = Math.hypot(...r);
    if (len < 1e-6) r = [1, 0, 0];
    else r = r.map((c) => c / len);
    let u = [f[1] * r[2] - f[2] * r[1], f[2] * r[0] - f[0] * r[2], f[0] * r[1] - f[1] * r[0]]; // forward × right
    const t = (doc.camera.twist || 0) / 4096 * Math.PI / 180;
    if (t) {
      const c = Math.cos(t), s = Math.sin(t);
      const r2 = r.map((_, i) => r[i] * c + u[i] * s);
      u = u.map((_, i) => u[i] * c - r[i] * s);
      r = r2;
    }
    return { from: [fx, fy, fz], f, r, u, h: doc.projection || 320 };
  }
  function toCamera(cam, p) {
    const d = [p[0] - cam.from[0], p[1] - cam.from[1], p[2] - cam.from[2]];
    return [dot(d, cam.r), dot(d, cam.u), dot(d, cam.f)];
  }
  const dot = (a, b) => a[0] * b[0] + a[1] * b[1] + a[2] * b[2];
  function toScreen(cam, c) {
    if (c[2] <= 1) return null;
    return [W / 2 + c[0] * cam.h / c[2], H / 2 + c[1] * cam.h / c[2], c[2]];
  }

  // rotation makes the matrix of an object's rotation, Z then Y then X as
  // RotMatrix applies them, with scale folded in.
  function rotation(rot, scale) {
    const a = rot.map((v) => v / 4096 * 2 * Math.PI);
    const [cx, sx, cy, sy, cz, sz] = [Math.cos(a[0]), Math.sin(a[0]), Math.cos(a[1]), Math.sin(a[1]), Math.cos(a[2]), Math.sin(a[2])];
    const rz = [[cz, -sz, 0], [sz, cz, 0], [0, 0, 1]];
    const ry = [[cy, 0, sy], [0, 1, 0], [-sy, 0, cy]];
    const rx = [[1, 0, 0], [0, cx, -sx], [0, sx, cx]];
    const mul = (p, q) => p.map((row, i) => [0, 1, 2].map((j) => row[0] * q[0][j] + row[1] * q[1][j] + row[2] * q[2][j]));
    const m = mul(rx, mul(ry, rz));
    const s = scale.some((c) => c) ? scale.map((c) => c / 4096) : [1, 1, 1];
    return { m, s };
  }
  const apply = (m, v) => [m[0][0] * v[0] + m[0][1] * v[1] + m[0][2] * v[2], m[1][0] * v[0] + m[1][1] * v[1] + m[1][2] * v[2], m[2][0] * v[0] + m[2][1] * v[1] + m[2][2] * v[2]];

  // Lighting, as GsSetFlatLight and GsSetAmbient light a face: the
  // ambient, and each light by how squarely the face turns against it.
  function lighting() {
    const lights = doc.lights.map((l) => {
      const len = Math.hypot(...l.direction) || 1;
      return { d: l.direction.map((c) => c / len), c: l.color.map((c) => c / 256) };
    });
    const amb = doc.ambient.map((c) => c / 4096);
    return (n) => {
      const out = [amb[0], amb[1], amb[2]];
      for (const l of lights) {
        const k = Math.max(0, -dot(n, l.d));
        if (!k) continue;
        out[0] += l.c[0] * k;
        out[1] += l.c[1] * k;
        out[2] += l.c[2] * k;
      }
      return out;
    };
  }

  function render() {
    if (!shown) return;
    const out = img.data;
    out.fill(0);
    ids.fill(0);
    // The background the scene example clears to.
    for (let i = 0; i < out.length; i += 4) {
      out[i] = 40;
      out[i + 1] = 60;
      out[i + 2] = 100;
      out[i + 3] = 255;
    }
    if (!doc) {
      ctx.putImageData(img, 0, 0);
      status.textContent = error || "No scene.";
      return;
    }
    const cam = camera();
    const light = lighting();
    const v = vram();

    // The ground, a grid on the plane of the camera's target, for the
    // sense of where things stand; a tile map covers it.
    if (doc.background) {
      drawBackground(out, v);
    } else {
      const gy = doc.camera.at[1];
      for (let k = -8; k <= 8; k++) {
        gridLine(cam, [k * 128, gy, -1024], [k * 128, gy, 1024]);
        gridLine(cam, [-1024, gy, k * 128], [1024, gy, k * 128]);
      }
    }

    const tris = [];
    let missing = 0;
    let polygons = 0;
    const untextured = new Set();
    problems.clear();
    doc.objects.forEach((o, i) => {
      if (o.hidden) return;
      const tmd = tmdFor(o.model);
      const obj = tmd?.objects[o.index];
      if (!obj) {
        // No model to draw: a box of its own color marks the place.
        missing++;
        problems.set(i, tmd ? `${o.model.replace(/^ASSETS\//, "")} has no object ${o.index}` : `${o.model.replace(/^ASSETS\//, "")} is not in assets/`);
        drawBox(cam, o.position, 48, i + 1, [255, 64, 255]);
        return;
      }
      polygons += preview.polygons(obj).filter((q) => !q.line).length;
      const { m, s } = rotation(o.rotation, o.scale);
      const nv = obj.vertices.length / 3;
      const sv = new Float32Array(nv * 3);
      const seen = new Uint8Array(nv);
      for (let k = 0; k < nv; k++) {
        const local = [obj.vertices[3 * k] * s[0], obj.vertices[3 * k + 1] * s[1], obj.vertices[3 * k + 2] * s[2]];
        const w = apply(m, local);
        const c = toCamera(cam, [w[0] + o.position[0], w[1] + o.position[1], w[2] + o.position[2]]);
        const p = toScreen(cam, c);
        if (!p) continue;
        seen[k] = 1;
        sv[3 * k] = p[0];
        sv[3 * k + 1] = p[1];
        sv[3 * k + 2] = p[2];
      }
      const nn = obj.normals.length / 3;
      const shade = [];
      for (let k = 0; k < nn; k++) {
        const n = apply(m, [obj.normals[3 * k] / 4096, obj.normals[3 * k + 1] / 4096, obj.normals[3 * k + 2] / 4096]);
        shade.push(light(n));
      }
      for (const p of preview.polygons(obj)) {
        if (!p.verts.every((k) => k < nv && seen[k]) || (p.normals && !p.normals.every((k) => k < nn))) continue;
        const z = p.verts.reduce((acc, k) => acc + sv[3 * k + 2], 0) / p.verts.length;
        if (p.line) {
          tris.push({ p, sv, idx: [0, 1], z, id: i + 1, shade });
          continue;
        }
        tris.push({ p, sv, idx: [0, 1, 2], z, id: i + 1, shade });
        if (p.verts.length === 4) tris.push({ p, sv, idx: [1, 3, 2], z, id: i + 1, shade });
      }
    });
    tris.sort((a, b) => b.z - a.z);
    for (const t of tris) {
      const { p, sv, idx, id, shade } = t;
      const vv = idx.map((k) => p.verts[k]);
      if (p.line) {
        preview.drawLine(out, ids, id, sv[3 * vv[0]], sv[3 * vv[0] + 1], sv[3 * vv[1]], sv[3 * vv[1] + 1], p.colors[0], p.colors[1]);
        continue;
      }
      const x0 = sv[3 * vv[0]], y0 = sv[3 * vv[0] + 1];
      const x1 = sv[3 * vv[1]], y1 = sv[3 * vv[1] + 1];
      const x2 = sv[3 * vv[2]], y2 = sv[3 * vv[2] + 1];
      const area = (x1 - x0) * (y2 - y0) - (y1 - y0) * (x2 - x0);
      if (area === 0 || (area < 0 && !p.double)) continue;
      const cols = idx.map((k) => p.colors[k]);
      const shades = p.normals ? idx.map((k) => shade[p.normals[k]]) : [[1, 1, 1], [1, 1, 1], [1, 1, 1]];
      // A texture page with no TIM on it would draw nothing, as the
      // console skips empty texels; here the model shows untextured
      // instead, so that it can still be seen and picked.
      let uvs = null;
      let texels = v;
      if (p.texture) {
        uvs = idx.map((k) => p.texture.uv[k]);
        if (!pageHasTexture(v, p.texture.tpage)) {
          // Nothing on the page: the checker every editor shows for a
          // missing texture, so that it is seen and named.
          texels = checker;
          untextured.add(id - 1);
          const tp = p.texture.tpage;
          problems.set(id - 1, `no picture on texture page 0x${tp.toString(16)} (${(tp & 0xf) * 64}, ${(tp >> 4 & 1) * 256}); move one there in Files, Video memory, or convert it again`);
        }
      }
      preview.fillTriangle(out, ids, id, x0, y0, x1, y1, x2, y2, area, cols, shades, uvs, p, texels);
    }
    doc.sprites.forEach((sp, j) => preview.drawSprite(out, ids, -(j + 1), v, sp, W / 2, H / 2));
    drawLights(cam);
    if (showConsole && consoleFrame) {
      ctx.drawImage(consoleFrame, 0, 0, W, H);
    } else {
      ctx.putImageData(img, 0, 0);
    }
    drawGizmo(cam);
    const n = doc.objects.length, ns = doc.sprites.length;
    const notes = [];
    if (missing) notes.push(`${missing} without a model`);
    if (untextured.size) notes.push(`${untextured.size} without a texture`);
    const first = [...problems.values()][0];
    status.textContent = typing ? typingStatus() : `${n} object${n === 1 ? "" : "s"}, ${ns} sprite${ns === 1 ? "" : "s"}, ${polygons} polygon${polygons === 1 ? "" : "s"} a frame${notes.length ? ", " + notes.join(", ") : ""}.${first ? ` ${first[0].toUpperCase()}${first.slice(1)}.` : ""} ${modeHelp()}`;
  }
  function modeHelp() {
    const m = gizmoMode === "move" ? "Drag an axis or a square to move, elsewhere to move freely, with Shift for depth" : gizmoMode === "rotate" ? "Drag a ring to turn about its axis" : "Drag a cube to scale along its axis, the middle to scale all";
    return `${m}; W, E, R pick move, rotate, scale; G then a number types a move (R or S for a turn or a scale, X, Y, Z for an axis); right drag looks around with WASD, Alt drag orbits, middle or Ctrl drag pans, wheel approaches, F frames what is picked.`;
  }

  // The checker a model shows where its texture should be.
  const checker = { texel: (tpage, cba, u, v) => (((u >> 3) + (v >> 3)) & 1 ? 0x7c1f : 0x8000) };

  // drawBox draws a wire box of half-size r about a point, pickable as id.
  function drawBox(cam, p, r, id, color) {
    const corner = (sx, sy, sz) => toScreen(cam, toCamera(cam, [p[0] + sx * r, p[1] + sy * r, p[2] + sz * r]));
    const c = [];
    for (const sx of [-1, 1]) for (const sy of [-1, 1]) for (const sz of [-1, 1]) c.push(corner(sx, sy, sz));
    const edges = [[0, 1], [2, 3], [4, 5], [6, 7], [0, 2], [1, 3], [4, 6], [5, 7], [0, 4], [1, 5], [2, 6], [3, 7]];
    for (const [a, b] of edges) {
      if (!c[a] || !c[b]) continue;
      preview.drawLine(img.data, ids, id, c[a][0], c[a][1], c[b][0], c[b][1], color, color);
    }
  }

  // drawLights draws each light as an arrow flying in at the camera's
  // target, in the light's color.
  function drawLights(cam) {
    const at = doc.camera.at;
    doc.lights.forEach((l) => {
      const len = Math.hypot(...l.direction) || 1;
      const d = l.direction.map((c) => c / len);
      const tail = [0, 1, 2].map((i) => at[i] - d[i] * 220);
      const head = [0, 1, 2].map((i) => at[i] - d[i] * 40);
      const a = toScreen(cam, toCamera(cam, tail)), b = toScreen(cam, toCamera(cam, head));
      if (!a || !b) return;
      const col = l.color.map((c) => Math.min(255, c + 40));
      preview.drawLine(img.data, null, 0, a[0], a[1], b[0], b[1], col, col);
      // The head: two short strokes back from the tip.
      const ang = Math.atan2(b[1] - a[1], b[0] - a[0]);
      for (const k of [-1, 1]) {
        preview.drawLine(img.data, null, 0, b[0], b[1], b[0] - 6 * Math.cos(ang + k * 0.5), b[1] - 6 * Math.sin(ang + k * 0.5), col, col);
      }
    });
  }

  // pageHasTexture says whether any TIM's pixels lie in a texture page.
  const pageCache = new Map();
  function pageHasTexture(v, tpage) {
    const key = tpage & 0x1f;
    if (pageCache.has(key) && pageCache.get(key).v === v) return pageCache.get(key).has;
    const px = (tpage & 0xf) * 64, py = (tpage >> 4 & 1) * 256;
    const has = v.placed.some(({ block, kind }) => kind === "pixels" && block.x < px + 64 && px < block.x + block.w && block.y < py + 256 && py < block.y + block.h);
    pageCache.set(key, { v, has });
    return has;
  }

  // The tile map, as GsSortFixBg16 draws it: the window on the screen,
  // the map scrolled within it and repeating past its edge, each cell cut
  // from the texture in reading order.
  function tileCells(bg) {
    const tim = timFor(bg.texture);
    if (!tim) return null;
    const [u0, v0] = preview.uv(tim);
    const across = Math.max(1, Math.floor(tim.width / 16));
    const count = Math.min(256, across * Math.floor(tim.height / 16));
    return { tim, u0, v0, across, count, tpage: preview.tpage(tim), cba: preview.cba(tim) };
  }
  function drawBackground(out, v) {
    const bg = doc.background;
    const cells = tileCells(bg);
    if (!cells) return;
    const win = bg.window.some((c) => c) ? bg.window : [-160, -120, 320, 240];
    const left = W / 2 + win[0], top = H / 2 + win[1];
    const mw = bg.width * 16, mh = bg.height * 16;
    const [br, bgg, bb] = bg.brightness;
    for (let y = 0; y < win[3]; y++) {
      const py = top + y;
      if (py < 0 || py >= H) continue;
      const my = (((bg.scroll[1] + y) % mh) + mh) % mh;
      for (let x = 0; x < win[2]; x++) {
        const px = left + x;
        if (px < 0 || px >= W) continue;
        const mx = (((bg.scroll[0] + x) % mw) + mw) % mw;
        const cell = bg.map[(my >> 4) * bg.width + (mx >> 4)];
        if (cell === 0xffff || cell >= cells.count) continue;
        const c = v.texel(cells.tpage, cells.cba, (cells.u0 + (cell % cells.across) * 16 + (mx & 15)) & 255, (cells.v0 + Math.floor(cell / cells.across) * 16 + (my & 15)) & 255);
        if (c === 0) continue;
        const i = (py * W + px) * 4;
        const r = (c & 31) * 255 / 31 * br / 128, g = (c >> 5 & 31) * 255 / 31 * bgg / 128, b = (c >> 10 & 31) * 255 / 31 * bb / 128;
        out[i] = r > 255 ? 255 : r;
        out[i + 1] = g > 255 ? 255 : g;
        out[i + 2] = b > 255 ? 255 : b;
      }
    }
  }

  // The tile map's panel: its texture and size, the cells to paint with,
  // and the map itself, painted by clicking and dragging; Shift empties a
  // cell. The window the screen shows is outlined on the map.
  const tilesForm = $("scene-tiles");
  const palette = $("scene-tile-palette");
  const mapCanvas = $("scene-tile-map");
  const tilesInfo = $("scene-tiles-info");
  const tileTools = $("scene-tile-tools");
  let brush = { w: 1, h: 1, cells: [0] }; // the stamp: cells laid together
  let lastLaid = null; // the last cell the pen laid, for Shift lines
  let tool = "pen"; // pen, fill, rect or erase
  let ghost = null; // the map cell under the pointer, or the rectangle being drawn
  for (const b of tileTools.querySelectorAll("button")) b.addEventListener("click", () => setTool(b.dataset.tool));
  function renderTiles() {
    tilesForm.replaceChildren();
    if (!doc) {
      palette.hidden = mapCanvas.hidden = true;
      return;
    }
    const bg = doc.background;
    const tims = dataFiles(/\.tim$/i).map(discPath);
    tilesForm.append(field("Cells from", bg?.texture ?? "", (v) => {
      if (!v) doc.background = null;
      else if (!bg) doc.background = { texture: v, cell: 16, width: 20, height: 15, map: new Array(300).fill(0), scroll: [0, 0], window: [0, 0, 0, 0], brightness: [128, 128, 128] };
      else bg.texture = v;
      brush = { w: 1, h: 1, cells: [0] };
      changed(true);
      renderTiles();
    }, { options: [["", "no tile map"], ...tims.map((p) => [p, p.replace(/^ASSETS\//, "")])], title: "The TIM whose 16 by 16 cells the map is made of; GsSortFixBg16 draws it behind everything" }));
    if (!bg) {
      palette.hidden = mapCanvas.hidden = true;
      tileTools.hidden = true;
      tilesInfo.textContent = "A tile map is a picture cut into 16 by 16 cells, laid out in a grid and scrolled behind the scene, as the tiles example does. Pick the picture to start one.";
      return;
    }
    const resize = (w, h) => {
      const map = new Array(w * h).fill(0);
      for (let y = 0; y < Math.min(h, bg.height); y++) for (let x = 0; x < Math.min(w, bg.width); x++) map[y * w + x] = bg.map[y * bg.width + x];
      bg.width = w;
      bg.height = h;
      bg.map = map;
    };
    tilesForm.append(group("Map", [
      field("Across", bg.width, (v) => { resize(Math.max(1, Math.min(64, Math.round(v))), bg.height); changed(true); renderTiles(); }, { title: "Cells across, up to 64, with the map at most 4096 cells" }),
      field("Down", bg.height, (v) => { resize(bg.width, Math.max(1, Math.min(64, Math.round(v)))); changed(true); renderTiles(); }, { title: "Cells down" }),
    ]));
    tilesForm.append(group("Scroll", ["X", "Y"].map((l, i) => field(l, bg.scroll[i], (v) => { bg.scroll[i] = Math.round(v); changed(); renderMap(); }, { title: "How far the map is scrolled within the window, in pixels; the program changes it as it likes", step: 16 }))));
    tilesForm.append(group("Brightness", ["R", "G", "B"].map((l, i) => field(l, bg.brightness[i], (v) => { bg.brightness[i] = Math.max(0, Math.min(255, Math.round(v))); changed(); renderMap(); }, { title: "128 shows the cells as they are" }))));
    palette.hidden = mapCanvas.hidden = false;
    tileTools.hidden = false;
    renderPalette();
    renderMap();
  }
  function renderPalette() {
    const bg = doc?.background;
    const cells = bg && tileCells(bg);
    if (!cells) {
      palette.hidden = true;
      return;
    }
    const v = vram();
    const rows = Math.ceil(cells.count / cells.across);
    palette.width = cells.across * 16;
    palette.height = rows * 16;
    const pctx = palette.getContext("2d");
    const img = pctx.createImageData(palette.width, palette.height);
    for (let y = 0; y < palette.height; y++) {
      for (let x = 0; x < palette.width; x++) {
        const c = v.texel(cells.tpage, cells.cba, (cells.u0 + x) & 255, (cells.v0 + y) & 255);
        const i = (y * palette.width + x) * 4;
        if (c === 0) continue;
        img.data[i] = (c & 31) * 255 / 31;
        img.data[i + 1] = (c >> 5 & 31) * 255 / 31;
        img.data[i + 2] = (c >> 10 & 31) * 255 / 31;
        img.data[i + 3] = 255;
      }
    }
    pctx.putImageData(img, 0, 0);
    pctx.strokeStyle = "rgba(255,255,255,0.25)";
    for (let x = 16; x < palette.width; x += 16) { pctx.beginPath(); pctx.moveTo(x + 0.5, 0); pctx.lineTo(x + 0.5, palette.height); pctx.stroke(); }
    for (let y = 16; y < palette.height; y += 16) { pctx.beginPath(); pctx.moveTo(0, y + 0.5); pctx.lineTo(palette.width, y + 0.5); pctx.stroke(); }
    pctx.strokeStyle = themeColor("--accent");
    pctx.lineWidth = 2;
    pctx.strokeRect((brush.cells[0] % cells.across) * 16 + 1, Math.floor(brush.cells[0] / cells.across) * 16 + 1, brush.w * 16 - 2, brush.h * 16 - 2);
    // Doubled while it stays short; a tall picture of cells at its own size.
    palette.style.width = `${palette.width * (palette.height <= 128 ? 2 : 1)}px`;
    const stamp = brush.cells.length === 1 ? `Cell ${brush.cells[0]}` : `A stamp of ${brush.w} by ${brush.h} cells`;
    tilesInfo.textContent = `${cells.count} cells from ${bg.texture.replace(/^ASSETS\//, "")}, ${cells.across} across. ${stamp} is picked; drag across several to pick a stamp. The pen (B) lays it where you click or drag, with Shift a line from the last cell laid; the fill (F) floods cells like the one clicked; the rectangle (R) fills what you drag over; the eraser (E) empties cells. Alt picks the cell under the pointer. The box is the window the screen shows.`;
  }
  function renderMap() {
    const bg = doc?.background;
    const cells = bg && tileCells(bg);
    if (!cells) return;
    const v = vram();
    mapCanvas.width = bg.width * 16;
    mapCanvas.height = bg.height * 16;
    const mctx = mapCanvas.getContext("2d");
    const img = mctx.createImageData(mapCanvas.width, mapCanvas.height);
    const [br, bgg, bb] = bg.brightness;
    for (let y = 0; y < mapCanvas.height; y++) {
      for (let x = 0; x < mapCanvas.width; x++) {
        const cell = bg.map[(y >> 4) * bg.width + (x >> 4)];
        const i = (y * mapCanvas.width + x) * 4;
        if (cell === 0xffff || cell >= cells.count) {
          // An empty cell shows the screen's clear colour, dimmed.
          img.data[i] = 20; img.data[i + 1] = 30; img.data[i + 2] = 50; img.data[i + 3] = 255;
          continue;
        }
        const c = v.texel(cells.tpage, cells.cba, (cells.u0 + (cell % cells.across) * 16 + (x & 15)) & 255, (cells.v0 + Math.floor(cell / cells.across) * 16 + (y & 15)) & 255);
        if (c === 0) { img.data[i + 3] = 0; continue; }
        img.data[i] = Math.min(255, (c & 31) * 255 / 31 * br / 128);
        img.data[i + 1] = Math.min(255, (c >> 5 & 31) * 255 / 31 * bgg / 128);
        img.data[i + 2] = Math.min(255, (c >> 10 & 31) * 255 / 31 * bb / 128);
        img.data[i + 3] = 255;
      }
    }
    mctx.putImageData(img, 0, 0);
    const win = bg.window.some((c) => c) ? bg.window : [-160, -120, 320, 240];
    mctx.strokeStyle = themeColor("--accent");
    mctx.lineWidth = 2;
    mctx.strokeRect(bg.scroll[0] + 1, bg.scroll[1] + 1, win[2] - 2, win[3] - 2);
    // The brush under the pointer, or the rectangle being drawn.
    if (ghost) {
      mctx.globalAlpha = 0.6;
      if (ghost.rect) {
        const [x0, y0, x1, y1] = ghost.rect;
        for (let y = Math.min(y0, y1); y <= Math.max(y0, y1); y++) for (let x = Math.min(x0, x1); x <= Math.max(x0, x1); x++) drawCell(mctx, cells, v, x, y, stampCell(ghost.value, x - Math.min(x0, x1), y - Math.min(y0, y1)));
      } else {
        for (let dy = 0; dy < ghost.value.h; dy++) for (let dx = 0; dx < ghost.value.w; dx++) if (ghost.x + dx < bg.width && ghost.y + dy < bg.height) drawCell(mctx, cells, v, ghost.x + dx, ghost.y + dy, stampCell(ghost.value, dx, dy));
      }
      mctx.globalAlpha = 1;
      mctx.strokeStyle = themeColor("--ink");
      mctx.lineWidth = 1;
      if (ghost.rect) {
        const [x0, y0, x1, y1] = ghost.rect;
        mctx.strokeRect(Math.min(x0, x1) * 16 + 0.5, Math.min(y0, y1) * 16 + 0.5, (Math.abs(x1 - x0) + 1) * 16 - 1, (Math.abs(y1 - y0) + 1) * 16 - 1);
      } else {
        mctx.strokeRect(ghost.x * 16 + 0.5, ghost.y * 16 + 0.5, ghost.value.w * 16 - 1, ghost.value.h * 16 - 1);
      }
    }
    mapCanvas.style.width = `${mapCanvas.width}px`;
  }
  // drawCell paints one cell of the map canvas with a cell index, or the
  // empty color.
  const cellCanvas = document.createElement("canvas");
  function drawCell(mctx, cells, v, x, y, value) {
    cellCanvas.width = cellCanvas.height = 16;
    const cctx = cellCanvas.getContext("2d");
    const img = cctx.createImageData(16, 16);
    for (let py = 0; py < 16; py++) {
      for (let px = 0; px < 16; px++) {
        const i = (py * 16 + px) * 4;
        if (value === 0xffff || value >= cells.count) {
          img.data[i] = 20; img.data[i + 1] = 30; img.data[i + 2] = 50; img.data[i + 3] = 255;
          continue;
        }
        const c = v.texel(cells.tpage, cells.cba, (cells.u0 + (value % cells.across) * 16 + px) & 255, (cells.v0 + Math.floor(value / cells.across) * 16 + py) & 255);
        if (c === 0) continue;
        img.data[i] = (c & 31) * 255 / 31;
        img.data[i + 1] = (c >> 5 & 31) * 255 / 31;
        img.data[i + 2] = (c >> 10 & 31) * 255 / 31;
        img.data[i + 3] = 255;
      }
    }
    cctx.putImageData(img, 0, 0);
    mctx.drawImage(cellCanvas, x * 16, y * 16);
  }
  // stampCell is the cell a stamp lays at dx, dy within it; the eraser's
  // stamp is one empty cell.
  const ERASER = { w: 1, h: 1, cells: [0xffff] };
  function stampCell(stamp, dx, dy) {
    return stamp.cells[(dy % stamp.h) * stamp.w + (dx % stamp.w)];
  }
  function paletteCell(e) {
    const r = palette.getBoundingClientRect();
    return [Math.floor((e.clientX - r.left) * palette.width / r.width / 16), Math.floor((e.clientY - r.top) * palette.height / r.height / 16)];
  }
  // A click picks a cell; a drag picks a stamp of several.
  palette.addEventListener("pointerdown", (e) => {
    const bg = doc?.background;
    const cells = bg && tileCells(bg);
    if (!cells) return;
    e.preventDefault();
    palette.setPointerCapture(e.pointerId);
    const [sx, sy] = paletteCell(e);
    const pick = (ex, ey) => {
      const x0 = Math.max(0, Math.min(sx, ex)), y0 = Math.max(0, Math.min(sy, ey));
      const x1 = Math.min(cells.across - 1, Math.max(sx, ex)), y1 = Math.max(y0, Math.min(Math.ceil(cells.count / cells.across) - 1, Math.max(sy, ey)));
      const picked = [];
      for (let y = y0; y <= y1; y++) for (let x = x0; x <= x1; x++) picked.push(Math.min(cells.count - 1, y * cells.across + x));
      brush = { w: x1 - x0 + 1, h: y1 - y0 + 1, cells: picked };
      if (tool === "erase") setTool("pen");
      renderPalette();
    };
    pick(sx, sy);
    const move = (ev) => { const [x, y] = paletteCell(ev); pick(x, y); };
    const up = () => {
      palette.removeEventListener("pointermove", move);
      palette.removeEventListener("pointerup", up);
    };
    palette.addEventListener("pointermove", move);
    palette.addEventListener("pointerup", up);
  });
  function setTool(t) {
    tool = t;
    for (const other of tileTools.querySelectorAll("button")) other.setAttribute("aria-pressed", String(other.dataset.tool === t));
  }
  mapCanvas.addEventListener("keydown", (e) => {
    const k = e.key.toLowerCase();
    if (e.ctrlKey || e.metaKey) {
      if (k === "z") { e.preventDefault(); e.shiftKey ? redo() : undo(); }
      else if (k === "y") { e.preventDefault(); redo(); }
      return;
    }
    if (k === "b") setTool("pen");
    else if (k === "f") setTool("fill");
    else if (k === "r") setTool("rect");
    else if (k === "e") setTool("erase");
  });
  function mapCell(ev) {
    const r = mapCanvas.getBoundingClientRect();
    return [Math.floor((ev.clientX - r.left) * mapCanvas.width / r.width / 16), Math.floor((ev.clientY - r.top) * mapCanvas.height / r.height / 16)];
  }
  mapCanvas.addEventListener("pointermove", (e) => {
    const bg = doc?.background;
    if (!bg || mapDragging) return;
    const [x, y] = mapCell(e);
    const inside = x >= 0 && y >= 0 && x < bg.width && y < bg.height;
    const next = inside ? { x, y, value: tool === "erase" ? ERASER : brush } : null;
    if (JSON.stringify(next) === JSON.stringify(ghost)) return;
    ghost = next;
    renderMap();
  });
  mapCanvas.addEventListener("pointerleave", () => {
    if (ghost && !mapDragging) {
      ghost = null;
      renderMap();
    }
  });
  let mapDragging = false;
  mapCanvas.addEventListener("pointerdown", (e) => {
    const bg = doc?.background;
    if (!bg) return;
    e.preventDefault();
    const [sx, sy] = mapCell(e);
    if (sx < 0 || sy < 0 || sx >= bg.width || sy >= bg.height) return;
    if (e.altKey) {
      // Pick the cell under the pointer as the brush.
      const under = bg.map[sy * bg.width + sx];
      if (under !== 0xffff) {
        brush = { w: 1, h: 1, cells: [under] };
        setTool("pen");
        renderPalette();
      }
      return;
    }
    mapCanvas.setPointerCapture(e.pointerId);
    mapCanvas.focus({ preventScroll: true });
    mapDragging = true;
    beginEdit();
    const stamp = tool === "erase" ? ERASER : brush;
    // set lays the stamp with its corner at x, y; lay lays one cell of it
    // as the pen passes, keeping the stamp's pattern aligned to the map.
    const setOne = (x, y, value) => {
      if (x < 0 || y < 0 || x >= bg.width || y >= bg.height || bg.map[y * bg.width + x] === value) return false;
      bg.map[y * bg.width + x] = value;
      return true;
    };
    const set = (x, y) => {
      let any = false;
      for (let dy = 0; dy < stamp.h; dy++) for (let dx = 0; dx < stamp.w; dx++) any = setOne(x + dx, y + dy, stampCell(stamp, dx, dy)) || any;
      return any;
    };
    const line = (x0, y0, x1, y1) => {
      // Bresenham, the pen between two cells.
      let any = false;
      const dx = Math.abs(x1 - x0), dy = -Math.abs(y1 - y0);
      const sx = x0 < x1 ? 1 : -1, sy = y0 < y1 ? 1 : -1;
      let err = dx + dy, x = x0, y = y0;
      for (;;) {
        any = set(x, y) || any;
        if (x === x1 && y === y1) break;
        const e2 = 2 * err;
        if (e2 >= dy) { err += dy; x += sx; }
        if (e2 <= dx) { err += dx; y += sy; }
      }
      return any;
    };
    if (tool === "fill") {
      // Flood the cells like the one clicked, four ways, with the stamp's
      // pattern laid across them.
      const was = bg.map[sy * bg.width + sx];
      const value0 = stampCell(stamp, 0, 0);
      if (!(stamp.w === 1 && stamp.h === 1 && was === value0)) {
        const stack = [[sx, sy]];
        const seen = new Set();
        while (stack.length) {
          const [x, y] = stack.pop();
          const key = y * bg.width + x;
          if (x < 0 || y < 0 || x >= bg.width || y >= bg.height || seen.has(key) || bg.map[key] !== was) continue;
          seen.add(key);
          bg.map[key] = stampCell(stamp, x - sx, y - sy);
          stack.push([x + 1, y], [x - 1, y], [x, y + 1], [x, y - 1]);
        }
        changed();
      }
      renderMap();
    } else if (tool === "rect") {
      ghost = { rect: [sx, sy, sx, sy], value: stamp };
      renderMap();
    } else {
      const any = e.shiftKey && lastLaid ? line(lastLaid[0], lastLaid[1], sx, sy) : set(sx, sy);
      lastLaid = [sx, sy];
      if (any) changed();
      renderMap();
    }
    const move = (ev) => {
      const [x, y] = mapCell(ev);
      if (x < 0 || y < 0 || x >= bg.width || y >= bg.height) return;
      if (tool === "rect") {
        ghost = { rect: [sx, sy, x, y], value: stamp };
        renderMap();
      } else if (tool !== "fill") {
        const any = lastLaid ? line(lastLaid[0], lastLaid[1], x, y) : set(x, y);
        lastLaid = [x, y];
        ghost = { x, y, value: stamp };
        if (any) changed();
        renderMap();
      }
    };
    const up = () => {
      mapCanvas.removeEventListener("pointermove", move);
      mapCanvas.removeEventListener("pointerup", up);
      if (tool === "rect" && ghost?.rect) {
        const [x0, y0, x1, y1] = ghost.rect;
        const ox = Math.min(x0, x1), oy = Math.min(y0, y1);
        let any = false;
        for (let y = oy; y <= Math.max(y0, y1); y++) for (let x = ox; x <= Math.max(x0, x1); x++) any = setOne(x, y, stampCell(stamp, x - ox, y - oy)) || any;
        if (any) changed();
      }
      ghost = null;
      mapDragging = false;
      endEdit();
      renderMap();
    };
    mapCanvas.addEventListener("pointermove", move);
    mapCanvas.addEventListener("pointerup", up);
  });

  function gridLine(cam, a, b) {
    const ca = toCamera(cam, a), cb = toCamera(cam, b);
    // Clip to in front of the camera.
    const near = 16;
    if (ca[2] < near && cb[2] < near) return;
    const clip = (p, q) => {
      const t = (near - p[2]) / (q[2] - p[2]);
      return [p[0] + (q[0] - p[0]) * t, p[1] + (q[1] - p[1]) * t, near];
    };
    const pa = ca[2] < near ? clip(ca, cb) : ca;
    const pb = cb[2] < near ? clip(cb, ca) : cb;
    const sa = toScreen(cam, pa), sb = toScreen(cam, pb);
    if (!sa || !sb) return;
    preview.drawLine(img.data, null, 0, sa[0], sa[1], sb[0], sb[1], [60, 82, 124], [60, 82, 124]);
  }

  // The gizmo: the selected object's axes in the pad's colors, X circle
  // red, Y triangle green (downward, as the console's Y goes), Z cross
  // blue; a selected sprite's outline.
  // themeColor resolves one of the page's color tokens, which hold a
  // light and a dark value, to the one in force.
  const swatch = document.createElement("span");
  swatch.hidden = true;
  document.body.append(swatch);
  function themeColor(token) {
    swatch.style.color = `var(${token})`;
    return getComputedStyle(swatch).color;
  }

  // The gizmo's geometry on the screen, for the mode in force. Move: the
  // three axes, 96 units long, and a square a third of the way out in
  // each plane; rotate: a ring about each axis, 48 units out; scale: the
  // axes with a cube at each end and a square at the middle. Each handle
  // carries what the hit test needs: a segment, a point or a ring.
  const AXIS_LEN = 96;
  const RING = 48;
  function gizmoGeometry(cam) {
    if (selected?.kind !== "object") return null;
    const o = doc.objects[selected.index];
    if (!o) return null;
    const P = (dx, dy, dz) => toScreen(cam, toCamera(cam, [o.position[0] + dx, o.position[1] + dy, o.position[2] + dz]));
    const origin = P(0, 0, 0);
    if (!origin) return null;
    const unit = [[1, 0, 0], [0, 1, 0], [0, 0, 1]];
    const axes = unit.map((a) => P(a[0] * AXIS_LEN, a[1] * AXIS_LEN, a[2] * AXIS_LEN));
    const handles = [];
    if (gizmoMode === "move" || gizmoMode === "scale") {
      axes.forEach((end, i) => { if (end) handles.push({ kind: gizmoMode === "move" ? "axis" : "scale", axis: i, seg: [origin, end], point: end }); });
    }
    if (gizmoMode === "move") {
      for (const [a, b] of [[0, 1], [0, 2], [1, 2]]) {
        const t = AXIS_LEN / 3;
        const corners = [[t, 0], [t, t], [0, t]].map(([ka, kb]) => P(unit[a][0] * ka + unit[b][0] * kb, unit[a][1] * ka + unit[b][1] * kb, unit[a][2] * ka + unit[b][2] * kb));
        const center = P(unit[a][0] * t / 2 + unit[b][0] * t / 2, unit[a][1] * t / 2 + unit[b][1] * t / 2, unit[a][2] * t / 2 + unit[b][2] * t / 2);
        if (corners.every(Boolean) && center) handles.push({ kind: "plane", axes: [a, b], poly: [origin, ...corners], point: center });
      }
    }
    if (gizmoMode === "rotate") {
      unit.forEach((axis, i) => {
        const u = unit[(i + 1) % 3], w = unit[(i + 2) % 3];
        const ring = [];
        for (let k = 0; k < 32; k++) {
          const a = k / 32 * 2 * Math.PI;
          ring.push(P((u[0] * Math.cos(a) + w[0] * Math.sin(a)) * RING, (u[1] * Math.cos(a) + w[1] * Math.sin(a)) * RING, (u[2] * Math.cos(a) + w[2] * Math.sin(a)) * RING));
        }
        handles.push({ kind: "ring", axis: i, ring });
      });
    }
    if (gizmoMode === "scale") handles.push({ kind: "uniform", point: origin });
    return { origin, axes, handles };
  }
  let hovered = null; // the handle under the pointer, or null
  function handleAt(x, y, g) {
    if (!g) return null;
    const segDist = (a, b) => {
      if (!a || !b) return Infinity;
      const dx = b[0] - a[0], dy = b[1] - a[1];
      const len2 = dx * dx + dy * dy || 1;
      const t = Math.max(0, Math.min(1, ((x - a[0]) * dx + (y - a[1]) * dy) / len2));
      return Math.hypot(x - a[0] - dx * t, y - a[1] - dy * t);
    };
    // Points first, then rings, then axes, so that an end is not an axis.
    for (const h of g.handles) if ((h.kind === "scale" || h.kind === "uniform" || h.kind === "plane") && Math.hypot(x - h.point[0], y - h.point[1]) < 7) return h;
    for (const h of g.handles) if (h.kind === "ring") for (let k = 0; k < h.ring.length; k++) if (segDist(h.ring[k], h.ring[(k + 1) % h.ring.length]) < 4) return h;
    for (const h of g.handles) if ((h.kind === "axis" || h.kind === "scale") && segDist(h.seg[0], h.seg[1]) < 5) return h.kind === "scale" ? { ...h, kind: "axis" } : h;
    return null;
  }
  const sameHandle = (a, b) => (a ? `${a.kind}${a.axis ?? a.axes?.join("") ?? ""}` : "") === (b ? `${b.kind}${b.axis ?? b.axes?.join("") ?? ""}` : "");

  function drawGizmo(cam) {
    if (!selected) return;
    const colors = [themeColor("--circle"), themeColor("--triangle"), themeColor("--cross")];
    ctx.lineWidth = 1;
    if (selected.kind === "sprite") {
      const sp = doc.sprites[selected.index];
      if (!sp) return;
      ctx.strokeStyle = themeColor("--accent");
      ctx.strokeRect(W / 2 + sp.x + 0.5, H / 2 + sp.y + 0.5, sp.w - 1, sp.h - 1);
      return;
    }
    const g = gizmoGeometry(cam);
    if (!g) return;
    const active = dragging?.handle ?? hovered;
    const ink = themeColor("--ink");
    for (const h of g.handles) {
      const on = sameHandle(h, active);
      if (h.kind === "ring") {
        ctx.strokeStyle = colors[h.axis];
        ctx.globalAlpha = on ? 1 : 0.7;
        ctx.lineWidth = on ? 2 : 1;
        ctx.beginPath();
        h.ring.forEach((p, k) => { if (p) (k ? ctx.lineTo(p[0], p[1]) : ctx.moveTo(p[0], p[1])); });
        ctx.closePath();
        ctx.stroke();
        ctx.globalAlpha = 1;
      } else if (h.kind === "plane") {
        ctx.fillStyle = colors[3 - h.axes[0] - h.axes[1]];
        ctx.globalAlpha = on ? 0.75 : 0.35;
        ctx.beginPath();
        h.poly.forEach((p, k) => (k ? ctx.lineTo(p[0], p[1]) : ctx.moveTo(p[0], p[1])));
        ctx.closePath();
        ctx.fill();
        ctx.globalAlpha = 1;
      } else if (h.kind === "axis" || h.kind === "scale") {
        const i = h.axis;
        ctx.strokeStyle = colors[i];
        ctx.lineWidth = on ? 3 : 1;
        ctx.beginPath();
        ctx.moveTo(g.origin[0], g.origin[1]);
        ctx.lineTo(h.point[0], h.point[1]);
        ctx.stroke();
        ctx.fillStyle = colors[i];
        const size = h.kind === "scale" ? 7 : 4;
        if (h.kind === "scale") ctx.fillRect(Math.round(h.point[0]) - 3, Math.round(h.point[1]) - 3, size, size);
        else {
          // An arrow head.
          const ang = Math.atan2(h.point[1] - g.origin[1], h.point[0] - g.origin[0]);
          ctx.beginPath();
          ctx.moveTo(h.point[0], h.point[1]);
          ctx.lineTo(h.point[0] - 7 * Math.cos(ang - 0.4), h.point[1] - 7 * Math.sin(ang - 0.4));
          ctx.lineTo(h.point[0] - 7 * Math.cos(ang + 0.4), h.point[1] - 7 * Math.sin(ang + 0.4));
          ctx.closePath();
          ctx.fill();
        }
      } else if (h.kind === "uniform") {
        ctx.strokeStyle = ink;
        ctx.lineWidth = on ? 2 : 1;
        ctx.strokeRect(Math.round(h.point[0]) - 4.5, Math.round(h.point[1]) - 4.5, 9, 9);
      }
    }
    ctx.lineWidth = 1;
    if (gizmoMode !== "scale") {
      ctx.strokeStyle = ink;
      ctx.strokeRect(Math.round(g.origin[0]) - 2.5, Math.round(g.origin[1]) - 2.5, 5, 5);
    }
  }

  // Picking and dragging.
  function canvasPoint(e) {
    const r = canvas.getBoundingClientRect();
    return [(e.clientX - r.left) * W / r.width, (e.clientY - r.top) * H / r.height];
  }
  let dragging = null; // {mode, handle, ...} while the pointer is down
  canvas.addEventListener("contextmenu", (e) => e.preventDefault());
  canvas.addEventListener("pointermove", (e) => {
    if (!doc || dragging) return;
    const [x, y] = canvasPoint(e);
    const h = handleAt(x, y, gizmoGeometry(camera()));
    if (!sameHandle(h, hovered)) {
      hovered = h;
      canvas.style.cursor = h ? "grab" : "crosshair";
      render();
    }
  });
  canvas.addEventListener("pointerdown", (e) => {
    if (!doc) return;
    e.preventDefault();
    const [x, y] = canvasPoint(e);
    const cam = camera();
    // The camera: the right button looks around, with WASD to move while
    // it is held; Alt orbits what the camera looks at; the middle button
    // or Ctrl pans. The left button picks and drags.
    let mode = e.button === 2 ? "look" : e.altKey ? "orbit" : e.button === 1 || e.ctrlKey ? "pan" : "pick";
    if (mode === "look") flying = true;
    let handle = null;
    if (mode === "pick") {
      handle = handleAt(x, y, gizmoGeometry(cam));
      if (!handle) {
        const px = Math.max(0, Math.min(W - 1, Math.floor(x)));
        const py = Math.max(0, Math.min(H - 1, Math.floor(y)));
        const id = ids[py * W + px];
        if (id > 0) select({ kind: "object", index: id - 1 });
        else if (id < 0) select({ kind: "sprite", index: -id - 1 });
        else select(null);
      }
    }
    canvas.setPointerCapture(e.pointerId);
    canvas.focus({ preventScroll: true });
    beginEdit();
    const o = selected?.kind === "object" ? doc.objects[selected.index] : null;
    dragging = { mode, handle, last: [e.clientX, e.clientY], start: [x, y], startRotation: o ? o.rotation.slice() : null, startScale: o ? o.scale.slice() : null, startPosition: o ? o.position.slice() : null, startSprite: selected?.kind === "sprite" ? [doc.sprites[selected.index].x, doc.sprites[selected.index].y] : null, acc: [0, 0, 0] };
    const scale = canvas.getBoundingClientRect().width / W;
    const move = (ev) => {
      const dx = (ev.clientX - dragging.last[0]) / scale;
      const dy = (ev.clientY - dragging.last[1]) / scale;
      dragging.last = [ev.clientX, ev.clientY];
      const [cx, cy] = canvasPoint(ev);
      if (!dx && !dy) return;
      if (dragging.mode === "orbit") orbit(dx, dy);
      else if (dragging.mode === "look") look(dx, dy);
      else if (dragging.mode === "pan") pan(dx, dy);
      else if (selected?.kind === "object") dragObject(dx, dy, ev.shiftKey, cx, cy);
      else if (selected?.kind === "sprite") dragSprite(cx, cy);
    };
    const up = () => {
      canvas.removeEventListener("pointermove", move);
      canvas.removeEventListener("pointerup", up);
      dragging = null;
      flying = false;
      endEdit();
      render();
    };
    canvas.addEventListener("pointermove", move);
    canvas.addEventListener("pointerup", up);
  });

  // snapped rounds a position to the grid when snapping is on.
  const snapped = (v) => (snap ? Math.round(v / 16) * 16 : Math.round(v));
  const snappedTurn = (v) => (snap ? Math.round(v / 64) * 64 : Math.round(v));

  function dragObject(dx, dy, into, cx, cy) {
    const o = doc.objects[selected.index];
    const cam = camera();
    const g = gizmoGeometry(cam);
    const h = dragging.handle;
    // along is how far the pointer has travelled along an axis's direction
    // on the screen, in the axis's units.
    const along = (i) => {
      const end = g.axes[i];
      const sx = end[0] - g.origin[0], sy = end[1] - g.origin[1];
      const len2 = sx * sx + sy * sy || 1;
      return ((cx - dragging.start[0]) * sx + (cy - dragging.start[1]) * sy) / len2 * AXIS_LEN;
    };
    if (h?.kind === "ring" && g) {
      // The angle the pointer has swept about the object turns it about
      // the ring's axis, the way round depending on which side the camera
      // sees the ring from.
      const a0 = Math.atan2(dragging.start[1] - g.origin[1], dragging.start[0] - g.origin[0]);
      const a1 = Math.atan2(cy - g.origin[1], cx - g.origin[0]);
      let turn = (a1 - a0) / (2 * Math.PI) * 4096;
      const axis = [[1, 0, 0], [0, 1, 0], [0, 0, 1]][h.axis];
      const toCam = [0, 1, 2].map((i) => cam.from[i] - o.position[i]);
      if (dot(axis, toCam) > 0) turn = -turn;
      o.rotation[h.axis] = ((snappedTurn(dragging.startRotation[h.axis] + turn) % 4096) + 4096) % 4096;
      changed();
      renderInspector();
      return;
    }
    if (h?.kind === "axis" && g && g.axes[h.axis]) {
      const pos = dragging.startPosition.slice();
      pos[h.axis] = snapped(pos[h.axis] + along(h.axis));
      o.position = pos;
      changed();
      renderInspector();
      return;
    }
    if (h?.kind === "plane" && g) {
      // In a plane: the pointer's travel split between the two axes'
      // directions on the screen.
      const [a, b] = h.axes;
      const A = [g.axes[a][0] - g.origin[0], g.axes[a][1] - g.origin[1]];
      const B = [g.axes[b][0] - g.origin[0], g.axes[b][1] - g.origin[1]];
      const dx = cx - dragging.start[0], dy = cy - dragging.start[1];
      const det = A[0] * B[1] - A[1] * B[0];
      if (Math.abs(det) > 1e-6) {
        const ta = (dx * B[1] - dy * B[0]) / det * AXIS_LEN;
        const tb = (A[0] * dy - A[1] * dx) / det * AXIS_LEN;
        const pos = dragging.startPosition.slice();
        pos[a] = snapped(pos[a] + ta);
        pos[b] = snapped(pos[b] + tb);
        o.position = pos;
        changed();
        renderInspector();
      }
      return;
    }
    if ((h?.kind === "scale" || h?.kind === "uniform") && g) {
      const base = dragging.startScale.map((c) => c || 4096);
      if (h.kind === "uniform") {
        const k = Math.max(0.05, 1 + (cx - dragging.start[0]) / 100);
        o.scale = base.map((c) => Math.max(1, Math.round(c * k)));
      } else {
        const k = Math.max(0.05, 1 + along(h.axis) / AXIS_LEN);
        o.scale = base.slice();
        o.scale[h.axis] = Math.max(1, Math.round(base[h.axis] * k));
      }
      changed();
      renderInspector();
      return;
    }
    const c = toCamera(cam, o.position);
    const k = Math.max(c[2], 16) / cam.h;
    let d;
    if (into) d = cam.f.map((v) => v * -dy * k);
    else d = [0, 1, 2].map((i) => (cam.r[i] * dx + cam.u[i] * dy) * k);
    const acc = dragging.acc;
    acc[0] += d[0]; acc[1] += d[1]; acc[2] += d[2];
    o.position = [0, 1, 2].map((i) => snapped(dragging.startPosition[i] + acc[i]));
    changed();
    renderInspector();
  }

  function dragSprite(cx, cy) {
    const sp = doc.sprites[selected.index];
    sp.x = Math.round(dragging.startSprite[0] + cx - dragging.start[0]);
    sp.y = Math.round(dragging.startSprite[1] + cy - dragging.start[1]);
    changed();
    renderInspector();
  }

  function orbit(dx, dy) {
    const { from, at } = doc.camera;
    const d = [from[0] - at[0], from[1] - at[1], from[2] - at[2]];
    const dist = Math.hypot(...d) || 1;
    let yaw = Math.atan2(d[0], d[2]);
    let pitch = Math.asin(Math.max(-1, Math.min(1, d[1] / dist)));
    yaw -= dx * 0.01;
    pitch = Math.max(-1.5, Math.min(1.5, pitch + dy * 0.01));
    doc.camera.from = [
      Math.round(at[0] + dist * Math.cos(pitch) * Math.sin(yaw)),
      Math.round(at[1] + dist * Math.sin(pitch)),
      Math.round(at[2] + dist * Math.cos(pitch) * Math.cos(yaw)),
    ];
    changed();
    renderCamera();
  }
  // pan slides the camera and what it looks at together across the view.
  function pan(dx, dy) {
    const cam = camera();
    const dist = Math.hypot(...[0, 1, 2].map((i) => doc.camera.from[i] - doc.camera.at[i])) || 1;
    const k = dist / cam.h;
    const d = [0, 1, 2].map((i) => -(cam.r[i] * dx + cam.u[i] * dy) * k);
    doc.camera.from = doc.camera.from.map((v, i) => Math.round(v + d[i]));
    doc.camera.at = doc.camera.at.map((v, i) => Math.round(v + d[i]));
    changed();
    renderCamera();
  }
  // lookAt turns the camera on what is picked, or the origin.
  // look turns the camera where it stands, as a head turns.
  function look(dx, dy) {
    const { from, at } = doc.camera;
    const d = [at[0] - from[0], at[1] - from[1], at[2] - from[2]];
    const dist = Math.hypot(...d) || 1;
    let yaw = Math.atan2(d[0], d[2]);
    let pitch = Math.asin(Math.max(-1, Math.min(1, d[1] / dist)));
    yaw += dx * 0.006;
    pitch = Math.max(-1.5, Math.min(1.5, pitch + dy * 0.006));
    doc.camera.at = [
      Math.round(from[0] + dist * Math.cos(pitch) * Math.sin(yaw)),
      Math.round(from[1] + dist * Math.sin(pitch)),
      Math.round(from[2] + dist * Math.cos(pitch) * Math.cos(yaw)),
    ];
    changed();
    renderCamera();
  }
  // fly moves the camera and its target together along its own axes.
  function fly(forward, right, up) {
    const cam = camera();
    const step = 24;
    const d = [0, 1, 2].map((i) => (cam.f[i] * forward + cam.r[i] * right + cam.u[i] * up) * step);
    doc.camera.from = doc.camera.from.map((v, i) => Math.round(v + d[i]));
    doc.camera.at = doc.camera.at.map((v, i) => Math.round(v + d[i]));
    changed();
    renderCamera();
  }
  // frame turns the camera on what is picked, or the origin, and stands
  // back far enough to see a model whole.
  function frame() {
    let target = [0, 0, 0];
    let radius = 200;
    if (selected?.kind === "object") {
      const o = doc.objects[selected.index];
      target = o.position;
      const obj = tmdFor(o.model)?.objects[o.index];
      if (obj) {
        const { s } = rotation(o.rotation, o.scale);
        radius = 1;
        for (let k = 0; k < obj.vertices.length / 3; k++) radius = Math.max(radius, Math.hypot(obj.vertices[3 * k] * s[0], obj.vertices[3 * k + 1] * s[1], obj.vertices[3 * k + 2] * s[2]));
      }
    }
    const d = [0, 1, 2].map((i) => doc.camera.from[i] - doc.camera.at[i]);
    const len = Math.hypot(...d) || 1;
    const dist = Math.max(radius * 3.5, 64);
    doc.camera.at = target.slice();
    doc.camera.from = target.map((v, i) => Math.round(v + d[i] / len * dist));
    changed(true);
    renderCamera();
  }
  // preset puts the camera in front of, above or beside what it looks at,
  // as far away as it is now.
  function preset(which) {
    const { from, at } = doc.camera;
    const dist = Math.round(Math.hypot(...[0, 1, 2].map((i) => from[i] - at[i]))) || 900;
    const offset = which === "top" ? [0, -dist, 1] : which === "side" ? [dist, 0, 0] : [0, 0, -dist];
    doc.camera.from = at.map((v, i) => v + offset[i]);
    changed(true);
    renderCamera();
  }
  canvas.addEventListener("wheel", (e) => {
    if (!doc) return;
    e.preventDefault();
    const { from, at } = doc.camera;
    const k = Math.pow(1.1, e.deltaY / 100);
    doc.camera.from = [0, 1, 2].map((i) => Math.round(at[i] + (from[i] - at[i]) * k));
    changed();
    renderCamera();
  }, { passive: false });

  // Keys, on the picture and the outline: arrows nudge what is picked (8
  // units, 64 with Shift; a sprite a pixel, 8 with Shift), Page Up and
  // Down raise and lower it, Q and E turn it, G snaps, Ctrl+D duplicates,
  // Delete removes, Ctrl+Z and Ctrl+Y undo and redo, Escape unpicks.
  function onKeys(e) {
    if (!doc) return;
    const mod = e.ctrlKey || e.metaKey;
    if (typing) {
      typeKey(e);
      return;
    }
    if (flying) {
      // WASD walk, Q and E sink and rise, while the right button is held.
      const k = e.key.toLowerCase();
      const moves = { w: [1, 0, 0], s: [-1, 0, 0], d: [0, 1, 0], a: [0, -1, 0], e: [0, 0, -1], q: [0, 0, 1] };
      if (moves[k]) {
        e.preventDefault();
        fly(...moves[k]);
        return;
      }
    }
    if (!mod && !e.altKey) {
      const k = e.key.toLowerCase();
      if (k === "w" || k === "e" || k === "r") {
        setGizmoMode({ w: "move", e: "rotate", r: "scale" }[k]);
        return;
      }
      if (k === "g" && selected?.kind === "object") {
        e.preventDefault();
        typing = { op: "move", axis: null, text: "" };
        render();
        return;
      }
      if (k === "f") {
        frame();
        return;
      }
    }
    if (mod && (e.key === "z" || e.key === "Z")) {
      e.preventDefault();
      if (e.shiftKey) redo(); else undo();
      return;
    }
    if (mod && (e.key === "y" || e.key === "Y")) {
      e.preventDefault();
      redo();
      return;
    }
    if (mod && (e.key === "d" || e.key === "D")) {
      e.preventDefault();
      duplicate();
      return;
    }
    if (e.key === "n" || e.key === "N") {
      setSnap(!snap);
      return;
    }
    if (e.key === "Escape") {
      select(null);
      return;
    }
    if (!selected) return;
    if (e.key === "Delete" || e.key === "Backspace") {
      e.preventDefault();
      remove();
      return;
    }
    const big = e.shiftKey;
    if (selected.kind === "object") {
      const o = doc.objects[selected.index];
      const n = big ? 64 : 8;
      const moves = { ArrowLeft: [-n, 0, 0], ArrowRight: [n, 0, 0], ArrowUp: [0, 0, n], ArrowDown: [0, 0, -n], PageUp: [0, -n, 0], PageDown: [0, n, 0] };
      if (moves[e.key]) {
        e.preventDefault();
        o.position = o.position.map((v, i) => v + moves[e.key][i]);
      } else if (e.key === "[") o.rotation[1] = (o.rotation[1] - 64 + 4096) % 4096;
      else if (e.key === "]") o.rotation[1] = (o.rotation[1] + 64) % 4096;
      else return;
    } else {
      const sp = doc.sprites[selected.index];
      const n = big ? 8 : 1;
      const moves = { ArrowLeft: [-n, 0], ArrowRight: [n, 0], ArrowUp: [0, -n], ArrowDown: [0, n] };
      if (!moves[e.key]) return;
      e.preventDefault();
      sp.x += moves[e.key][0];
      sp.y += moves[e.key][1];
    }
    changed();
    renderInspector();
  }
  canvas.addEventListener("keydown", onKeys);
  outline.addEventListener("keydown", (e) => { if (!e.target.closest("input")) onKeys(e); });

  // A typed transform: G starts a move; R or S then make it a turn or a
  // scale; X, Y or Z picks the axis; digits, a minus and a point give the
  // amount; Enter applies it and Escape lets it go.
  function typeKey(e) {
    e.preventDefault();
    const k = e.key;
    if (k === "Escape") {
      typing = null;
      render();
      return;
    }
    if (k === "Enter") {
      applyTyped();
      typing = null;
      render();
      return;
    }
    if (k === "Backspace") typing.text = typing.text.slice(0, -1);
    else if (/^[0-9.]$/.test(k)) typing.text += k;
    else if (k === "-") typing.text = typing.text.startsWith("-") ? typing.text.slice(1) : "-" + typing.text;
    else if (/^[xyz]$/i.test(k)) typing.axis = "xyz".indexOf(k.toLowerCase());
    else if (/^[grs]$/i.test(k)) typing.op = { g: "move", r: "rotate", s: "scale" }[k.toLowerCase()];
    render();
  }
  function typingStatus() {
    const axis = typing.axis === null ? (typing.op === "rotate" ? "Y" : typing.op === "scale" ? "every axis" : "X") : "XYZ"[typing.axis];
    const unit = typing.op === "move" ? "units" : typing.op === "rotate" ? "degrees" : "times";
    return `${typing.op === "move" ? "Move" : typing.op === "rotate" ? "Turn" : "Scale"} ${typing.text || "…"} ${unit} along ${axis}. X, Y, Z choose the axis, R or S a turn or a scale, Enter applies, Escape cancels.`;
  }
  function applyTyped() {
    const n = Number(typing.text);
    if (!Number.isFinite(n) || selected?.kind !== "object") return;
    const o = doc.objects[selected.index];
    if (typing.op === "move") {
      o.position[typing.axis ?? 0] = Math.round(o.position[typing.axis ?? 0] + n);
    } else if (typing.op === "rotate") {
      const i = typing.axis ?? 1;
      o.rotation[i] = ((Math.round(o.rotation[i] + n * 4096 / 360) % 4096) + 4096) % 4096;
    } else {
      const base = o.scale.map((c) => c || 4096);
      if (typing.axis === null) o.scale = base.map((c) => Math.max(1, Math.round(c * n)));
      else {
        o.scale = base.slice();
        o.scale[typing.axis] = Math.max(1, Math.round(base[typing.axis] * n));
      }
    }
    changed(true);
    renderInspector();
  }

  function setGizmoMode(m) {
    gizmoMode = m;
    hovered = null;
    for (const b of document.querySelectorAll("#scene-modes button")) b.setAttribute("aria-pressed", String(b.dataset.mode === m));
    render();
  }
  for (const b of document.querySelectorAll("#scene-modes button")) b.addEventListener("click", () => setGizmoMode(b.dataset.mode));
  function setSnap(on) {
    snap = on;
    snapButton.setAttribute("aria-pressed", String(on));
  }
  snapButton.addEventListener("click", () => setSnap(!snap));
  $("scene-undo").addEventListener("click", undo);
  $("scene-redo").addEventListener("click", redo);
  $("scene-look").addEventListener("click", frame);
  $("scene-console").addEventListener("click", () => setConsole(!showConsole));

  // The console's own picture in the viewport, while this project runs,
  // fetched from the emulator's worker a few times a second.
  function setConsole(on) {
    showConsole = on;
    $("scene-console").setAttribute("aria-pressed", String(on));
    clearInterval(consoleTimer);
    consoleTimer = 0;
    if (on) consoleTimer = setInterval(fetchFrame, 120);
    else consoleFrame = null;
    render();
  }
  function liveNow() {
    return !!(state.running && state.current?.id && state.current.id === project()?.id);
  }
  async function fetchFrame() {
    if (!shown || !showConsole) return;
    if (!liveNow()) {
      if (consoleFrame) {
        consoleFrame = null;
        render();
      }
      return;
    }
    const reply = await call("frame");
    if (reply.bitmap) {
      consoleFrame?.close?.();
      consoleFrame = reply.bitmap;
      render();
    }
  }
  function renderLive() {
    const live = liveNow();
    $("scene-view").classList.toggle("live", live);
    $("scene-console").disabled = !state.current?.id || state.current.id !== project()?.id;
    if (!live && consoleFrame) {
      consoleFrame = null;
      render();
    }
  }
  projectEvents.addEventListener("live", renderLive);
  for (const which of ["front", "top", "side"]) $(`scene-view-${which}`).addEventListener("click", () => preset(which));

  function select(sel) {
    selected = sel;
    hovered = null;
    render();
    renderInspector();
    renderOutline();
    if (sel) showPanel("object");
  }

  // The outline: everything in the scene, to pick from.
  function renderOutline() {
    if (!doc) {
      outline.replaceChildren();
      return;
    }
    const items = [];
    const row = (label, meta, { kind, index, panel, cls = "" }) => {
      const li = document.createElement("li");
      li.setAttribute("role", "option");
      const on = kind && selected?.kind === kind && selected.index === index;
      li.setAttribute("aria-selected", String(!!on));
      li.className = `outline-row ${cls}`;
      li.tabIndex = on ? 0 : -1;
      const name = document.createElement("span");
      name.className = "outline-name";
      name.textContent = label;
      const m = document.createElement("span");
      m.className = "muted outline-meta";
      m.textContent = meta;
      li.append(name, m);
      li.addEventListener("click", () => {
        if (kind) select({ kind, index });
        else select(null);
        if (panel) showPanel(panel);
      });
      li.addEventListener("dblclick", () => {
        if (kind) inspector.querySelector('input[type="text"]')?.focus();
      });
      li.addEventListener("contextmenu", (e) => {
        e.preventDefault();
        if (!kind) return;
        select({ kind, index });
        files.popupMenu([
          ["Rename", "Give it another name", () => inspector.querySelector('input[type="text"]')?.focus()],
          ["Duplicate", "A copy beside it (Ctrl+D)", duplicate],
          ["Frame", "Turn the camera on it (F)", frame],
          [kind === "object" && doc.objects[index].hidden ? "Show" : "Hide in the editor", "The console still draws it", () => { if (kind === "object") { doc.objects[index].hidden = !doc.objects[index].hidden; changed(true); } }, kind !== "object"],
          ["Remove", "Take it out of the scene (Delete)", remove],
        ], e);
      });
      items.push(li);
    };
    const head = (label) => {
      const li = document.createElement("li");
      li.className = "outline-group";
      li.setAttribute("role", "presentation");
      li.textContent = label;
      items.push(li);
    };
    const matches = (name) => !search || name.toLowerCase().includes(search);
    if (matches("camera")) row("Camera", `${doc.camera.from.join(", ")}`, { panel: "camera" });
    doc.lights.forEach((l, i) => { if (matches(`light ${i + 1}`)) row(`Light ${i + 1}`, `${l.direction.join(", ")}`, { panel: "lights" }); });
    if (matches("tile map")) row("Tile map", doc.background ? `${doc.background.width} by ${doc.background.height}` : "none", { panel: "tiles" });
    head(`Objects (${doc.objects.length})`);
    doc.objects.forEach((o, i) => {
      if (!matches(o.name || `object ${i}`)) return;
      row(o.name || `object ${i}`, o.model.replace(/^ASSETS\//, ""), { kind: "object", index: i, cls: o.hidden ? "hidden-row" : "" });
      const li = items.at(-1);
      if (problems.has(i)) {
        const warn = document.createElement("span");
        warn.className = "outline-warn";
        warn.textContent = "!";
        warn.title = problems.get(i);
        li.insertBefore(warn, li.firstChild);
      }
      const eye = document.createElement("button");
      eye.type = "button";
      eye.className = "outline-eye";
      eye.setAttribute("aria-pressed", String(!o.hidden));
      eye.setAttribute("aria-label", o.hidden ? `Show ${o.name}` : `Hide ${o.name} in the editor`);
      eye.title = o.hidden ? "Hidden in the editor; the console still draws it" : "Hide in the editor; the console still draws it";
      eye.textContent = o.hidden ? "○" : "●";
      eye.addEventListener("click", (e) => {
        e.stopPropagation();
        o.hidden = !o.hidden;
        changed(true);
      });
      li.append(eye);
    });
    head(`Sprites (${doc.sprites.length})`);
    doc.sprites.forEach((sp, i) => { if (matches(sp.name || `sprite ${i}`)) row(sp.name || `sprite ${i}`, sp.texture.replace(/^ASSETS\//, ""), { kind: "sprite", index: i }); });
    outline.replaceChildren(...items);
  }
  $("scene-search").addEventListener("input", (e) => {
    search = e.target.value.trim().toLowerCase();
    renderOutline();
  });

  // The panels under the picture, one at a time.
  const PANELS = ["object", "camera", "lights", "tiles"];
  function showPanel(name) {
    for (const p of PANELS) {
      $(`spanel-${p}`).hidden = p !== name;
      const tab = $(`stab-${p}`);
      tab.setAttribute("aria-selected", String(p === name));
      tab.tabIndex = p === name ? 0 : -1;
    }
  }
  for (const p of PANELS) $(`stab-${p}`).addEventListener("click", () => showPanel(p));

  // The toolbar.
  function fillLists() {
    const tmds = dataFiles(/\.tmd$/i).map(discPath);
    addModel.replaceChildren(new Option("Add model…", ""), ...tmds.map((p) => new Option(p.replace(/^ASSETS\//, ""), p)));
    addModel.disabled = !tmds.length;
    const tims = dataFiles(/\.tim$/i).map(discPath);
    addSprite.replaceChildren(new Option("Add sprite…", ""), ...tims.map((p) => new Option(p.replace(/^ASSETS\//, ""), p)));
    addSprite.disabled = !tims.length;
  }
  function uniqueName(base, list) {
    const taken = new Set(list.map((o) => o.name));
    if (!taken.has(base)) return base;
    for (let n = 2; ; n++) if (!taken.has(`${base} ${n}`)) return `${base} ${n}`;
  }
  addModel.addEventListener("change", () => {
    const model = addModel.value;
    addModel.value = "";
    if (!model || !doc) return;
    const base = model.replace(/^ASSETS\//, "").replace(/\.TMD$/, "").toLowerCase().slice(0, 12);
    doc.objects.push({ name: uniqueName(base, doc.objects), model, index: 0, position: [0, 0, 0], rotation: [0, 0, 0], scale: [0, 0, 0] });
    select({ kind: "object", index: doc.objects.length - 1 });
    changed(true);
  });
  addSprite.addEventListener("change", () => {
    const texture = addSprite.value;
    addSprite.value = "";
    if (!texture || !doc) return;
    const tim = timFor(texture);
    if (!tim) return;
    const [u, v] = preview.uv(tim);
    const base = texture.replace(/^ASSETS\//, "").replace(/\.TIM$/, "").toLowerCase().slice(0, 12);
    doc.sprites.push({
      name: uniqueName(base, doc.sprites), texture, attribute: 0,
      x: -Math.min(tim.width, 256) / 2, y: -Math.min(tim.height, 256) / 2, w: Math.min(tim.width, 256), h: Math.min(tim.height, 256),
      tpage: preview.tpage(tim), u, v, cx: tim.clut?.x ?? 0, cy: tim.clut?.y ?? 0, r: 128, g: 128, b: 128, mx: 0, my: 0, scalex: 0, scaley: 0, rotate: 0,
    });
    select({ kind: "sprite", index: doc.sprites.length - 1 });
    changed(true);
  });
  function remove() {
    if (!selected || !doc) return;
    const list = selected.kind === "object" ? doc.objects : doc.sprites;
    list.splice(selected.index, 1);
    select(null);
    changed(true);
  }
  function duplicate() {
    if (!selected || !doc) return;
    const list = selected.kind === "object" ? doc.objects : doc.sprites;
    const copy = JSON.parse(JSON.stringify(list[selected.index]));
    copy.name = uniqueName(copy.name.replace(/ \d+$/, ""), list);
    if (selected.kind === "object") copy.position = [copy.position[0] + 64, copy.position[1], copy.position[2]];
    else copy.x += 16;
    list.push(copy);
    select({ kind: selected.kind, index: list.length - 1 });
    changed(true);
  }
  $("scene-remove").addEventListener("click", remove);
  $("scene-duplicate").addEventListener("click", duplicate);

  // The inspector: the selection's fields, in the console's units.
  // field makes a labelled input. A number's label scrubs: dragging on it
  // changes the value by step for every few pixels. With unit "deg" the
  // field shows degrees for a value kept in 4096ths of a turn.
  function field(label, value, onChange, { type = "number", title = "", options = null, step = 1, unit = "" } = {}) {
    const wrap = document.createElement("label");
    wrap.title = title;
    const span = document.createElement("span");
    span.textContent = unit === "deg" ? `${label}°` : label;
    let input;
    const toShow = (v) => (unit === "deg" ? Math.round(v * 360 / 4096 * 10) / 10 : v);
    const fromShow = (v) => (unit === "deg" ? Math.round(v * 4096 / 360) : v);
    if (options) {
      input = document.createElement("select");
      for (const [v, name] of options) input.append(new Option(name, v));
      input.value = value;
      input.addEventListener("change", () => onChange(input.value));
    } else {
      input = document.createElement("input");
      input.type = type;
      input.value = toShow(value);
      if (type === "number") {
        input.step = unit === "deg" ? "any" : String(step);
        input.addEventListener("input", () => { if (input.value !== "") onChange(fromShow(Number(input.value))); });
        span.className = "scrub";
        span.title = (title ? title + ". " : "") + "Drag here to change it";
        span.addEventListener("pointerdown", (e) => {
          e.preventDefault();
          span.setPointerCapture(e.pointerId);
          const startX = e.clientX;
          const start = Number(input.value) || 0;
          const per = unit === "deg" ? 1 : step; // per 4 pixels
          beginEdit();
          const move = (ev) => {
            const v = start + Math.round((ev.clientX - startX) / 4) * per;
            input.value = v;
            onChange(fromShow(v));
          };
          const up = () => {
            span.removeEventListener("pointermove", move);
            span.removeEventListener("pointerup", up);
            endEdit();
          };
          span.addEventListener("pointermove", move);
          span.addEventListener("pointerup", up);
        });
      } else {
        input.addEventListener("change", () => onChange(input.value));
      }
    }
    input.dataset.unit = unit;
    wrap.append(span, input);
    return wrap;
  }
  function vectorFields(form, labels, get, set, title, step = 1) {
    labels.forEach((l, i) => form.append(field(l, get()[i], (v) => {
      const arr = get().slice();
      arr[i] = v;
      set(arr);
      changed();
    }, { title, step })));
  }
  let inspectorFor = null;
  function renderInspector() {
    const key = selected ? `${selected.kind}:${selected.index}` : "";
    if (key === inspectorFor && inspector.childElementCount) {
      // Only the numbers change while dragging.
      for (const input of inspector.querySelectorAll("input[data-live]")) {
        const [kind, i] = input.dataset.live.split(":");
        const item = selected.kind === "object" ? doc.objects[selected.index] : doc.sprites[selected.index];
        let value = kind === "position" ? item.position[Number(i)] : kind === "rotation" ? item.rotation[Number(i)] : item[kind];
        if (input.dataset.unit === "deg") value = Math.round(value * 360 / 4096 * 10) / 10;
        if (document.activeElement !== input) input.value = value;
      }
      return;
    }
    inspectorFor = key;
    inspector.replaceChildren();
    if (!selected || !doc) {
      const p = document.createElement("span");
      p.className = "muted";
      p.textContent = "Nothing picked. Click an object or a sprite in the picture, or add one.";
      inspector.append(p);
      return;
    }
    if (selected.kind === "object") {
      const o = doc.objects[selected.index];
      inspector.append(field("Name", o.name, (v) => { o.name = v.slice(0, 15); changed(); }, { type: "text", title: "Up to 15 characters; the program finds the object by it with world.Object(name)" }));
      const tmds = dataFiles(/\.tmd$/i).map(discPath);
      if (!tmds.includes(o.model)) tmds.push(o.model);
      inspector.append(field("Model", o.model, (v) => { o.model = v; changed(); }, { options: tmds.map((p) => [p, p.replace(/^ASSETS\//, "")]) }));
      const tmd = tmdFor(o.model);
      if (tmd && tmd.objects.length > 1) inspector.append(field("Object", o.index, (v) => { o.index = Math.max(0, Math.min(tmd.objects.length - 1, Math.round(v))); changed(); }, { title: `Which of the TMD's ${tmd.objects.length} objects` }));
      const pos = [];
      ["X", "Y", "Z"].forEach((l, i) => {
        const f = field(`${l}`, o.position[i], (v) => { o.position[i] = Math.round(v); changed(); }, { title: "Position in world units; Y is down" });
        f.querySelector("input").dataset.live = `position:${i}`;
        pos.push(f);
      });
      inspector.append(group("Position", pos));
      inspector.append(group("Rotation", ["X", "Y", "Z"].map((l, i) => {
        const f = field(l, o.rotation[i], (v) => { o.rotation[i] = ((Math.round(v) % 4096) + 4096) % 4096; changed(); }, { title: "In degrees here; the file keeps 4096ths of a turn", unit: "deg" });
        f.querySelector("input").dataset.live = `rotation:${i}`;
        return f;
      })));
      inspector.append(group("Scale", ["X", "Y", "Z"].map((l, i) => field(l, o.scale[i], (v) => { o.scale[i] = Math.round(v); changed(); }, { title: "4096 is the model's own size; all zero leaves it unscaled", step: 256 }))));
      inspector.append(group("Fields", ["A", "B"].map((l, i) => field(l, o.fields[i], (v) => { o.fields[i] = Math.round(v); changed(); }, { title: "Two numbers of your own, which the program reads with world.ObjectFields(i): a kind, a score, a door's key" }))));
    } else {
      const sp = doc.sprites[selected.index];
      inspector.append(field("Name", sp.name, (v) => { sp.name = v.slice(0, 15); changed(); }, { type: "text", title: "Up to 15 characters; the program finds the sprite by it with world.Sprite(name)" }));
      const tims = dataFiles(/\.tim$/i).map(discPath);
      if (!tims.includes(sp.texture)) tims.push(sp.texture);
      inspector.append(field("Texture", sp.texture, (v) => {
        sp.texture = v;
        const tim = timFor(v);
        if (tim) {
          const [u, vv] = preview.uv(tim);
          Object.assign(sp, { tpage: preview.tpage(tim), u, v: vv, cx: tim.clut?.x ?? 0, cy: tim.clut?.y ?? 0, w: Math.min(tim.width, 256), h: Math.min(tim.height, 256) });
        }
        changed();
        inspectorFor = null;
        renderInspector();
      }, { options: tims.map((p) => [p, p.replace(/^ASSETS\//, "")]) }));
      const xy = ["x", "y"].map((k) => {
        const f = field(k.toUpperCase(), sp[k], (v) => { sp[k] = Math.round(v); changed(); }, { title: "Pixels from the middle of the screen" });
        f.querySelector("input").dataset.live = k;
        return f;
      });
      inspector.append(group("At", xy));
      inspector.append(group("Size", ["w", "h"].map((k) => field(k.toUpperCase(), sp[k], (v) => { sp[k] = Math.max(0, Math.round(v)); changed(); }, { title: "Pixels" }))));
      inspector.append(group("Cut from", ["u", "v"].map((k) => field(k.toUpperCase(), sp[k], (v) => { sp[k] = Math.max(0, Math.min(255, Math.round(v))); changed(); }, { title: "The corner within the texture page, in texels" }))));
      inspector.append(group("Brightness", ["r", "g", "b"].map((k) => field(k.toUpperCase(), sp[k], (v) => { sp[k] = Math.max(0, Math.min(255, Math.round(v))); changed(); }, { title: "128 shows the texture as it is" }))));
    }
  }
  function group(name, fields) {
    const g = document.createElement("fieldset");
    const legend = document.createElement("legend");
    legend.textContent = name;
    g.append(legend, ...fields);
    return g;
  }

  // The camera and the lights.
  function renderCamera() {
    if (!doc) {
      cameraForm.replaceChildren();
      return;
    }
    if (cameraForm.childElementCount) {
      for (const input of cameraForm.querySelectorAll("input[data-cam]")) {
        const [k, i] = input.dataset.cam.split(":");
        if (document.activeElement !== input) input.value = doc.camera[k][Number(i)];
      }
      return;
    }
    const fromFields = ["X", "Y", "Z"].map((l, i) => {
      const f = field(l, doc.camera.from[i], (v) => { doc.camera.from[i] = Math.round(v); changed(); }, { title: "Where the camera is", step: 16 });
      f.querySelector("input").dataset.cam = `from:${i}`;
      return f;
    });
    const atFields = ["X", "Y", "Z"].map((l, i) => {
      const f = field(l, doc.camera.at[i], (v) => { doc.camera.at[i] = Math.round(v); changed(); }, { title: "What it looks at", step: 16 });
      f.querySelector("input").dataset.cam = `at:${i}`;
      return f;
    });
    cameraForm.append(
      group("Camera at", fromFields),
      group("Looking at", atFields),
      field("Twist", doc.camera.twist || 0, (v) => { doc.camera.twist = Math.round(v); changed(); }, { title: "About the line of sight, 4096 a degree", step: 4096 }),
      field("Projection", doc.projection, (v) => { doc.projection = Math.max(1, Math.round(v)); changed(); renderFOV(); }, { title: "The distance from the viewpoint to the screen: smaller is a wider view", step: 16 }),
    );
    const fov = document.createElement("span");
    fov.className = "muted scene-fov";
    cameraForm.append(fov);
    renderFOV();
  }
  // renderFOV says what the projection distance means on a 320-wide screen.
  function renderFOV() {
    const el = cameraForm.querySelector(".scene-fov");
    if (!el || !doc) return;
    const deg = Math.round(2 * Math.atan(160 / Math.max(1, doc.projection)) * 180 / Math.PI);
    el.textContent = `${deg}° across the screen`;
  }
  function renderLights() {
    lightsForm.replaceChildren();
    if (!doc) return;
    lightsForm.append(group("Ambient", ["R", "G", "B"].map((l, i) => field(l, doc.ambient[i], (v) => { doc.ambient[i] = Math.max(0, Math.round(v)); changed(); }, { title: "4096 is full", step: 256 }))));
    doc.lights.forEach((light, i) => {
      const dir = ["X", "Y", "Z"].map((l, k) => field(l, light.direction[k], (v) => { light.direction[k] = Math.round(v); changed(); }, { title: "The direction the light shines in" }));
      const color = document.createElement("label");
      const span = document.createElement("span");
      span.textContent = "Color";
      const input = document.createElement("input");
      input.type = "color";
      input.value = "#" + light.color.map((c) => c.toString(16).padStart(2, "0")).join("");
      input.addEventListener("input", () => {
        light.color = [1, 3, 5].map((k) => parseInt(input.value.slice(k, k + 2), 16));
        changed();
      });
      color.append(span, input);
      const off = document.createElement("button");
      off.type = "button";
      off.className = "small-button";
      off.textContent = "Remove";
      off.addEventListener("click", () => {
        doc.lights.splice(i, 1);
        changed(true);
        renderLights();
      });
      lightsForm.append(group(`Light ${i + 1}`, [...dir, color, off]));
    });
    if (doc.lights.length < 3) {
      const add = document.createElement("button");
      add.type = "button";
      add.className = "small-button";
      add.textContent = "Add light";
      add.addEventListener("click", () => {
        doc.lights.push({ direction: [doc.lights.length ? -1 : 1, 1, 1], color: [128, 128, 128] });
        changed(true);
        renderLights();
      });
      lightsForm.append(add);
    }
  }

  // Showing and hiding.
  function renderAll() {
    inspectorFor = null;
    cameraForm.replaceChildren();
    fillLists();
    render();
    renderInspector();
    renderOutline();
    renderCamera();
    renderLights();
    renderTiles();
    renderHistory();
  }
  function show() {
    shown = true;
    pane.hidden = false;
    read();
    renderAll();
    renderLive();
    if (showConsole && !consoleTimer) consoleTimer = setInterval(fetchFrame, 120);
    // The build cooks the scene when main.go has mkscene's line.
    const path = scenePath();
    if (path) convert.ensureLine("mkscene", ["-o", datPath(path), path], path);
  }
  function hide() {
    shown = false;
    pane.hidden = true;
    clearInterval(consoleTimer);
    consoleTimer = 0;
  }

  // create asks for a name and gives the project a scene so called, with
  // its line, picking it; it returns the scene's path, or null.
  function create() {
    const p = project();
    if (!p) return null;
    const taken = new Set(p.sceneFiles().map(stemOf));
    let suggested = "scene";
    for (let n = 2; taken.has(suggested); n++) suggested = `scene${n}`;
    const name = prompt("Name of the new scene", suggested);
    if (name === null) return null;
    const stem = sceneStem(name) || suggested;
    const path = `scenes/${stem}.json`;
    if (p.has(path)) {
      alert(`There is already a scene ${stem}.`);
      return null;
    }
    const dat = convert.outputFor(path, "mkscene");
    if (convert.lineForOutput(dat)) {
      alert(`The scene would be cooked into ${dat}, which another line already makes; a name different in its first eight letters is needed.`);
      return null;
    }
    p.set(path, JSON.stringify(blank(), null, 2) + "\n", "scene");
    convert.ensureLine("mkscene", ["-o", dat, path], path);
    convert.run();
    pick(path);
    return path;
  }

  // pick makes path the scene being edited.
  function pick(path) {
    if (path === current) return;
    current = path;
    selected = null;
    history.undo.length = history.redo.length = 0;
    if (shown) {
      read();
      renderAll();
    }
  }
  projectEvents.addEventListener("open", () => {
    selected = null;
    current = null;
    history.undo.length = history.redo.length = 0;
    cache.clear();
    vramCache = { keys: "", vram: null };
    hide();
  });
  projectEvents.addEventListener("change", ({ detail: { path, source } }) => {
    if (!shown) return;
    if (path === current && source !== "scene" && source !== "scene-history") {
      read();
      renderAll();
    } else if (path.startsWith("assets/") || path.startsWith("pack/")) {
      fillLists();
      render();
      renderOutline(); // the badges follow what the picture found
      if (/\.tim$/i.test(path)) renderTiles();
    }
  });
  projectEvents.addEventListener("files", () => {
    if (!shown) return;
    const p = project();
    if (!p?.sceneFiles().length) {
      hide();
      return;
    }
    if (!p.has(current)) {
      // The scene being edited was renamed or removed: the first one then.
      current = null;
      selected = null;
      history.undo.length = history.redo.length = 0;
      read();
      renderAll();
      return;
    }
    fillLists();
    render();
  });

  return { show, hide, create, pick, current: () => scenePath(), shown: () => shown, undo, redo, remove };
})();

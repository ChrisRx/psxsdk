// The room editor: a scene that is a room of cells, as package stage draws
// one, edited in the scene editor. The viewport draws the room the way the
// console does, through a camera like a game's, behind and above a point
// on the floor, turned about it an eighth of a turn at a time, and cells
// are painted by clicking them there or on the map from above in the
// viewport's corner, with the tools in the toolbar over it. The brushes,
// the room's look, its links to other scenes and its markers are in the
// inspector beside it. The scene editor hands this module its document,
// its drawing buffers and its helpers (attach), and asks it to draw, to
// handle the pointer and the keys, and to fill its panel and its outline.
//
// The runes are stage's legend: '.' floor, '#' wall, 'r' rock, 'c' a
// cabin's wall, 'b' and 'p' a bed, 't' a chest, '~' water, 'o' a pot, 'x'
// blocked ground, '0' to '9' a doorway. The geometry is stage's too:
// corner heights averaged across joined cells, cliffs between cells two
// levels apart, rock leaned and smoothed, cabins roofed, so that what is
// painted here is what the console shows.
"use strict";

const roomEdit = (() => {
  const TILE = 64;
  const LEVEL = 20;
  const MAX = 32;
  const WALL_HEIGHT = 128;
  const RIDGE = 40;
  const BED_HEIGHT = 24;
  const POOL = 6;
  const JITTER = 14;
  const CHEST = { hw: 20, hd: 14, box: 18, lid: 8 };

  // The legend, with the colors cells show when the room names no art.
  const LEGEND = [
    [".", "Floor", [168, 168, 168]],
    ["#", "Wall", [140, 140, 152]],
    ["r", "Rock", [128, 104, 80]],
    ["c", "Cabin wall", [150, 100, 60]],
    ["b", "Bed", [200, 70, 70]],
    ["p", "Pillow", [236, 236, 236]],
    ["t", "Chest", [160, 110, 50]],
    ["~", "Water", [80, 130, 210]],
    ["o", "Pot", [150, 95, 55]],
    ["x", "Blocked", [120, 120, 100]],
    ["0", "Door 0", [60, 60, 70]], ["1", "Door 1", [60, 60, 70]], ["2", "Door 2", [60, 60, 70]], ["3", "Door 3", [60, 60, 70]], ["4", "Door 4", [60, 60, 70]],
    ["5", "Door 5", [60, 60, 70]], ["6", "Door 6", [60, 60, 70]], ["7", "Door 7", [60, 60, 70]], ["8", "Door 8", [60, 60, 70]], ["9", "Door 9", [60, 60, 70]],
  ];
  const legendColor = Object.fromEntries(LEGEND.map(([c, , col]) => [c, col]));
  const legendName = Object.fromEntries(LEGEND.map(([c, name]) => [c, name]));
  const isDoor = (c) => c >= "0" && c <= "9";
  const isWall = (c) => c === "#" || c === "r";
  const isBed = (c) => c === "b" || c === "p";
  const isGround = (c) => !isWall(c) && c !== "c" && !isBed(c);
  const isWalkable = (c) => c === "." || c === "t" || isDoor(c);

  let host = null; // what the scene editor gives: doc(), changed, beginEdit, endEdit, field, group, vram, timFor, dataFiles, discPath, themeColor, sceneDats, select, selected, W, H, canvas, ctx, ids
  let W = 320, H = 240;

  // The editor's camera: an eighth of a turn, 1 the diagonal the game
  // starts at, the point on the floor it looks down on, in units, and a
  // zoom, 1 being the game's own distance.
  const view = { dir: 1, focus: [0, 0], zoom: 1 };
  let tool = "pen"; // pen, height, fill, rect, marker, look
  let brush = "."; // the rune the pen lays
  let hover = null; // the cell under the pointer {i, j}, or the rectangle being drawn
  let dragging = null;
  let lastLaid = null;
  let built = null; // the room's geometry, cached by key
  let builtKey = "";
  let markerPins = []; // where each marker is on the screen, for picking

  function attach(h) {
    host = h;
    W = h.W;
    H = h.H;
  }
  const doc = () => host.doc();
  const room = () => doc()?.room ?? null;

  // normalize fills in what a room file leaves out, so that the editor
  // has every field.
  function normalize(r) {
    if (!r || !Array.isArray(r.cells) || !r.cells.length) return null;
    const cols = Math.max(1, Math.min(MAX, Math.max(...r.cells.map((row) => row.length))));
    const cells = r.cells.slice(0, MAX).map((row) => (row + ".".repeat(cols)).slice(0, cols));
    let heights = null;
    if (Array.isArray(r.heights) && r.heights.length) {
      heights = cells.map((_, j) => ((r.heights[j] ?? "") + "0".repeat(cols)).slice(0, cols).replace(/[^0-9]/g, "0"));
    }
    return {
      floor: r.floor | 0, wall: r.wall | 0,
      tint: r.tint ?? [128, 128, 128],
      cells, heights,
      rough: !!r.rough, black: !!r.black,
      rockTall: r.rockTall | 0, rockLean: r.rockLean | 0,
      camUp: r.camUp | 0, camBack: r.camBack | 0,
      sky: r.sky ?? [[0, 0, 0], [0, 0, 0]],
      background: r.background ?? [0, 0, 0],
      links: (r.links ?? []).map((l) => ({ door: l.door | 0, scene: l.scene ?? "" })),
      markers: (r.markers ?? []).map((m) => ({ name: m.name ?? "", cell: m.cell ?? [0, 0], fields: m.fields ?? [0, 0] })),
      art: r.art ?? {},
    };
  }

  // out is the room as the file has it: the editor's blanks left out.
  function out(r) {
    const o = { floor: r.floor, wall: r.wall, tint: r.tint, cells: r.cells };
    if (r.heights) o.heights = r.heights;
    if (r.rough) o.rough = true;
    if (r.black) o.black = true;
    if (r.rockTall) o.rockTall = r.rockTall;
    if (r.rockLean) o.rockLean = r.rockLean;
    if (r.camUp) o.camUp = r.camUp;
    if (r.camBack) o.camBack = r.camBack;
    o.sky = r.sky;
    o.background = r.background;
    if (r.links.length) o.links = r.links.filter((l) => l.scene);
    if (r.markers.length) o.markers = r.markers.map((m) => (m.fields.some((c) => c) ? m : { name: m.name, cell: m.cell }));
    if (Object.values(r.art).some((v) => (Array.isArray(v) ? v.some(Boolean) : v))) o.art = r.art; // the editor's own; mkscene leaves it
    return o;
  }

  // A fresh room: a hall of stone with a doorway in its far wall.
  function template() {
    const cells = ["######1#####"];
    for (let j = 0; j < 8; j++) cells.push("#..........#");
    cells.push("############");
    return { floor: 0, wall: 0, tint: [128, 128, 128], cells, sky: [[0, 0, 0], [0, 0, 0]], background: [0, 0, 0] };
  }

  // The geometry, as stage works it out on entering a room.
  function build(r) {
    const key = JSON.stringify([r.cells, r.heights, r.rough, r.rockTall, r.rockLean]);
    if (built && builtKey === key) return built;
    const cols = r.cells[0].length, rows = r.cells.length;
    const cell = (i, j) => (j < 0 || j >= rows || i < 0 || i >= cols ? "#" : r.cells[j][i]);
    const level = (i, j) => {
      if (!r.heights) return 0;
      i = Math.max(0, Math.min(cols - 1, i));
      j = Math.max(0, Math.min(rows - 1, j));
      return r.heights[j].charCodeAt(i) - 48;
    };
    const rockHeight = r.rockTall || TILE;
    // Corner heights: the mean of the levels of the cells round the
    // corner that the cell is joined to without a cliff between.
    const corners = new Int16Array(rows * cols * 4);
    const cornerHeight = (i, j, k) => {
      if (!r.heights) return 0;
      const ci = i + (k & 1), cj = j + (k >> 1);
      const levels = [level(ci - 1, cj - 1), level(ci, cj - 1), level(ci - 1, cj), level(ci, cj)];
      const lo = Math.min(...levels), hi = Math.max(...levels);
      if (hi - lo <= 1) return Math.floor((levels[0] + levels[1] + levels[2] + levels[3]) * LEVEL / 4);
      const joined = [false, false, false, false];
      joined[3 - k] = true;
      for (let n = 0; n < 3; n++) {
        for (const [a, b] of [[0, 1], [2, 3], [0, 2], [1, 3]]) {
          if (joined[a] !== joined[b] && Math.abs(levels[a] - levels[b]) <= 1) joined[a] = joined[b] = true;
        }
      }
      let sum = 0, n = 0;
      for (let c = 0; c < 4; c++) if (joined[c]) { sum += levels[c]; n++; }
      return Math.floor(sum * LEVEL / n);
    };
    for (let j = 0; j < rows; j++) for (let i = 0; i < cols; i++) for (let k = 0; k < 4; k++) corners[(j * cols + i) * 4 + k] = cornerHeight(i, j, k);
    const h = (i, j, k) => corners[(j * cols + i) * 4 + k];
    // The corners' places, jittered in a rough room and round rock.
    const CW = cols + 1;
    const atX = new Float64Array((rows + 1) * CW), atZ = new Float64Array((rows + 1) * CW);
    let rocky = false;
    for (let j = 0; j < rows; j++) for (let i = 0; i < cols; i++) if (r.cells[j][i] === "r") rocky = true;
    const place = (ci, cj) => {
      let x = ci * TILE - cols * TILE / 2, z = rows * TILE / 2 - cj * TILE;
      const nearRock = cell(ci - 1, cj - 1) === "r" || cell(ci, cj - 1) === "r" || cell(ci - 1, cj) === "r" || cell(ci, cj) === "r";
      if (r.rough || nearRock) {
        let hh = (Math.imul(ci, 374761393) + Math.imul(cj, 668265263)) >>> 0;
        hh = Math.imul(hh ^ (hh >>> 13), 1274126177) >>> 0;
        x += hh % (2 * JITTER + 1) - JITTER;
        z += (hh >>> 8) % (2 * JITTER + 1) - JITTER;
      }
      return [x, z];
    };
    const edges = new Uint8Array((rows + 1) * CW), ring = new Uint8Array((rows + 1) * CW);
    const borders = (i, j) => isGround(cell(i + 1, j)) || isGround(cell(i - 1, j)) || isGround(cell(i, j + 1)) || isGround(cell(i, j - 1));
    for (let cj = 0; cj <= rows; cj++) {
      for (let ci = 0; ci <= cols; ci++) {
        const [x, z] = place(ci, cj);
        atX[cj * CW + ci] = x;
        atZ[cj * CW + ci] = z;
        if (!rocky) continue;
        let rock = false, open = false, inRing = false;
        for (let k = 0; k < 4; k++) {
          const i = ci - 1 + (k & 1), j = cj - 1 + (k >> 1);
          const c = cell(i, j);
          rock = rock || c === "r";
          open = open || isGround(c);
          if (c === "r" && borders(i, j)) inRing = true;
        }
        edges[cj * CW + ci] = rock && open ? 1 : 0;
        ring[cj * CW + ci] = inRing ? 1 : 0;
      }
    }
    const smooth = (xs, zs, which, passes) => {
      for (let pass = 0; pass < passes; pass++) {
        const nx = xs.slice(), nz = zs.slice();
        for (let cj = 0; cj <= rows; cj++) {
          for (let ci = 0; ci <= cols; ci++) {
            if (!which[cj * CW + ci]) continue;
            let sx = 0, sz = 0, n = 0;
            for (const [dx, dz] of [[1, 0], [-1, 0], [0, 1], [0, -1]]) {
              const ni = ci + dx, nj = cj + dz;
              if (ni >= 0 && nj >= 0 && ni <= cols && nj <= rows && which[nj * CW + ni]) {
                sx += xs[nj * CW + ni];
                sz += zs[nj * CW + ni];
                n++;
              }
            }
            if (n > 0) {
              nx[cj * CW + ci] = Math.trunc((xs[cj * CW + ci] * n + sx) / (2 * n));
              nz[cj * CW + ci] = Math.trunc((zs[cj * CW + ci] * n + sz) / (2 * n));
            }
          }
        }
        xs.set(nx);
        zs.set(nz);
      }
    };
    if (rocky) smooth(atX, atZ, edges, 2);
    const topX = atX.slice(), topZ = atZ.slice();
    if (r.rockLean) {
      for (let cj = 0; cj <= rows; cj++) {
        for (let ci = 0; ci <= cols; ci++) {
          let sx = 0, sz = 0, rock = false;
          for (let k = 0; k < 4; k++) {
            const di = (k & 1) * 2 - 1, dj = (k >> 1) * 2 - 1;
            const c = cell(ci + ((di - 1) >> 1), cj + ((dj - 1) >> 1));
            rock = rock || c === "r";
            if (isGround(c)) { sx -= di; sz += dj; }
          }
          if (!rock || (!sx && !sz)) continue;
          const n = Math.max(Math.abs(sx), Math.abs(sz));
          topX[cj * CW + ci] += Math.trunc(r.rockLean * sx / n);
          topZ[cj * CW + ci] += Math.trunc(r.rockLean * sz / n);
        }
      }
    }
    if (rocky) smooth(topX, topZ, ring, 2);
    // The lowest ground at each corner, where a cliff face ends, and
    // which sides of each cell drop to a lower neighbor.
    const low = new Int16Array((rows + 1) * CW);
    for (let cj = 0; cj <= rows; cj++) {
      for (let ci = 0; ci <= cols; ci++) {
        let lowest = 32767;
        for (let k = 0; k < 4; k++) {
          const i = ci - 1 + (k & 1), j = cj - 1 + (k >> 1);
          if (i >= 0 && j >= 0 && i < cols && j < rows) lowest = Math.min(lowest, h(i, j, 3 - k));
          else lowest = 0;
        }
        low[cj * CW + ci] = lowest;
      }
    }
    const SIDES = [[0, 1], [0, -1], [1, 0], [-1, 0]]; // towards -Z, +Z, +X, -X as steps in column and row
    const ACROSS = [[[2, 0], [3, 1]], [[1, 3], [0, 2]], [[3, 2], [1, 0]], [[0, 1], [2, 3]]];
    const dropMask = new Uint8Array(rows * cols);
    for (let j = 0; j < rows; j++) {
      for (let i = 0; i < cols; i++) {
        for (let k = 0; k < 4; k++) {
          const ni = i + SIDES[k][0], nj = j + SIDES[k][1];
          const [a, b] = ACROSS[k];
          let ha = 0, hb = 0;
          if (ni >= 0 && nj >= 0 && ni < cols && nj < rows) { ha = h(ni, nj, a[1]); hb = h(ni, nj, b[1]); }
          if (h(i, j, a[0]) > ha || h(i, j, b[0]) > hb) dropMask[j * cols + i] |= 1 << k;
        }
      }
    }
    // The cabins, numbered by flood fill, with each one's rows.
    const blocks = new Uint8Array(rows * cols);
    const blockRows = [null];
    const fill = (i, j, n) => {
      const stack = [[i, j]];
      while (stack.length) {
        const [x, y] = stack.pop();
        if (cell(x, y) !== "c" || blocks[y * cols + x]) continue;
        blocks[y * cols + x] = n;
        blockRows[n][0] = Math.min(blockRows[n][0], y);
        blockRows[n][1] = Math.max(blockRows[n][1], y);
        stack.push([x + 1, y], [x - 1, y], [x, y + 1], [x, y - 1]);
      }
    };
    for (let j = 0; j < rows; j++) for (let i = 0; i < cols; i++) if (r.cells[j][i] === "c" && !blocks[j * cols + i] && blockRows.length < 16) { blockRows.push([j, j]); fill(i, j, blockRows.length - 1); }
    built = { cols, rows, cell, level, rockHeight, corners, h, CW, atX, atZ, topX, topZ, low, dropMask, blocks, blockRows, SIDES, ACROSS, borders };
    builtKey = key;
    return built;
  }

  // heightAt is the ground's height under a point, between its cell's
  // corners.
  function heightAt(b, x, z) {
    const u = Math.max(0, Math.min(b.cols * TILE - 1, x + b.cols * TILE / 2));
    const v = Math.max(0, Math.min(b.rows * TILE - 1, b.rows * TILE / 2 - z));
    const i = Math.floor(u / TILE), j = Math.floor(v / TILE);
    const fx = (u % TILE) / TILE, fz = (v % TILE) / TILE;
    const tl = b.h(i, j, 0), tr = b.h(i, j, 1), bl = b.h(i, j, 2), br = b.h(i, j, 3);
    const top = tl + (tr - tl) * fx, bottom = bl + (br - bl) * fx;
    return top + (bottom - top) * fz;
  }

  // The camera: behind and above the focus, turned about it by its
  // eighth, looking a little above the floor, as the game's camera sits
  // behind its hero; the basis is what the scene editor's projection takes.
  function camera() {
    const r = room();
    const b = build(r);
    const up = (r.camUp || 380) * view.zoom, back = (r.camBack || 440) * view.zoom;
    const angle = view.dir * Math.PI / 4;
    const ahead = [Math.sin(angle), Math.cos(angle)];
    const [fx, fz] = view.focus;
    const fy = -heightAt(b, fx, fz);
    const from = [fx - ahead[0] * back, fy - up, fz - ahead[1] * back];
    const at = [fx, fy - 20, fz];
    let f = [at[0] - from[0], at[1] - from[1], at[2] - from[2]];
    let len = Math.hypot(...f) || 1;
    f = f.map((c) => c / len);
    let rt = [f[2], 0, -f[0]];
    len = Math.hypot(...rt) || 1;
    rt = rt.map((c) => c / len);
    const u = [f[1] * rt[2] - f[2] * rt[1], f[2] * rt[0] - f[0] * rt[2], f[0] * rt[1] - f[1] * rt[0]];
    return { from, f, r: rt, u, h: 320, ahead, at };
  }
  const toCamera = (cam, p) => {
    const d = [p[0] - cam.from[0], p[1] - cam.from[1], p[2] - cam.from[2]];
    const dot = (a, b) => a[0] * b[0] + a[1] * b[1] + a[2] * b[2];
    return [dot(d, cam.r), dot(d, cam.u), dot(d, cam.f)];
  };
  const toScreen = (cam, c) => (c[2] <= 1 ? null : [W / 2 + c[0] * cam.h / c[2], H / 2 + c[1] * cam.h / c[2], c[2]]);

  // Textures, when the room's art names TIMs for them: the quad is cut
  // from the whole 64 by 64 picture.
  function skin(path) {
    if (!path) return null;
    const tim = host.timFor(path);
    if (!tim) return null;
    const [u0, v0] = preview.uv(tim);
    return { tpage: preview.tpage(tim), cba: preview.cba(tim), uv: [[u0, v0], [u0 + 63, v0], [u0, v0 + 63], [u0 + 63, v0 + 63]] };
  }
  function art(r) {
    const a = r.art;
    return {
      floor: skin(a.floors?.[r.floor]), wall: skin(a.walls?.[r.wall]),
      rock: skin(a.rock) ?? skin(a.walls?.[r.wall]), logs: skin(a.logs), roof: skin(a.roof), water: skin(a.water),
    };
  }

  // draw draws the room into the picture, each cell's pieces as quads
  // sorted back to front, with the cell's id in ids so that it can be
  // picked: 1000 and the cell's number.
  function draw(outBuf, ids, cam, vram) {
    const r = room();
    const b = build(r);
    const skins = art(r);
    const quads = [];
    const tint = r.tint.map((c) => c / 128);
    const mul = (c, k) => [c[0] * k, c[1] * k, c[2] * k];
    // quad queues a quad, corners top left, top right, bottom left, bottom
    // right as seen from its front, in world units; one-sided unless
    // ground.
    const quad = (pts, color, tex, id, { ground = false, back = 0, shade = 1 } = {}) => {
      const cs = pts.map((p) => toCamera(cam, p));
      if (cs.some((c) => c[2] <= 8)) return;
      const ss = cs.map((c) => toScreen(cam, c));
      const z = (cs[0][2] + cs[1][2] + cs[2][2] + cs[3][2]) / 4 + back;
      quads.push({ ss, z, color: mul(color, shade), tex, id, ground, shade });
    };
    const tri = (pts, color, id, shade = 1) => {
      const cs = pts.map((p) => toCamera(cam, p));
      if (cs.some((c) => c[2] <= 8)) return;
      const ss = cs.map((c) => toScreen(cam, c));
      quads.push({ ss, z: (cs[0][2] + cs[1][2] + cs[2][2]) / 3, color: mul(color, shade), tex: null, id, ground: false, tri: true, shade });
    };
    const corner = (i, j, k, which = "at") => {
      const ci = i + (k & 1), cj = j + (k >> 1);
      return which === "at" ? [b.atX[cj * b.CW + ci], b.atZ[cj * b.CW + ci]] : [b.topX[cj * b.CW + ci], b.topZ[cj * b.CW + ci]];
    };
    const floorColor = mul(legendColor["."], 1).map((c, k) => c * tint[k]);
    const wallColor = legendColor["#"], rockColor = legendColor.r;
    const wallSide = (k) => [[2, 3, 120], [1, 0, 120], [3, 1, 100], [0, 2, 100]][k];
    for (let j = 0; j < b.rows; j++) {
      for (let i = 0; i < b.cols; i++) {
        const c = r.cells[j][i];
        const id = 1000 + j * b.cols + i;
        const hs = [0, 1, 2, 3].map((k) => b.h(i, j, k));
        const ats = [0, 1, 2, 3].map((k) => corner(i, j, k));
        const rock = c === "r" || isDoor(c);
        const tops = [0, 1, 2, 3].map((k) => (rock ? corner(i, j, k, "top") : ats[k]));
        const P = (k, y, lean = 0, height = 1) => {
          // Corner k at height y up from the ground, leaned towards the
          // top by lean/height.
          const [ax, az] = ats[k], [tx, tz] = tops[k];
          const t = lean > 0 ? lean / height : 0;
          return [ax + (tx - ax) * t, -hs[k] - y, az + (tz - az) * t];
        };
        // Cliff faces down to lower neighbors.
        const mask = b.dropMask[j * b.cols + i];
        for (let k = 0; k < 4 && mask; k++) {
          if (!(mask & (1 << k))) continue;
          const [a, bb] = b.ACROSS[k];
          const ha = b.low[(j + (a[0] >> 1)) * b.CW + i + (a[0] & 1)], hb = b.low[(j + (bb[0] >> 1)) * b.CW + i + (bb[0] & 1)];
          quad([P(a[0], 0), P(bb[0], 0), [ats[a[0]][0], -ha, ats[a[0]][1]], [ats[bb[0]][0], -hb, ats[bb[0]][1]]], wallColor, skins.wall, id, { shade: 104 / 128 });
        }
        if (isGround(c)) {
          const [tl, tr, bl, br] = hs;
          const bright = Math.max(64, Math.min(200, 128 + (tr + br - tl - bl) / 2 + (tl + tr - bl - br) / 2)) / 128;
          let color = floorColor, tex = skins.floor;
          const rise = Math.max(tl - tr, tr - tl, bl - br, br - bl, tl - bl, bl - tl, tr - br, br - tr);
          if (isDoor(c)) color = [48, 48, 56].map((v) => v * 168 / 128);
          else if (rise * 3 > LEVEL * 4) { color = mul(rockColor, 120 / 128); tex = skins.wall; }
          else if (c === "x") color = mul(floorColor, 0.8);
          if (c === "~") {
            quad([P(0, -2 * POOL), P(1, -2 * POOL), P(2, -2 * POOL), P(3, -2 * POOL)], color, tex, id, { ground: true, back: 8, shade: bright * 2 / 3 });
            quad([P(0, -POOL), P(1, -POOL), P(2, -POOL), P(3, -POOL)], legendColor["~"], skins.water, id, { ground: true, back: 7 });
          } else {
            quad([P(0, 0), P(1, 0), P(2, 0), P(3, 0)], color, tex, id, { ground: true, back: 8, shade: bright });
          }
          const mid = [(ats[0][0] + ats[3][0]) / 2, (ats[0][1] + ats[3][1]) / 2];
          const y0 = -heightAt(b, mid[0], mid[1]);
          const box = (hw, hd, height, color, lift = 0, texTop = null) => {
            const x0 = mid[0] - hw, x1 = mid[0] + hw, z0 = mid[1] - hd, z1 = mid[1] + hd;
            const top = y0 - lift - height, bot = y0 - lift;
            quad([[x0, top, z1], [x1, top, z1], [x0, top, z0], [x1, top, z0]], color, texTop, id, { shade: 1 });
            for (const [ax, az, bx, bz, sh] of [[x0, z0, x1, z0, 0.9], [x1, z1, x0, z1, 0.9], [x1, z0, x1, z1, 0.75], [x0, z1, x0, z0, 0.75]]) {
              quad([[ax, top, az], [bx, top, bz], [ax, bot, az], [bx, bot, bz]], color, null, id, { shade: sh });
            }
          };
          if (c === "t") box(CHEST.hw, CHEST.hd, CHEST.box + CHEST.lid, legendColor.t);
          if (c === "o") box(10, 10, 26, legendColor.o);
          continue;
        }
        if (isBed(c)) {
          const x0 = i * TILE - b.cols * TILE / 2, x1 = x0 + TILE, z1 = b.rows * TILE / 2 - j * TILE, z0 = z1 - TILE;
          quad([[x0, -BED_HEIGHT, z1], [x1, -BED_HEIGHT, z1], [x0, -BED_HEIGHT, z0], [x1, -BED_HEIGHT, z0]], legendColor[c], null, id);
          const rect = [[x0, z0, x1, z0], [x1, z1, x0, z1], [x1, z0, x1, z1], [x0, z1, x0, z0]];
          for (let k = 0; k < 4; k++) {
            const n = b.cell(i + b.SIDES[k][0], j + b.SIDES[k][1]);
            if (isBed(n)) continue;
            const [ax, az, bx, bz] = rect[k];
            quad([[ax, -BED_HEIGHT, az], [bx, -BED_HEIGHT, bz], [ax, 0, az], [bx, 0, bz]], legendColor.c, null, id, { shade: 110 / 128 });
          }
          continue;
        }
        // Walls, rock and cabins: the sides onto the ground, and the top.
        const cabin = c === "c";
        let color = wallColor, tex = skins.wall, height = WALL_HEIGHT, topColor = [72, 72, 80];
        if (c === "r") { color = rockColor; tex = skins.rock; height = b.rockHeight; topColor = mul(rockColor, 100 / 128); }
        if (cabin) { color = legendColor.c; tex = skins.logs; }
        for (let k = 0; k < 4; k++) {
          const n = b.cell(i + b.SIDES[k][0], j + b.SIDES[k][1]);
          if (!isGround(n)) continue;
          const [a, bb, sh] = wallSide(k);
          const shade = isDoor(n) && c === "r" ? 36 / 128 : sh / 128;
          for (let top = height; top > 0; top -= TILE) {
            quad([P(a, top, top, height), P(bb, top, top, height), P(a, top - TILE, top - TILE, height), P(bb, top - TILE, top - TILE, height)], color, tex, id, { shade });
          }
        }
        if (cabin) {
          const blk = b.blocks[j * b.cols + i];
          const [r0, r1] = b.blockRows[blk];
          const n = r1 - r0 + 1, k = j - r0;
          const rise = (kk) => Math.floor(Math.min(kk, n - kk) * 2 * RIDGE / n);
          const far = rise(k), near = rise(k + 1);
          const x0 = i * TILE - b.cols * TILE / 2, x1 = x0 + TILE, z1 = b.rows * TILE / 2 - j * TILE, z0 = z1 - TILE;
          quad([[x0, -WALL_HEIGHT - far, z1], [x1, -WALL_HEIGHT - far, z1], [x0, -WALL_HEIGHT - near, z0], [x1, -WALL_HEIGHT - near, z0]], [150, 60, 50], skins.roof, id, { shade: near > far ? 96 / 128 : 1 });
          for (const side of [-1, 1]) {
            if (b.cell(i + side, j) === "c") continue;
            const x = side < 0 ? x0 : x1;
            const [tl, tr, zl, zr] = side < 0 ? [far, near, z1, z0] : [near, far, z0, z1];
            const top = (t, z) => [x, -WALL_HEIGHT - t, z];
            if (tl === 0 && tr === 0) continue;
            if (tl === 0) tri([top(tr, zr), top(0, zr), top(0, zl)], legendColor.c, id, 100 / 128);
            else if (tr === 0) tri([top(tl, zl), top(0, zr), top(0, zl)], legendColor.c, id, 100 / 128);
            else quad([top(tl, zl), top(tr, zr), top(0, zl), top(0, zr)], legendColor.c, null, id, { shade: 100 / 128 });
          }
          continue;
        }
        if (c === "r" && r.black && !b.borders(i, j)) continue;
        quad([P(0, height, height, height), P(1, height, height, height), P(2, height, height, height), P(3, height, height, height)], topColor, c === "r" ? tex : null, id, { shade: c === "r" ? 100 / 128 : 1 });
      }
    }
    quads.sort((p, q) => q.z - p.z);
    const lit = { lit: true, texture: null };
    for (const q of quads) {
      const { ss, color, tex, id, ground, tri: isTri } = q;
      const idx = isTri ? [[0, 1, 2]] : [[0, 1, 2], [1, 3, 2]];
      for (const [a, bb, c] of idx) {
        const [x0, y0] = ss[a], [x1, y1] = ss[bb], [x2, y2] = ss[c];
        const area = (x1 - x0) * (y2 - y0) - (y1 - y0) * (x2 - x0);
        if (area === 0 || (area < 0 && !ground)) continue;
        const shades = [[q.shade, q.shade, q.shade], [q.shade, q.shade, q.shade], [q.shade, q.shade, q.shade]];
        const cols = [color, color, color];
        if (tex) {
          lit.texture = tex;
          preview.fillTriangle(outBuf, ids, id, x0, y0, x1, y1, x2, y2, area, cols, shades, [tex.uv[a], tex.uv[bb], tex.uv[c]], lit, vram);
        } else {
          preview.fillTriangle(outBuf, ids, id, x0, y0, x1, y1, x2, y2, area, cols, [[1, 1, 1], [1, 1, 1], [1, 1, 1]], null, lit, vram);
        }
      }
    }
    // Where the markers stand, for the overlay and for picking.
    markerPins = r.markers.map((m, k) => {
      const [i, j] = m.cell;
      const x = i * TILE - b.cols * TILE / 2 + TILE / 2, z = b.rows * TILE / 2 - j * TILE - TILE / 2;
      const y = i < 0 || j < 0 || i >= b.cols || j >= b.rows ? 0 : -heightAt(b, x, z);
      const foot = toScreen(cam, toCamera(cam, [x, y, z]));
      const head = toScreen(cam, toCamera(cam, [x, y - 40, z]));
      return { k, foot, head, name: m.name };
    });
  }

  // overlay draws what sits on the picture: the markers as pins with
  // their names, the focus, and the cell under the pointer.
  function overlay(ctx, cam) {
    const r = room();
    const b = build(r);
    const ink = host.themeColor("--ink"), accent = host.themeColor("--accent");
    ctx.lineWidth = 1;
    const sel = host.selected();
    for (const pin of markerPins) {
      if (!pin.foot || !pin.head) continue;
      const on = sel?.kind === "marker" && sel.index === pin.k;
      ctx.strokeStyle = on ? accent : ink;
      ctx.fillStyle = on ? accent : host.themeColor("--triangle");
      ctx.lineWidth = on ? 2 : 1;
      ctx.beginPath();
      ctx.moveTo(pin.foot[0], pin.foot[1]);
      ctx.lineTo(pin.head[0], pin.head[1]);
      ctx.stroke();
      ctx.beginPath();
      ctx.arc(pin.head[0], pin.head[1], 4, 0, 2 * Math.PI);
      ctx.fill();
      ctx.font = "10px sans-serif";
      ctx.fillStyle = ink;
      ctx.fillText(pin.name || "marker", pin.head[0] + 6, pin.head[1] + 3);
    }
    ctx.lineWidth = 1;
    // The focus: a small figure's worth of box where the camera looks.
    const [fx, fz] = view.focus;
    const fy = -heightAt(b, fx, fz);
    const p0 = toScreen(cam, toCamera(cam, [fx, fy, fz])), p1 = toScreen(cam, toCamera(cam, [fx, fy - 40, fz]));
    if (p0 && p1) {
      ctx.strokeStyle = accent;
      ctx.strokeRect(Math.round(p0[0]) - 6.5, Math.round(p1[1]) + 0.5, 13, Math.max(1, p0[1] - p1[1]));
    }
    // The cell under the pointer, or the rectangle being drawn.
    const outlineCell = (i, j, color) => {
      const pts = [0, 1, 3, 2].map((k) => {
        const ci = i + (k & 1), cj = j + (k >> 1);
        return toScreen(cam, toCamera(cam, [b.atX[cj * b.CW + ci], -b.h(i, j, k), b.atZ[cj * b.CW + ci]]));
      });
      if (pts.some((p) => !p)) return;
      ctx.strokeStyle = color;
      ctx.beginPath();
      pts.forEach((p, k) => (k ? ctx.lineTo(p[0], p[1]) : ctx.moveTo(p[0], p[1])));
      ctx.closePath();
      ctx.stroke();
    };
    if (hover?.rect) {
      const [x0, y0, x1, y1] = hover.rect;
      for (let j = Math.min(y0, y1); j <= Math.max(y0, y1); j++) for (let i = Math.min(x0, x1); i <= Math.max(x0, x1); i++) outlineCell(i, j, accent);
    } else if (hover) {
      outlineCell(hover.i, hover.j, ink);
    }
  }

  // status is what the line under the picture says: the tool in hand and
  // the cell under the pointer, and the room's counts beside it.
  function status() {
    const r = room();
    const b = build(r);
    const under = hover && !hover.rect ? `Cell ${hover.i}, ${hover.j}: ${legendName[b.cell(hover.i, hover.j)] ?? "wall"}${r.heights ? `, level ${b.level(hover.i, hover.j)}` : ""}. ` : "";
    const name = legendName[brush];
    const t = tool === "pen" ? `The pen lays ${name}; Shift draws a line, Alt-click takes a cell's brush.` : tool === "height" ? "A click raises the ground a level; Shift lowers it." : tool === "fill" ? `The fill floods cells like the one clicked with ${name}.` : tool === "rect" ? `The rectangle fills what you drag over with ${name}.` : tool === "marker" ? "A click puts a marker on a cell; drag one to move it." : "A click turns the camera on a cell.";
    const doors = new Set(r.cells.join("").replace(/[^0-9]/g, "")).size;
    return [under + t, `${b.cols} × ${b.rows} cells · ${r.markers.length} marker${r.markers.length === 1 ? "" : "s"} · ${doors} doorway${doors === 1 ? "" : "s"}`];
  }

  // Editing the cells.
  function setCell(r, i, j, c) {
    const b = build(r);
    if (i < 0 || j < 0 || i >= b.cols || j >= b.rows || r.cells[j][i] === c) return false;
    r.cells[j] = r.cells[j].slice(0, i) + c + r.cells[j].slice(i + 1);
    return true;
  }
  function setLevel(r, i, j, lv) {
    const b = build(r);
    if (i < 0 || j < 0 || i >= b.cols || j >= b.rows) return false;
    if (!r.heights) r.heights = r.cells.map((row) => "0".repeat(row.length));
    lv = Math.max(0, Math.min(9, lv));
    if (r.heights[j][i] === String(lv)) return false;
    r.heights[j] = r.heights[j].slice(0, i) + lv + r.heights[j].slice(i + 1);
    if (r.heights.every((row) => !/[1-9]/.test(row))) r.heights = null;
    return true;
  }
  function line(x0, y0, x1, y1, f) {
    let any = false;
    const dx = Math.abs(x1 - x0), dy = -Math.abs(y1 - y0);
    const sx = x0 < x1 ? 1 : -1, sy = y0 < y1 ? 1 : -1;
    let err = dx + dy, x = x0, y = y0;
    for (;;) {
      any = f(x, y) || any;
      if (x === x1 && y === y1) break;
      const e2 = 2 * err;
      if (e2 >= dy) { err += dy; x += sx; }
      if (e2 <= dx) { err += dx; y += sy; }
    }
    return any;
  }
  function flood(r, i, j, c) {
    const b = build(r);
    const was = b.cell(i, j);
    if (was === c || i < 0 || j < 0 || i >= b.cols || j >= b.rows) return false;
    const stack = [[i, j]];
    while (stack.length) {
      const [x, y] = stack.pop();
      if (x < 0 || y < 0 || x >= b.cols || y >= b.rows || r.cells[y][x] !== was) continue;
      setCell(r, x, y, c);
      stack.push([x + 1, y], [x - 1, y], [x, y + 1], [x, y - 1]);
    }
    return true;
  }
  // apply does the tool's work on a cell: the pen and the heights lay as
  // the pointer passes; the fill and the rectangle work from the press.
  function applyAt(r, i, j, e, first) {
    const b = build(r);
    if (i < 0 || j < 0 || i >= b.cols || j >= b.rows) return false;
    if (tool === "pen") {
      const any = e.shiftKey && lastLaid && first ? line(lastLaid[0], lastLaid[1], i, j, (x, y) => setCell(r, x, y, brush)) : lastLaid && !first ? line(lastLaid[0], lastLaid[1], i, j, (x, y) => setCell(r, x, y, brush)) : setCell(r, i, j, brush);
      lastLaid = [i, j];
      return any;
    }
    if (tool === "height") {
      if (!first && lastLaid && lastLaid[0] === i && lastLaid[1] === j) return false;
      lastLaid = [i, j];
      return setLevel(r, i, j, b.level(i, j) + (e.shiftKey ? -1 : 1));
    }
    if (tool === "fill" && first) return flood(r, i, j, brush);
    return false;
  }
  // cellAt is the cell under a point of the picture, by the ids the
  // drawing left, or null.
  function cellAt(x, y, ids) {
    const px = Math.max(0, Math.min(W - 1, Math.floor(x))), py = Math.max(0, Math.min(H - 1, Math.floor(y)));
    const id = ids[py * W + px];
    if (id < 1000) return null;
    const b = build(room());
    const n = id - 1000;
    return { i: n % b.cols, j: Math.floor(n / b.cols) };
  }
  function markerAt(x, y) {
    for (const pin of markerPins) {
      if (pin.head && Math.hypot(x - pin.head[0], y - pin.head[1]) < 7) return pin.k;
    }
    return -1;
  }

  // The pointer on the picture. pointerDown returns the handlers for
  // the drag it starts, or null when it is not the room's.
  function pointerDown(x, y, e, ids) {
    const r = room();
    if (!r || e.button !== 0 || e.altKey || e.ctrlKey) return null;
    const mk = markerAt(x, y);
    if (mk >= 0 && tool !== "pen" && tool !== "height") {
      host.select({ kind: "marker", index: mk });
      // Drag it from cell to cell.
      return {
        move: (cx, cy) => {
          const c = cellAt(cx, cy, ids);
          const m = r.markers[mk];
          if (c && (m.cell[0] !== c.i || m.cell[1] !== c.j)) {
            m.cell = [c.i, c.j];
            host.changed();
          }
        },
        up: () => {},
      };
    }
    const c = cellAt(x, y, ids);
    if (!c) {
      if (tool !== "pen" && tool !== "height") host.select(null);
      return { move: () => {}, up: () => {} };
    }
    if (tool === "look") {
      lookAt(c.i, c.j);
      return { move: () => {}, up: () => {} };
    }
    if (tool === "marker") {
      const b = build(r);
      r.markers.push({ name: uniqueMarker(r), cell: [c.i, c.j], fields: [0, 0] });
      host.select({ kind: "marker", index: r.markers.length - 1 });
      host.changed(true);
      renderPanel();
      void b;
      return { move: () => {}, up: () => {} };
    }
    if (tool === "rect") {
      hover = { rect: [c.i, c.j, c.i, c.j] };
      return {
        move: (cx, cy) => {
          const d = cellAt(cx, cy, ids);
          if (d) { hover = { rect: [c.i, c.j, d.i, d.j] }; host.render(); }
        },
        up: () => {
          const [x0, y0, x1, y1] = hover.rect;
          let any = false;
          for (let j = Math.min(y0, y1); j <= Math.max(y0, y1); j++) for (let i = Math.min(x0, x1); i <= Math.max(x0, x1); i++) any = setCell(r, i, j, brush) || any;
          hover = null;
          if (any) host.changed(true);
          else host.render();
        },
      };
    }
    if (tool !== "pen" && tool !== "height" && tool !== "fill") return null;
    if (applyAt(r, c.i, c.j, e, true)) host.changed();
    return {
      move: (cx, cy, ev) => {
        const d = cellAt(cx, cy, ids);
        if (!d) return;
        hover = d;
        if (applyAt(r, d.i, d.j, ev, false)) host.changed();
        else host.render();
      },
      up: () => {
        if (tool !== "pen") lastLaid = null;
        // A doorway painted or painted over changes the links offered.
        renderPanel();
      },
    };
  }
  function pointerHover(x, y, ids) {
    const r = room();
    if (!r) return false;
    const c = cellAt(x, y, ids);
    const next = c ? { i: c.i, j: c.j } : null;
    if (JSON.stringify(next) === JSON.stringify(hover)) return false;
    hover = next;
    return true;
  }
  function pointerLeave() {
    if (hover && !dragging) { hover = null; return true; }
    return false;
  }

  // The camera's moves.
  function turn(steps) {
    view.dir = ((view.dir + steps) % 8 + 8) % 8;
    host.render();
    renderPanel();
  }
  function lookAt(i, j) {
    const b = build(room());
    view.focus = [i * TILE - b.cols * TILE / 2 + TILE / 2, b.rows * TILE / 2 - j * TILE - TILE / 2];
    host.render();
  }
  function pan(dx, dy) {
    // Across the screen is the camera's right; up the screen is ahead.
    const cam = camera();
    const k = 2 * view.zoom;
    view.focus = [view.focus[0] - dx * k * cam.r[0] + dy * k * cam.ahead[0], view.focus[1] - dx * k * cam.r[2] + dy * k * cam.ahead[1]];
    clampFocus();
    host.render();
  }
  function clampFocus() {
    const b = build(room());
    view.focus = [Math.max(-b.cols * TILE / 2, Math.min(b.cols * TILE / 2, view.focus[0])), Math.max(-b.rows * TILE / 2, Math.min(b.rows * TILE / 2, view.focus[1]))];
  }
  function zoom(k) {
    view.zoom = Math.max(0.4, Math.min(4, view.zoom * k));
    host.render();
  }
  function keys(e) {
    const r = room();
    if (!r || e.ctrlKey || e.metaKey || e.altKey) return false;
    const k = e.key.toLowerCase();
    const tools = { b: "pen", h: "height", f: "fill", r: "rect", m: "marker", l: "look" };
    if (tools[k]) { setTool(tools[k]); return true; }
    if (k === "p") { playHere(); return true; }
    if (e.key === "[") { turn(1); return true; }
    if (e.key === "]") { turn(-1); return true; }
    const sel = host.selected();
    const b = build(r);
    const moves = { ArrowLeft: [-1, 0], ArrowRight: [1, 0], ArrowUp: [0, 1], ArrowDown: [0, -1] };
    if (moves[e.key]) {
      e.preventDefault();
      const cam = camera();
      // Along the camera's right and ahead, in whole cells.
      const [mx, mz] = moves[e.key];
      const dx = mx * cam.r[0] + mz * cam.ahead[0], dz = mx * cam.r[2] + mz * cam.ahead[1];
      if (sel?.kind === "marker" && r.markers[sel.index]) {
        const m = r.markers[sel.index];
        m.cell = [Math.max(0, Math.min(b.cols - 1, m.cell[0] + Math.round(dx))), Math.max(0, Math.min(b.rows - 1, m.cell[1] - Math.round(dz)))];
        host.changed();
        renderPanel();
      } else {
        view.focus = [view.focus[0] + Math.round(dx) * TILE, view.focus[1] + Math.round(dz) * TILE];
        clampFocus();
        host.render();
      }
      return true;
    }
    if ((e.key === "Delete" || e.key === "Backspace") && sel?.kind === "marker") {
      e.preventDefault();
      r.markers.splice(sel.index, 1);
      host.select(null);
      host.changed(true);
      renderPanel();
      return true;
    }
    return false;
  }
  // playHere asks the running program to put its figure on the cell
  // the camera looks at, through the scene package's live hook.
  function playHere() {
    const r = room();
    if (!r) return;
    const b = build(r);
    const i = Math.max(0, Math.min(b.cols - 1, Math.floor((view.focus[0] + b.cols * TILE / 2) / TILE)));
    const j = Math.max(0, Math.min(b.rows - 1, Math.floor((b.rows * TILE / 2 - view.focus[1]) / TILE)));
    host.teleport(i, j);
  }
  function setTool(t) {
    tool = t;
    for (const b of document.querySelectorAll("#scene-room-tools button")) b.setAttribute("aria-pressed", String(b.dataset.tool === t));
    // Looking at a cell is in the view menu, which shows it in hand.
    $("scene-view-menu").setAttribute("aria-pressed", String(t === "look"));
    host.render();
  }
  function setBrush(c) {
    brush = c;
    if (tool !== "pen" && tool !== "fill" && tool !== "rect") setTool("pen");
    for (const b of document.querySelectorAll("#scene-room-brushes button, #scene-room-doors button")) b.setAttribute("aria-pressed", String(b.dataset.rune === c));
    renderDoors();
    renderInfo();
    host.render();
  }
  // renderInfo says under the brushes where a doorway brush leads.
  function renderInfo() {
    const r = room();
    const link = r?.links.find((l) => String(l.door) === brush);
    info().textContent = !isDoor(brush) ? "" : `Doorway ${brush} is the brush: cells that lead ${link ? `to ${link.scene.replace(/^.*\//, "")}, at its doorway ${brush}` : "nowhere yet; say where under Doorways lead to"}.`;
  }
  // resetView puts the camera back where the game starts it.
  function resetView() {
    view.dir = 1;
    view.focus = [0, 0];
    view.zoom = 1;
    host.render();
  }
  // start makes the scene a room, from the template.
  function start() {
    doc().room = normalize(template());
    resetView();
    host.changed(true);
    renderPanel();
  }
  // addMarker puts a marker on the cell the camera looks at.
  function addMarker() {
    const r = room();
    if (!r) return;
    const b = build(r);
    const i = Math.max(0, Math.min(b.cols - 1, Math.floor((view.focus[0] + b.cols * TILE / 2) / TILE)));
    const j = Math.max(0, Math.min(b.rows - 1, Math.floor((b.rows * TILE / 2 - view.focus[1]) / TILE)));
    r.markers.push({ name: uniqueMarker(r), cell: [i, j], fields: [0, 0] });
    host.select({ kind: "marker", index: r.markers.length - 1 });
    host.changed(true);
    renderPanel();
  }
  function uniqueMarker(r) {
    const taken = new Set(r.markers.map((m) => m.name));
    for (let n = 1; ; n++) if (!taken.has(`marker ${n}`)) return `marker ${n}`;
  }

  // The outline's rows: the room and its markers.
  function outlineRows(row, head, matches) {
    const r = room();
    if (!r) return;
    const b = build(r);
    if (matches("room")) row("Room", `${b.cols} × ${b.rows}`, { panel: "room", icon: "room" });
    head(`Markers (${r.markers.length})`);
    r.markers.forEach((m, k) => { if (matches(m.name || `marker ${k}`)) row(m.name || `marker ${k}`, `cell ${m.cell[0]}, ${m.cell[1]}`, { kind: "marker", index: k, icon: "marker" }); });
  }

  // The inspector for a picked marker.
  function renderMarker(inspector, k) {
    const r = room();
    const m = r?.markers[k];
    if (!m) return;
    const { field, group } = host;
    inspector.append(field("Name", m.name, (v) => { m.name = v.slice(0, 15); host.changed(); renderPanel(); }, { type: "text", title: "Up to 15 characters; the program finds the marker by it with world.Marker(name)" }));
    inspector.append(group("Cell", ["Column", "Row"].map((l, i) => field(l, m.cell[i], (v) => { m.cell[i] = Math.max(0, Math.round(v)); host.changed(); }, { title: "Row 0 is the far side of the room before the camera has turned" }))));
    inspector.append(group("Fields", ["A", "B"].map((l, i) => field(l, m.fields[i], (v) => { m.fields[i] = Math.round(v); host.changed(); }, { title: "Two numbers of your own, which the program reads from the marker: which way to face, what to say" }))));
  }

  // The Room panel in the inspector: the brushes, as swatches in the
  // colors the cells are drawn in, the doorways, and the room's size,
  // look, art, links and markers. The tools are in the toolbar and the
  // map from above in the viewport's corner, which the scene editor
  // places.
  const panel = () => $("scene-room");
  const toolsBar = () => $("scene-room-tools");
  const brushBar = () => $("scene-room-brushes");
  const doorBar = () => $("scene-room-doors");
  const gridCanvas = () => $("scene-room-grid");
  const info = () => $("scene-room-info");
  let wired = false;
  function wire() {
    if (wired) return;
    wired = true;
    for (const b of toolsBar().querySelectorAll("button")) b.addEventListener("click", () => setTool(b.dataset.tool));
    for (const [c, name, col] of LEGEND) {
      const b = document.createElement("button");
      b.type = "button";
      b.dataset.rune = c;
      b.setAttribute("aria-pressed", String(c === brush));
      b.addEventListener("click", () => setBrush(c));
      if (isDoor(c)) {
        b.className = "room-door";
        b.textContent = c;
        b.title = `Doorway ${c}: floor that leads to the doorway ${c} of the scene its link names`;
        doorBar().append(b);
        continue;
      }
      b.className = "room-swatch";
      b.title = `${name}, ${c} in the file`;
      const chip = document.createElement("i");
      chip.style.background = `rgb(${col.join(",")})`;
      const label = document.createElement("span");
      label.textContent = name.replace(/ wall$/, "");
      const rune = document.createElement("s");
      rune.textContent = c;
      b.append(chip, label, rune);
      brushBar().append(b);
    }
    const g = gridCanvas();
    let gridDrag = null;
    const cellOf = (e) => {
      const rect = g.getBoundingClientRect();
      const b = build(room());
      return { i: Math.floor((e.clientX - rect.left) * g.width / rect.width / 16), j: Math.floor((e.clientY - rect.top) * g.height / rect.height / 16), b };
    };
    g.addEventListener("pointermove", (e) => {
      if (gridDrag) return;
      const { i, j, b } = cellOf(e);
      const next = i >= 0 && j >= 0 && i < b.cols && j < b.rows ? { i, j } : null;
      if (JSON.stringify(next) !== JSON.stringify(hover)) { hover = next; host.render(); }
    });
    g.addEventListener("pointerleave", () => { if (!gridDrag && hover) { hover = null; host.render(); } });
    g.addEventListener("pointerdown", (e) => {
      const r = room();
      if (!r) return;
      e.preventDefault();
      g.setPointerCapture(e.pointerId);
      g.focus({ preventScroll: true });
      const { i, j, b } = cellOf(e);
      if (i < 0 || j < 0 || i >= b.cols || j >= b.rows) return;
      if (e.altKey) { setBrush(r.cells[j][i]); return; }
      host.beginEdit();
      if (tool === "look") { lookAt(i, j); host.endEdit(); return; }
      if (tool === "marker") {
        const k = r.markers.findIndex((m) => m.cell[0] === i && m.cell[1] === j);
        if (k >= 0) host.select({ kind: "marker", index: k });
        else {
          r.markers.push({ name: uniqueMarker(r), cell: [i, j], fields: [0, 0] });
          host.select({ kind: "marker", index: r.markers.length - 1 });
          host.changed(true);
        }
        renderPanel();
        host.endEdit();
        return;
      }
      gridDrag = { start: [i, j] };
      if (tool === "rect") hover = { rect: [i, j, i, j] };
      else if (applyAt(r, i, j, e, true)) host.changed();
      host.render();
      const move = (ev) => {
        const c = cellOf(ev);
        if (c.i < 0 || c.j < 0 || c.i >= b.cols || c.j >= b.rows) return;
        if (tool === "rect") { hover = { rect: [i, j, c.i, c.j] }; host.render(); return; }
        hover = { i: c.i, j: c.j };
        if (applyAt(r, c.i, c.j, ev, false)) host.changed();
        else host.render();
      };
      const up = () => {
        g.removeEventListener("pointermove", move);
        g.removeEventListener("pointerup", up);
        if (tool === "rect" && hover?.rect) {
          const [x0, y0, x1, y1] = hover.rect;
          let any = false;
          for (let jj = Math.min(y0, y1); jj <= Math.max(y0, y1); jj++) for (let ii = Math.min(x0, x1); ii <= Math.max(x0, x1); ii++) any = setCell(r, ii, jj, brush) || any;
          if (any) host.changed(true);
        }
        if (tool !== "pen") lastLaid = null;
        hover = null;
        gridDrag = null;
        host.endEdit();
        host.render();
      };
      g.addEventListener("pointermove", move);
      g.addEventListener("pointerup", up);
    });
    g.addEventListener("keydown", (e) => {
      if ((e.ctrlKey || e.metaKey) && (e.key === "z" || e.key === "Z")) { e.preventDefault(); e.shiftKey ? host.redo() : host.undo(); return; }
      if ((e.ctrlKey || e.metaKey) && (e.key === "y" || e.key === "Y")) { e.preventDefault(); host.redo(); return; }
      keys(e);
    });
  }

  // renderDoors marks the doorway brushes the map uses.
  function renderDoors() {
    const r = room();
    const used = new Set((r?.cells.join("") ?? "").replace(/[^0-9]/g, ""));
    for (const b of doorBar().querySelectorAll("button")) b.classList.toggle("used", used.has(b.dataset.rune));
  }

  function renderPanel() {
    wire();
    const form = panel();
    form.replaceChildren();
    const r = room();
    const has = !!r;
    $("scene-room-paint").hidden = !has;
    if (!has) {
      info().textContent = "A room is a map of cells with heights, drawn by package stage as Dragon Quest VII drew its towns: floors, walls, rock, water, cabins, doorways linked to other scenes. Start one to make this scene a room; its camera and lights then stay the program's own.";
      const begin = document.createElement("button");
      begin.type = "button";
      begin.className = "small-button";
      begin.textContent = "Start a room";
      begin.addEventListener("click", start);
      form.append(begin);
      return;
    }
    renderDoors();
    const { field, group } = host;
    const b = build(r);
    const resize = (cols, rows) => {
      cols = Math.max(1, Math.min(MAX, Math.round(cols)));
      rows = Math.max(1, Math.min(MAX, Math.round(rows)));
      const grow = (rowsIn, fill) => {
        const outRows = [];
        for (let j = 0; j < rows; j++) outRows.push(((rowsIn[j] ?? "") + fill.repeat(cols)).slice(0, cols));
        return outRows;
      };
      r.cells = grow(r.cells, ".");
      if (r.heights) r.heights = grow(r.heights, "0");
      r.markers = r.markers.filter((m) => m.cell[0] < cols && m.cell[1] < rows);
      host.changed(true);
      renderPanel();
    };
    form.append(group("Size", [
      field("Across", b.cols, (v) => resize(v, b.rows), { title: "Cells across, up to 32; a cell is a tile of 64 units" }),
      field("Down", b.rows, (v) => resize(b.cols, v), { title: "Cells down, up to 32; row 0 is the far side before the camera has turned" }),
    ]));
    const colorField = (label, get, set, title) => {
      const wrap = document.createElement("label");
      wrap.title = title;
      const span = document.createElement("span");
      span.textContent = label;
      const input = document.createElement("input");
      input.type = "color";
      input.value = "#" + get().map((c) => c.toString(16).padStart(2, "0")).join("");
      input.addEventListener("input", () => { set([1, 3, 5].map((k) => parseInt(input.value.slice(k, k + 2), 16))); host.changed(); });
      wrap.append(span, input);
      return wrap;
    };
    const yesNo = (label, get, set, title) => field(label, get() ? "1" : "0", (v) => { set(v === "1"); host.changed(true); }, { options: [["0", "no"], ["1", "yes"]], title });
    form.append(group("Look", [
      field("Floor", r.floor, (v) => { r.floor = Math.max(0, Math.round(v)); host.changed(); renderPanel(); }, { title: "Which of the program's floor textures, by index" }),
      field("Wall", r.wall, (v) => { r.wall = Math.max(0, Math.round(v)); host.changed(); renderPanel(); }, { title: "Which of the program's wall textures, by index: the walls, the cliff faces and ground too steep to walk" }),
      colorField("Tint", () => r.tint, (c) => { r.tint = c; }, "The ground's color; 128, 128, 128 shows its texture as it is"),
      yesNo("Rough", () => r.rough, (v) => { r.rough = v; }, "Moves every corner a little, for a cave's broken outlines"),
      yesNo("Black", () => r.black, (v) => { r.black = v; }, "Draws only the rock that borders the ground, black beyond: a cave is its passages alone"),
      field("Rock height", r.rockTall, (v) => { r.rockTall = Math.max(0, Math.round(v)); host.changed(); }, { title: "How high rock stands, in units; 0 is a tile, 64", step: 16 }),
      field("Rock lean", r.rockLean, (v) => { r.rockLean = Math.max(0, Math.round(v)); host.changed(); }, { title: "How far a rock face's top leans back into the rock, in units, so that it slopes like a cliff", step: 4 }),
    ]));
    form.append(group("Sky and camera", [
      colorField("Sky top", () => r.sky[0], (c) => { r.sky[0] = c; }, "The sky's color at the top of the screen; all black with the bottom means no sky"),
      colorField("Sky bottom", () => r.sky[1], (c) => { r.sky[1] = c; }, "The sky's color at the bottom of the screen"),
      colorField("Background", () => r.background, (c) => { r.background = c; }, "What the screen clears to when there is no sky; black leaves the program's own"),
      field("Camera up", r.camUp, (v) => { r.camUp = Math.max(0, Math.round(v)); host.changed(); }, { title: "How high above its figure the program's camera sits in this room; 0 is its usual slant", step: 20 }),
      field("Camera back", r.camBack, (v) => { r.camBack = Math.max(0, Math.round(v)); host.changed(); }, { title: "How far behind its figure the program's camera sits; 0 is its usual slant", step: 20 }),
    ]));
    // The art the editor draws with: which of the project's TIMs stand
    // for the floor and wall indexes and the rest, kept in the file for
    // the editor alone.
    const tims = host.dataFiles(/\.tim$/i).map(host.discPath);
    const options = [["", "none, a plain color"], ...tims.map((p) => [p, p.replace(/^ASSETS\//, "")])];
    const artField = (label, get, set, title) => field(label, get() ?? "", (v) => { set(v); host.changed(true); }, { options, title });
    form.append(group("Textures in the editor", [
      artField(`Floor ${r.floor}`, () => r.art.floors?.[r.floor], (v) => { r.art.floors = r.art.floors ?? []; r.art.floors[r.floor] = v; for (let k = 0; k < r.art.floors.length; k++) r.art.floors[k] = r.art.floors[k] ?? ""; }, "The TIM the program loads as this floor texture, for the picture here; the program keeps its own list"),
      artField(`Wall ${r.wall}`, () => r.art.walls?.[r.wall], (v) => { r.art.walls = r.art.walls ?? []; r.art.walls[r.wall] = v; for (let k = 0; k < r.art.walls.length; k++) r.art.walls[k] = r.art.walls[k] ?? ""; }, "The TIM the program loads as this wall texture"),
      artField("Rock", () => r.art.rock, (v) => { r.art.rock = v; }, "The rock's texture; the wall's when none"),
      artField("Logs", () => r.art.logs, (v) => { r.art.logs = v; }, "A cabin's walls"),
      artField("Roof", () => r.art.roof, (v) => { r.art.roof = v; }, "A cabin's roof"),
      artField("Water", () => r.art.water, (v) => { r.art.water = v; }, "A pool's surface"),
    ]));
    // Links: a select for each doorway digit in the map.
    const doors = [...new Set(r.cells.join("").replace(/[^0-9]/g, "").split("").filter(Boolean))].sort();
    const dats = host.sceneDats();
    if (doors.length) {
      form.append(group("Doorways lead to", doors.map((d) => {
        const link = r.links.find((l) => String(l.door) === d);
        return field(`Door ${d}`, link?.scene ?? "", (v) => {
          r.links = r.links.filter((l) => String(l.door) !== d);
          if (v) r.links.push({ door: Number(d), scene: v });
          r.links.sort((p, q) => p.door - q.door);
          host.changed(true);
        }, { options: [["", "nowhere"], ...dats.map((p) => [p, p.replace(/^ASSETS\//, "")])], title: `The scene the cells marked ${d} lead to, in at its doorway ${d}` });
      })));
    }
    const add = document.createElement("button");
    add.type = "button";
    add.className = "small-button";
    add.textContent = "Add a marker";
    add.title = "A marker on the cell the camera looks at, naming it for one of the program's own people or things to stand on; the marker tool (M) puts one where you click";
    add.addEventListener("click", addMarker);
    const drop = document.createElement("button");
    drop.type = "button";
    drop.className = "small-button";
    drop.textContent = "No room";
    drop.title = "Take the room out of the scene, leaving a scene of objects";
    drop.addEventListener("click", async () => {
      if (!(await dialogs.confirm({ title: "Take the room out of this scene?", text: "Its cells, doorways and markers go with it, leaving a scene of objects. Undo puts it back.", ok: "Take it out", danger: true }))) return;
      doc().room = null;
      host.select(null);
      host.changed(true);
      renderPanel();
    });
    form.append(group("Markers", [add]), drop);
    renderInfo();
    renderGrid();
  }

  // The flat grid: the cells from above, colored by rune with the level
  // as a digit, the markers as dots, the focus and the cell under the
  // pointer outlined.
  function renderGrid() {
    const r = room();
    const g = gridCanvas();
    if (!r || !g || g.hidden) return;
    const b = build(r);
    g.width = b.cols * 16;
    g.height = b.rows * 16;
    const gctx = g.getContext("2d");
    for (let j = 0; j < b.rows; j++) {
      for (let i = 0; i < b.cols; i++) {
        const c = r.cells[j][i];
        const col = legendColor[c] ?? legendColor["#"];
        const lv = b.level(i, j);
        const k = 0.7 + lv * 0.05;
        gctx.fillStyle = `rgb(${Math.min(255, col[0] * k)},${Math.min(255, col[1] * k)},${Math.min(255, col[2] * k)})`;
        gctx.fillRect(i * 16, j * 16, 16, 16);
        if (isDoor(c) || (r.heights && lv)) {
          gctx.fillStyle = isDoor(c) ? "#fff" : "rgba(0,0,0,0.6)";
          gctx.font = "bold 9px sans-serif";
          gctx.textAlign = "center";
          gctx.fillText(isDoor(c) ? c : String(lv), i * 16 + 8, j * 16 + 11);
        }
      }
    }
    gctx.strokeStyle = "rgba(255,255,255,0.12)";
    for (let x = 16; x < g.width; x += 16) { gctx.beginPath(); gctx.moveTo(x + 0.5, 0); gctx.lineTo(x + 0.5, g.height); gctx.stroke(); }
    for (let y = 16; y < g.height; y += 16) { gctx.beginPath(); gctx.moveTo(0, y + 0.5); gctx.lineTo(g.width, y + 0.5); gctx.stroke(); }
    const sel = host.selected();
    r.markers.forEach((m, k) => {
      const on = sel?.kind === "marker" && sel.index === k;
      gctx.fillStyle = on ? host.themeColor("--accent") : host.themeColor("--triangle");
      gctx.beginPath();
      gctx.arc(m.cell[0] * 16 + 8, m.cell[1] * 16 + 8, on ? 5 : 4, 0, 2 * Math.PI);
      gctx.fill();
    });
    // The focus and which way the camera looks, as an arrow.
    const fi = (view.focus[0] + b.cols * TILE / 2) / TILE * 16, fj = (b.rows * TILE / 2 - view.focus[1]) / TILE * 16;
    const angle = view.dir * Math.PI / 4;
    gctx.strokeStyle = host.themeColor("--accent");
    gctx.lineWidth = 2;
    gctx.beginPath();
    gctx.moveTo(fi - 10 * Math.sin(angle), fj + 10 * Math.cos(angle));
    gctx.lineTo(fi + 10 * Math.sin(angle), fj - 10 * Math.cos(angle));
    gctx.stroke();
    gctx.beginPath();
    gctx.arc(fi, fj, 3, 0, 2 * Math.PI);
    gctx.stroke();
    gctx.lineWidth = 1;
    if (hover?.rect) {
      const [x0, y0, x1, y1] = hover.rect;
      gctx.strokeStyle = host.themeColor("--accent");
      gctx.strokeRect(Math.min(x0, x1) * 16 + 0.5, Math.min(y0, y1) * 16 + 0.5, (Math.abs(x1 - x0) + 1) * 16 - 1, (Math.abs(y1 - y0) + 1) * 16 - 1);
    } else if (hover) {
      gctx.strokeStyle = host.themeColor("--ink");
      gctx.strokeRect(hover.i * 16 + 0.5, hover.j * 16 + 0.5, 15, 15);
    }
    host.sizeInset?.();
  }

  // reset forgets the view for another scene.
  function reset() {
    view.dir = 1;
    view.focus = [0, 0];
    view.zoom = 1;
    hover = null;
    lastLaid = null;
    built = null;
    builtKey = "";
  }

  return { attach, normalize, out, template, camera, draw, overlay, status, pointerDown, pointerHover, pointerLeave, keys, turn, pan, zoom, outlineRows, renderMarker, renderPanel, renderGrid, reset, resetView, start, addMarker, playHere, setTool, has: () => !!room(), tool: () => tool };
})();

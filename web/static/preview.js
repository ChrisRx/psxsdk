// The data files shown as the console shows them: a TIM decoded through
// its palette, a TMD drawn at 320 by 240 with the library's projection,
// lit as the square example lights its cube and textured from the TIMs
// beside it, and the map of video memory with every TIM where its header
// puts it, dragged into place.
//
// The formats are those of format/tim and format/tmd, read here in the
// browser. The drawing is a small software rasterizer: flat and gouraud
// shading, affine texture mapping with no perspective correction, polygons
// sorted far to near, as the GPU and the ordering table do.
"use strict";

const preview = (() => {
  const le = true;

  // TIM.

  function readBlock(view, at, bytes) {
    if (at + 12 > view.byteLength) throw new Error("too short for a block header");
    let size = view.getUint32(at, le);
    const block = {
      at,
      x: view.getUint16(at + 4, le),
      y: view.getUint16(at + 6, le),
      w: view.getUint16(at + 8, le),
      h: view.getUint16(at + 10, le),
    };
    const units = block.w * block.h;
    if (at + 12 + 2 * units > view.byteLength) throw new Error(`${block.w}x${block.h} units at ${block.x},${block.y} run past the end of the file`);
    if (size < 12 + 2 * units) size = 12 + 2 * units;
    block.data = new Uint16Array(units);
    for (let i = 0; i < units; i++) block.data[i] = view.getUint16(at + 12 + 2 * i, le);
    block.size = Math.min(size, view.byteLength - at);
    return block;
  }

  // decodeTIM reads a TIM: {mode, bits, clut, pixels, width, height}, with
  // each block's x, y, w (in 16-bit units), h and data, and where in the
  // file its header is, for writing a new position back.
  function decodeTIM(bytes) {
    const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
    if (view.byteLength < 8 || view.getUint32(0, le) !== 0x10) throw new Error("not a TIM file");
    const flags = view.getUint32(4, le);
    const mode = flags & 7;
    if (mode > 3) throw new Error(`unknown pixel mode ${mode}`);
    const tim = { mode, bits: [4, 8, 16, 24][mode], clut: null, pixels: null };
    let at = 8;
    if (flags & 8) {
      tim.clut = readBlock(view, at);
      at += tim.clut.size;
    }
    tim.pixels = readBlock(view, at);
    tim.width = mode === 0 ? tim.pixels.w * 4 : mode === 1 ? tim.pixels.w * 2 : mode === 3 ? Math.floor(tim.pixels.w * 2 / 3) : tim.pixels.w;
    tim.height = tim.pixels.h;
    return tim;
  }

  // tpage and cba are the texture page and the palette a primitive names
  // the image by, as the library's GetTPage and GetClut make them.
  function tpage(tim) { return (tim.mode & 3) << 7 | (tim.pixels.y >> 8) << 4 | (tim.pixels.x >> 6); }
  function cba(tim) { return tim.clut ? tim.clut.y << 6 | tim.clut.x >> 4 : 0; }
  // uv is where the pixels start inside their page, in texels.
  function uv(tim) { return [(tim.pixels.x % 64) * 16 / tim.bits, tim.pixels.y % 256]; }

  // rgba puts a 16-bit color into out at i: 0x0000 is not drawn.
  function rgba(c, out, i) {
    if (c === 0) {
      out[i + 3] = 0;
      return;
    }
    out[i] = (c & 31) * 255 / 31;
    out[i + 1] = (c >> 5 & 31) * 255 / 31;
    out[i + 2] = (c >> 10 & 31) * 255 / 31;
    out[i + 3] = 255;
  }

  // timImage decodes the picture through row of its palette.
  function timImage(tim, row = 0) {
    const { width, height, pixels, clut } = tim;
    const img = new ImageData(Math.max(1, width), Math.max(1, height));
    const out = img.data;
    const pal = clut ? clut.data.subarray(Math.min(row, clut.h - 1) * clut.w, Math.min(row, clut.h - 1) * clut.w + clut.w) : null;
    for (let y = 0; y < height; y++) {
      for (let x = 0; x < width; x++) {
        const i = (y * width + x) * 4;
        let c = 0;
        if (tim.mode === 0) {
          const unit = pixels.data[y * pixels.w + (x >> 2)];
          c = pal ? pal[(unit >> ((x & 3) * 4)) & 15] ?? 0 : 0;
        } else if (tim.mode === 1) {
          const unit = pixels.data[y * pixels.w + (x >> 1)];
          c = pal ? pal[(unit >> ((x & 1) * 8)) & 255] ?? 0 : 0;
        } else if (tim.mode === 2) {
          c = pixels.data[y * pixels.w + x];
        } else {
          // 24-bit: three bytes a pixel, two pixels in three units.
          const b = (y * pixels.w + Math.floor(x * 3 / 2)) * 2 + (x & 1);
          const d = pixels.data;
          const byte = (k) => (d[k >> 1] >> ((k & 1) * 8)) & 255;
          out[i] = byte(b);
          out[i + 1] = byte(b + 1);
          out[i + 2] = byte(b + 2);
          out[i + 3] = 255;
          continue;
        }
        rgba(c, out, i);
      }
    }
    return img;
  }

  // moveTIM returns the file with a block's position changed.
  function moveTIM(bytes, block, x, y) {
    const out = bytes.slice();
    const view = new DataView(out.buffer, out.byteOffset, out.byteLength);
    view.setUint16(block.at + 4, x, le);
    view.setUint16(block.at + 6, y, le);
    return out;
  }

  // TMD.

  // decodeTMD reads a TMD: {objects: [{vertices, normals, primitives}]},
  // the vectors as Int16Arrays of x, y, z triples and each primitive as
  // {mode, flag, olen, packet}.
  function decodeTMD(bytes) {
    const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
    if (view.byteLength < 12 || view.getUint32(0, le) !== 0x41) throw new Error("not a TMD file");
    if (view.getUint32(4, le) & 1) throw new Error("the object table holds addresses, not offsets");
    const n = view.getUint32(8, le);
    if (12 + 28 * n > view.byteLength) throw new Error(`${n} objects run past the end of the file`);
    const vectors = (off, count) => {
      const start = 12 + off;
      if (start + count * 8 > view.byteLength) throw new Error(`${count} vectors at ${off} run past the end of the file`);
      const out = new Int16Array(count * 3);
      for (let i = 0; i < count; i++) {
        out[3 * i] = view.getInt16(start + 8 * i, le);
        out[3 * i + 1] = view.getInt16(start + 8 * i + 2, le);
        out[3 * i + 2] = view.getInt16(start + 8 * i + 4, le);
      }
      return out;
    };
    const objects = [];
    for (let i = 0; i < n; i++) {
      const e = 12 + 28 * i;
      const o = {
        vertices: vectors(view.getUint32(e, le), view.getUint32(e + 4, le)),
        normals: vectors(view.getUint32(e + 8, le), view.getUint32(e + 12, le)),
        primitives: [],
        scale: view.getInt32(e + 24, le),
      };
      let pos = 12 + view.getUint32(e + 16, le);
      const count = view.getUint32(e + 20, le);
      for (let k = 0; k < count; k++) {
        if (pos + 4 > view.byteLength) throw new Error("primitives run past the end of the file");
        const h = view.getUint32(pos, le);
        const ilen = h >> 8 & 0xff;
        if (pos + 4 + 4 * ilen > view.byteLength) throw new Error("a primitive runs past the end of the file");
        const packet = new Uint32Array(ilen);
        for (let w = 0; w < ilen; w++) packet[w] = view.getUint32(pos + 4 + 4 * w, le);
        o.primitives.push({ mode: h >>> 24, flag: h >> 16 & 0xff, olen: h & 0xff, packet });
        pos += 4 + 4 * ilen;
      }
      objects.push(o);
    }
    return { objects };
  }

  // polygons reads an object's packets into what the drawing needs: for
  // each polygon or line its vertex indices, its normal indices or none
  // when it is not lit, a color per vertex, and its texture.
  function polygons(o) {
    const out = [];
    const lo = (w) => w & 0xffff;
    const hi = (w) => w >>> 16;
    const rgb = (w) => [w & 255, w >> 8 & 255, w >> 16 & 255];
    for (const p of o.primitives) {
      const kind = p.mode & 0xe0;
      const w = p.packet;
      if (kind === 0x40) {
        const gouraud = (p.mode & 0x10) !== 0;
        const c0 = rgb(w[0]);
        const c1 = gouraud ? rgb(w[1]) : c0;
        const v = w[gouraud ? 2 : 1];
        out.push({ line: true, verts: [lo(v), hi(v)], colors: [c0, c1], normals: null, texture: null, double: true });
        continue;
      }
      if (kind !== 0x20) continue;
      const quad = (p.mode & 8) !== 0;
      const n = quad ? 4 : 3;
      const textured = (p.mode & 4) !== 0;
      const iip = (p.mode & 0x10) !== 0;
      const lit = !(p.flag & 1) && !(p.mode & 1);
      const gradation = lit ? (p.flag & 4) !== 0 : iip;
      let at = 0;
      let texture = null;
      if (textured) {
        texture = { cba: hi(w[0]), tpage: hi(w[1]), uv: [] };
        for (let i = 0; i < n; i++) texture.uv.push([w[at + i] & 255, w[at + i] >> 8 & 255]);
        at += n;
      }
      let colors;
      if (!lit || !textured) {
        const count = gradation ? n : 1;
        colors = [];
        for (let i = 0; i < count; i++) colors.push(rgb(w[at + i]));
        at += count;
        while (colors.length < n) colors.push(colors[0]);
      } else {
        colors = Array(n).fill([128, 128, 128]);
      }
      const verts = [];
      let normals = null;
      if (lit) {
        normals = [];
        if (iip) {
          for (let i = 0; i < n; i++) {
            normals.push(lo(w[at + i]));
            verts.push(hi(w[at + i]));
          }
        } else {
          normals.push(lo(w[at]));
          verts.push(hi(w[at]), lo(w[at + 1]), hi(w[at + 1]));
          if (quad) verts.push(lo(w[at + 2]));
          while (normals.length < n) normals.push(normals[0]);
        }
      } else {
        verts.push(lo(w[at]), hi(w[at]), lo(w[at + 1]));
        if (quad) verts.push(hi(w[at + 1]));
      }
      out.push({ verts, normals, colors, texture, double: (p.flag & 2) !== 0, lit });
    }
    return out;
  }

  // Video memory: 1024 by 512 units, with every TIM placed, for the map
  // and for textured polygons.
  class VRAM {
    constructor() {
      this.data = new Uint16Array(1024 * 512);
      this.placed = []; // {path, tim, block, kind}
    }

    place(path, tim) {
      for (const [kind, block] of [["pixels", tim.pixels], ["clut", tim.clut]]) {
        if (!block) continue;
        for (let y = 0; y < block.h; y++) {
          const vy = block.y + y;
          if (vy >= 512) break;
          for (let x = 0; x < block.w; x++) {
            const vx = block.x + x;
            if (vx >= 1024) break;
            this.data[vy * 1024 + vx] = block.data[y * block.w + x];
          }
        }
        this.placed.push({ path, tim, block, kind });
      }
    }

    // texel returns the 16-bit color at u, v of a texture page, through
    // its palette for the indexed modes.
    texel(tpage, cba, u, v) {
      const mode = tpage >> 7 & 3;
      const px = (tpage & 0xf) * 64;
      const py = (tpage >> 4 & 1) * 256;
      const y = (py + (v & 255)) & 511;
      if (mode === 2) return this.data[y * 1024 + ((px + u) & 1023)];
      const cx = (cba & 0x3f) * 16;
      const cy = cba >> 6 & 511;
      if (mode === 0) {
        const unit = this.data[y * 1024 + ((px + (u >> 2)) & 1023)];
        return this.data[cy * 1024 + ((cx + ((unit >> ((u & 3) * 4)) & 15)) & 1023)];
      }
      const unit = this.data[y * 1024 + ((px + (u >> 1)) & 1023)];
      return this.data[cy * 1024 + ((cx + ((unit >> ((u & 1) * 8)) & 255)) & 1023)];
    }
  }

  // Drawing a model.

  const W = 320;
  const H = 240;

  // renderModel draws object o of a TMD into img, a 320 by 240 ImageData,
  // turned by ry about the vertical and rx about the horizontal, lit from
  // over the camera's shoulder as the square example lights its cube, with
  // textures taken from vram. It returns how many polygons it drew.
  function renderModel(img, o, vram, ry, rx) {
    const out = img.data;
    out.fill(0);
    const depth = new Float32Array(W * H).fill(Infinity);
    const nv = o.vertices.length / 3;
    const cy = Math.cos(ry), sy = Math.sin(ry), cx = Math.cos(rx), sx = Math.sin(rx);
    const rot = (x, y, z) => {
      // About Y, then about X.
      const x1 = x * cy + z * sy;
      const z1 = -x * sy + z * cy;
      const y2 = y * cx - z1 * sx;
      const z2 = y * sx + z1 * cx;
      return [x1, y2, z2];
    };
    // The model's size decides how far back the camera stands.
    let r = 1;
    for (let i = 0; i < nv; i++) r = Math.max(r, Math.hypot(o.vertices[3 * i], o.vertices[3 * i + 1], o.vertices[3 * i + 2]));
    const h = 320;
    const d = r * 2.6;
    const sv = new Float32Array(nv * 3); // screen x, y and depth
    for (let i = 0; i < nv; i++) {
      const [x, y, z] = rot(o.vertices[3 * i], o.vertices[3 * i + 1], o.vertices[3 * i + 2]);
      const zz = z + d;
      sv[3 * i] = W / 2 + x * h / zz;
      sv[3 * i + 1] = H / 2 + y * h / zz;
      sv[3 * i + 2] = zz;
    }
    const nn = o.normals.length / 3;
    const light = [1, 1, 1].map((c) => c / Math.sqrt(3));
    const shade = [];
    for (let i = 0; i < nn; i++) {
      const [x, y, z] = rot(o.normals[3 * i], o.normals[3 * i + 1], o.normals[3 * i + 2]);
      const dot = -(x * light[0] + y * light[1] + z * light[2]) / 4096;
      const k = 0.25 + 0.75 * Math.max(0, dot);
      shade.push([k, k, k]);
    }
    const polys = polygons(o).filter((p) => p.verts.every((v) => v < nv) && (!p.normals || p.normals.every((n) => n < nn)));
    const tris = [];
    for (const p of polys) {
      if (p.line) {
        tris.push({ p, idx: [0, 1], z: (sv[3 * p.verts[0] + 2] + sv[3 * p.verts[1] + 2]) / 2 });
        continue;
      }
      const z = p.verts.reduce((s, v) => s + sv[3 * v + 2], 0) / p.verts.length;
      tris.push({ p, idx: [0, 1, 2], z });
      if (p.verts.length === 4) tris.push({ p, idx: [1, 3, 2], z });
    }
    tris.sort((a, b) => b.z - a.z);
    const drawn = new Set(); // the polygons with a triangle facing the camera
    for (const t of tris) {
      const { p, idx } = t;
      if (p.line) {
        drawLine(out, null, 0, sv[3 * p.verts[0]], sv[3 * p.verts[0] + 1], sv[3 * p.verts[1]], sv[3 * p.verts[1] + 1], p.colors[0], p.colors[1]);
        drawn.add(p);
        continue;
      }
      const v = idx.map((i) => p.verts[i]);
      const x0 = sv[3 * v[0]], y0 = sv[3 * v[0] + 1];
      const x1 = sv[3 * v[1]], y1 = sv[3 * v[1] + 1];
      const x2 = sv[3 * v[2]], y2 = sv[3 * v[2] + 1];
      const area = (x1 - x0) * (y2 - y0) - (y1 - y0) * (x2 - x0);
      if (area === 0) continue;
      if (area < 0 && !p.double) continue;
      if (sv[3 * v[0] + 2] <= 1 || sv[3 * v[1] + 2] <= 1 || sv[3 * v[2] + 2] <= 1) continue;
      const cols = idx.map((i) => p.colors[i]);
      const shades = p.normals ? idx.map((i) => shade[p.normals[i]]) : [[1, 1, 1], [1, 1, 1], [1, 1, 1]];
      const uvs = p.texture ? idx.map((i) => p.texture.uv[i]) : null;
      fillTriangle(out, null, 0, x0, y0, x1, y1, x2, y2, area, cols, shades, uvs, p, vram);
      drawn.add(p);
    }
    return drawn.size;
  }

  // fillTriangle rasterizes one triangle into out, an ImageData's bytes,
  // with its corners' colors, shades (a factor per channel, from the
  // lights) and texture coordinates interpolated across it, the texture
  // read from vram; ids, when given, gets id at each pixel drawn, for
  // picking.
  function fillTriangle(out, ids, id, x0, y0, x1, y1, x2, y2, area, cols, shades, uvs, p, vram) {
    const minX = Math.max(0, Math.floor(Math.min(x0, x1, x2)));
    const maxX = Math.min(W - 1, Math.ceil(Math.max(x0, x1, x2)));
    const minY = Math.max(0, Math.floor(Math.min(y0, y1, y2)));
    const maxY = Math.min(H - 1, Math.ceil(Math.max(y0, y1, y2)));
    const inv = 1 / area;
    const lit = p.lit;
    for (let y = minY; y <= maxY; y++) {
      const py = y + 0.5;
      for (let x = minX; x <= maxX; x++) {
        const px = x + 0.5;
        let w0 = ((x1 - px) * (y2 - py) - (y1 - py) * (x2 - px)) * inv;
        let w1 = ((x2 - px) * (y0 - py) - (y2 - py) * (x0 - px)) * inv;
        let w2 = 1 - w0 - w1;
        if (w0 < 0 || w1 < 0 || w2 < 0) continue;
        let r, g, b;
        const sr = w0 * shades[0][0] + w1 * shades[1][0] + w2 * shades[2][0];
        const sg = w0 * shades[0][1] + w1 * shades[1][1] + w2 * shades[2][1];
        const sb = w0 * shades[0][2] + w1 * shades[1][2] + w2 * shades[2][2];
        if (uvs) {
          const u = Math.floor(w0 * uvs[0][0] + w1 * uvs[1][0] + w2 * uvs[2][0]);
          const v = Math.floor(w0 * uvs[0][1] + w1 * uvs[1][1] + w2 * uvs[2][1]);
          const c = vram.texel(p.texture.tpage, p.texture.cba, u & 255, v & 255);
          if (c === 0) continue;
          r = (c & 31) * 255 / 31;
          g = (c >> 5 & 31) * 255 / 31;
          b = (c >> 10 & 31) * 255 / 31;
          if (lit) {
            r *= sr; g *= sg; b *= sb;
          } else {
            // 128 is the texture as it is.
            const cr = w0 * cols[0][0] + w1 * cols[1][0] + w2 * cols[2][0];
            const cg = w0 * cols[0][1] + w1 * cols[1][1] + w2 * cols[2][1];
            const cb = w0 * cols[0][2] + w1 * cols[1][2] + w2 * cols[2][2];
            r = r * cr / 128; g = g * cg / 128; b = b * cb / 128;
          }
        } else {
          r = (w0 * cols[0][0] + w1 * cols[1][0] + w2 * cols[2][0]) * sr;
          g = (w0 * cols[0][1] + w1 * cols[1][1] + w2 * cols[2][1]) * sg;
          b = (w0 * cols[0][2] + w1 * cols[1][2] + w2 * cols[2][2]) * sb;
        }
        const i = (y * W + x) * 4;
        out[i] = r > 255 ? 255 : r;
        out[i + 1] = g > 255 ? 255 : g;
        out[i + 2] = b > 255 ? 255 : b;
        out[i + 3] = 255;
        if (ids) ids[y * W + x] = id;
      }
    }
  }

  // drawLine draws a line shaded from c0 at one end to c1 at the other.
  function drawLine(out, ids, id, x0, y0, x1, y1, c0, c1) {
    const steps = Math.max(1, Math.ceil(Math.max(Math.abs(x1 - x0), Math.abs(y1 - y0))));
    for (let k = 0; k <= steps; k++) {
      const t = k / steps;
      const x = Math.round(x0 + (x1 - x0) * t);
      const y = Math.round(y0 + (y1 - y0) * t);
      if (x < 0 || y < 0 || x >= W || y >= H) continue;
      const i = (y * W + x) * 4;
      for (let c = 0; c < 3; c++) out[i + c] = c0[c] + (c1[c] - c0[c]) * t;
      out[i + 3] = 255;
      if (ids) ids[y * W + x] = id;
    }
  }

  // drawSprite draws a sprite as GsSortFastSprite does, its top left at
  // X, Y from the origin ox, oy, each texel through its page and palette
  // in vram, lit by R, G and B with 128 as the texture is.
  function drawSprite(out, ids, id, vram, sp, ox, oy) {
    const left = Math.round(ox + sp.x), top = Math.round(oy + sp.y);
    for (let y = 0; y < sp.h; y++) {
      const py = top + y;
      if (py < 0 || py >= H) continue;
      for (let x = 0; x < sp.w; x++) {
        const px = left + x;
        if (px < 0 || px >= W) continue;
        const c = vram.texel(sp.tpage, sp.cx >> 4 & 0x3f | (sp.cy & 0x1ff) << 6, (sp.u + x) & 255, (sp.v + y) & 255);
        if (c === 0) continue;
        const i = (py * W + px) * 4;
        const r = (c & 31) * 255 / 31 * sp.r / 128;
        const g = (c >> 5 & 31) * 255 / 31 * sp.g / 128;
        const b = (c >> 10 & 31) * 255 / 31 * sp.b / 128;
        out[i] = r > 255 ? 255 : r;
        out[i + 1] = g > 255 ? 255 : g;
        out[i + 2] = b > 255 ? 255 : b;
        out[i + 3] = 255;
        if (ids) ids[py * W + px] = id;
      }
    }
  }

  // stats summarizes a TMD for the caption.
  function stats(tmd) {
    let verts = 0, polys = 0, textured = 0, lines = 0;
    for (const o of tmd.objects) {
      verts += o.vertices.length / 3;
      for (const p of polygons(o)) {
        if (p.line) lines++;
        else {
          polys++;
          if (p.texture) textured++;
        }
      }
    }
    return { objects: tmd.objects.length, verts, polys, textured, lines };
  }

  return { decodeTIM, timImage, moveTIM, tpage, cba, uv, decodeTMD, polygons, VRAM, renderModel, fillTriangle, drawLine, drawSprite, stats, W, H };
})();

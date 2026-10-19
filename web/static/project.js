// The playground's projects. A project is the files of one program, laid
// out as the repo's examples are, so that a download runs as it is with
// go run:
//
//	main.go           the program, and any other .go files beside it
//	data/             the files the program registers with package disc,
//	                  which the build puts on the disc
//	art/              the pictures, models and sounds the data files are
//	                  made from, by the converters the program's
//	                  go:generate lines name (convert.js)
//
// Other files at the root, such as the scene editor's own, are kept but
// go on no disc. Projects are kept in this browser, in IndexedDB, one
// record for each one's name and dates and another for its files; what is
// typed is saved a moment after it changes. A project goes out and comes
// in as a zip of that layout.
//
// files.js and assets.js show a project; playground.js builds it.
"use strict";

// projectEvents tells the page what changed:
//   open    {project}: a project was opened, replacing the one shown
//   files   {project}: files were added, removed or renamed
//   change  {project, path, source}: a file's content changed; source
//           says by whom, so the editor that made the change ignores it
//   name    {project}: the project was renamed
//   saved   {project, kept}: the project was written, or could not be
const projectEvents = new EventTarget();

function emit(type, detail) {
  projectEvents.dispatchEvent(new CustomEvent(type, { detail }));
}

const utf8 = { enc: new TextEncoder(), dec: new TextDecoder() };

class Project {
  // record: {id, name, created, modified} with files, a list of {path, data}.
  constructor(record, files = []) {
    this.id = record.id;
    this.name = record.name;
    this.created = record.created;
    this.modified = record.modified;
    this.files = new Map(files.map((f) => [f.path, typeof f.data === "string" ? utf8.enc.encode(f.data) : f.data]));
  }

  // meta is the record kept for the list of projects.
  meta() {
    let size = 0;
    for (const data of this.files.values()) size += data.length;
    return { id: this.id, name: this.name, created: this.created, modified: this.modified, size, count: this.files.size };
  }

  list() {
    return [...this.files].map(([path, data]) => ({ path, data }));
  }

  has(path) { return this.files.has(path); }
  get(path) { return this.files.get(path); }
  text(path) {
    const data = this.files.get(path);
    return data === undefined ? undefined : utf8.dec.decode(data);
  }

  // goFiles is the program's Go files, at the root, main.go first.
  goFiles() {
    return [...this.files.keys()].filter((p) => /^[^/]+\.go$/.test(p)).sort((a, b) => (a === "main.go" ? -1 : b === "main.go" ? 1 : a.localeCompare(b)));
  }

  // dataFiles is what goes on the disc.
  dataFiles() {
    return [...this.files.keys()].filter((p) => p.startsWith("data/")).sort();
  }

  // sceneFiles is the scenes, under scenes/, each cooked to a data file.
  sceneFiles() {
    return [...this.files.keys()].filter((p) => /^scenes\/[^/]+\.json$/.test(p)).sort();
  }

  // onDisc is every file that reaches the disc, under data/ as it is or
  // under pack/NAME/ inside an archive: what the program can draw from.
  onDisc() {
    return [...this.dataFiles(), ...this.packFiles()];
  }

  // packFiles is the files packed into archives, under pack/NAME/, each
  // directory one archive data/NAME.ARC that mkarc's line makes.
  packFiles() {
    return [...this.files.keys()].filter((p) => /^pack\/[^/]+\/[^/]+$/.test(p)).sort();
  }
  packs() {
    return [...new Set(this.packFiles().map((p) => p.split("/")[1]))].sort();
  }

  // artFiles is the sources the data files are made from, under art/.
  artFiles() {
    return [...this.files.keys()].filter((p) => p.startsWith("art/")).sort();
  }

  // set puts data, bytes or text, at path, and says who did it.
  set(path, data, source = "") {
    const bytes = typeof data === "string" ? utf8.enc.encode(data) : data;
    const added = !this.files.has(path);
    this.files.set(path, bytes);
    this.touch();
    if (added) emit("files", { project: this });
    emit("change", { project: this, path, source });
  }

  // setAll puts several files at once, as one change.
  setAll(files, source = "") {
    let added = false;
    for (const { path, data } of files) {
      added ||= !this.files.has(path);
      this.files.set(path, typeof data === "string" ? utf8.enc.encode(data) : data);
    }
    this.touch();
    if (added) emit("files", { project: this });
    for (const { path } of files) emit("change", { project: this, path, source });
  }

  delete(path) {
    if (!this.files.delete(path)) return;
    this.touch();
    emit("files", { project: this });
  }

  deleteAll(paths) {
    let any = false;
    for (const path of paths) any = this.files.delete(path) || any;
    if (!any) return;
    this.touch();
    emit("files", { project: this });
  }

  rename(from, to) {
    if (from === to || !this.files.has(from) || this.files.has(to)) return false;
    this.files.set(to, this.files.get(from));
    this.files.delete(from);
    this.touch();
    emit("files", { project: this, renamed: { from, to } });
    return true;
  }

  setName(name) {
    this.name = name;
    this.touch();
    emit("name", { project: this });
  }

  touch() {
    this.modified = Date.now();
    projects.changed(this);
  }
}

// The store. Version 1 kept the data files alone, in assets; version 2
// keeps whole projects, and takes the data files, with the draft of the
// program localStorage held, into the first one.
const projects = (() => {
  const DB = "playground";
  const CURRENT = "playground-project";
  const DRAFT = "playground-source"; // version 1's draft of main.go

  let db = null;
  function openDB() {
    db ??= new Promise((resolve) => {
      try {
        const req = indexedDB.open(DB, 2);
        req.onupgradeneeded = () => {
          const d = req.result;
          if (!d.objectStoreNames.contains("assets")) d.createObjectStore("assets", { keyPath: "path" });
          if (!d.objectStoreNames.contains("projects")) d.createObjectStore("projects", { keyPath: "id" });
          if (!d.objectStoreNames.contains("contents")) d.createObjectStore("contents", { keyPath: "id" });
        };
        req.onsuccess = () => resolve(req.result);
        req.onerror = () => resolve(null);
        req.onblocked = () => resolve(null);
      } catch {
        resolve(null);
      }
    });
    return db;
  }

  // run gives fn the named stores in one transaction and resolves when it
  // is done, to what fn returned, or to null where nothing is kept.
  async function run(mode, names, fn) {
    const d = await openDB();
    if (!d) return null;
    return new Promise((resolve) => {
      try {
        const tx = d.transaction(names, mode);
        const req = fn(...names.map((n) => tx.objectStore(n)));
        tx.oncomplete = () => resolve(req && "result" in req ? req.result : true);
        tx.onerror = tx.onabort = () => resolve(null);
      } catch {
        resolve(null);
      }
    });
  }

  const self = {
    // current is the open project.
    current: null,
    // kept says whether the browser keeps projects; false in some private
    // windows, where they last as long as the page.
    kept: true,

    // list returns every project's meta, the most recently changed first.
    async list() {
      const all = await run("readonly", ["projects"], (s) => s.getAll());
      const metas = Array.isArray(all) ? all : [];
      if (self.current && !metas.some((m) => m.id === self.current.id)) metas.push(self.current.meta());
      return metas.sort((a, b) => b.modified - a.modified);
    },

    async load(id) {
      const [meta, contents] = await Promise.all([
        run("readonly", ["projects"], (s) => s.get(id)),
        run("readonly", ["contents"], (s) => s.get(id)),
      ]);
      if (!meta) return null;
      return new Project(meta, contents?.files ?? []);
    },

    // save writes a project now.
    async save(project) {
      const kept = await run("readwrite", ["projects", "contents"], (metas, contents) => {
        metas.put(project.meta());
        return contents.put({ id: project.id, files: project.list() });
      });
      self.kept = kept !== null;
      emit("saved", { project, kept: self.kept });
      return self.kept;
    },

    // changed notes that a project needs saving, a moment from now.
    timer: 0,
    dirty: null,
    changed(project) {
      self.dirty = project;
      clearTimeout(self.timer);
      self.timer = setTimeout(() => self.flush(), 400);
    },
    async flush() {
      clearTimeout(self.timer);
      const p = self.dirty;
      self.dirty = null;
      if (p) await self.save(p);
    },

    // create makes a project of files, {path, data}, and saves it.
    async create(name, files) {
      const now = Date.now();
      const id = (crypto.randomUUID?.() ?? `${now}-${Math.random().toString(36).slice(2)}`);
      const project = new Project({ id, name, created: now, modified: now }, files);
      await self.save(project);
      return project;
    },

    async remove(id) {
      // A save still pending would write it back.
      if (self.dirty?.id === id) {
        clearTimeout(self.timer);
        self.dirty = null;
      }
      await run("readwrite", ["projects", "contents"], (metas, contents) => {
        metas.delete(id);
        return contents.delete(id);
      });
    },

    // open makes project the current one, saving the one before.
    async open(project) {
      await self.flush();
      self.current = project;
      try { localStorage.setItem(CURRENT, project.id); } catch { /* not kept */ }
      emit("open", { project });
    },

    // uniqueName returns name, or name with a number after it, unlike any
    // project's.
    async uniqueName(name) {
      const taken = new Set((await self.list()).map((m) => m.name));
      if (!taken.has(name)) return name;
      for (let n = 2; ; n++) if (!taken.has(`${name} ${n}`)) return `${name} ${n}`;
    },

    // start opens the project last open, or the most recently changed, or
    // makes the first from what version 1 kept, or from fallback(), the
    // starting program's files.
    async start(fallback) {
      let id = null;
      try { id = localStorage.getItem(CURRENT); } catch { /* none */ }
      let project = id ? await self.load(id) : null;
      if (!project) {
        const metas = await self.list();
        if (metas.length) project = await self.load(metas[0].id);
      }
      if (!project) {
        const files = await self.migrate();
        project = await self.create("Untitled", files ?? await fallback());
        if (files) {
          await run("readwrite", ["assets"], (s) => s.clear());
          try { localStorage.removeItem(DRAFT); } catch { /* none */ }
        }
      }
      await self.open(project);
      return project;
    },

    // migrate gathers what version 1 kept, the draft and the data files,
    // or returns null if there was nothing.
    async migrate() {
      let draft = null;
      try { draft = localStorage.getItem(DRAFT); } catch { /* none */ }
      const assets = await run("readonly", ["assets"], (s) => s.getAll());
      const files = Array.isArray(assets) ? assets.map(({ path, data }) => ({ path, data })) : [];
      if (draft === null && !files.length) return null;
      if (draft !== null) files.unshift({ path: "main.go", data: utf8.enc.encode(draft) });
      return files;
    },
  };

  // Whatever is pending is written before the page goes.
  addEventListener("pagehide", () => { self.flush(); });
  document.addEventListener("visibilitychange", () => { if (document.hidden) self.flush(); });

  return self;
})();

// Zip files, written and read here rather than with a library: the
// archive's layout is simple enough, and the browser deflates.
const zip = (() => {
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

  async function deflate(bytes) {
    const stream = new Blob([bytes]).stream().pipeThrough(new CompressionStream("deflate-raw"));
    return new Uint8Array(await new Response(stream).arrayBuffer());
  }
  async function inflate(bytes) {
    const stream = new Blob([bytes]).stream().pipeThrough(new DecompressionStream("deflate-raw"));
    return new Uint8Array(await new Response(stream).arrayBuffer());
  }

  // dosTime is a Date as the two words a zip keeps.
  function dosTime(d) {
    const time = (d.getHours() << 11) | (d.getMinutes() << 5) | (d.getSeconds() >> 1);
    const date = ((d.getFullYear() - 1980) << 9) | ((d.getMonth() + 1) << 5) | d.getDate();
    return { time, date };
  }

  // write makes a zip of entries, {path, data}, each deflated.
  async function write(entries) {
    const parts = [];
    const central = [];
    let offset = 0;
    const { time, date } = dosTime(new Date());
    for (const { path, data } of entries) {
      const name = utf8.enc.encode(path);
      const packed = await deflate(data);
      const method = packed.length < data.length ? 8 : 0;
      const body = method === 8 ? packed : data;
      const crc = crc32(data);
      const local = new DataView(new ArrayBuffer(30));
      local.setUint32(0, 0x04034b50, true);
      local.setUint16(4, 20, true);
      local.setUint16(6, 0x0800, true); // names are UTF-8
      local.setUint16(8, method, true);
      local.setUint16(10, time, true);
      local.setUint16(12, date, true);
      local.setUint32(14, crc, true);
      local.setUint32(18, body.length, true);
      local.setUint32(22, data.length, true);
      local.setUint16(26, name.length, true);
      local.setUint16(28, 0, true);
      const header = new DataView(new ArrayBuffer(46));
      header.setUint32(0, 0x02014b50, true);
      header.setUint16(4, 20, true);
      header.setUint16(6, 20, true);
      header.setUint16(8, 0x0800, true);
      header.setUint16(10, method, true);
      header.setUint16(12, time, true);
      header.setUint16(14, date, true);
      header.setUint32(16, crc, true);
      header.setUint32(20, body.length, true);
      header.setUint32(24, data.length, true);
      header.setUint16(28, name.length, true);
      header.setUint32(42, offset, true);
      central.push(new Uint8Array(header.buffer), name);
      parts.push(new Uint8Array(local.buffer), name, body);
      offset += 30 + name.length + body.length;
    }
    let centralSize = 0;
    for (const p of central) centralSize += p.length;
    const end = new DataView(new ArrayBuffer(22));
    end.setUint32(0, 0x06054b50, true);
    end.setUint16(8, entries.length, true);
    end.setUint16(10, entries.length, true);
    end.setUint32(12, centralSize, true);
    end.setUint32(16, offset, true);
    return new Blob([...parts, ...central, new Uint8Array(end.buffer)], { type: "application/zip" });
  }

  // read returns the files of a zip, {path, data}, without its directories.
  async function read(bytes) {
    const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
    let at = bytes.length - 22;
    while (at >= 0 && view.getUint32(at, true) !== 0x06054b50) at--;
    if (at < 0) throw new Error("not a zip file");
    const count = view.getUint16(at + 10, true);
    let pos = view.getUint32(at + 16, true);
    const files = [];
    for (let i = 0; i < count; i++) {
      if (view.getUint32(pos, true) !== 0x02014b50) throw new Error("the zip's directory is damaged");
      const method = view.getUint16(pos + 10, true);
      const packedSize = view.getUint32(pos + 20, true);
      const size = view.getUint32(pos + 24, true);
      const nameLen = view.getUint16(pos + 28, true);
      const extraLen = view.getUint16(pos + 30, true);
      const commentLen = view.getUint16(pos + 32, true);
      const local = view.getUint32(pos + 42, true);
      const path = utf8.dec.decode(bytes.subarray(pos + 46, pos + 46 + nameLen));
      pos += 46 + nameLen + extraLen + commentLen;
      if (path.endsWith("/")) continue;
      const start = local + 30 + view.getUint16(local + 26, true) + view.getUint16(local + 28, true);
      const body = bytes.subarray(start, start + packedSize);
      let data;
      if (method === 0) data = body.slice();
      else if (method === 8) data = await inflate(body);
      else throw new Error(`${path} is compressed in a way this page cannot read`);
      if (data.length !== size) throw new Error(`${path} did not unpack to its size`);
      files.push({ path, data });
    }
    return files;
  }

  return { write, read };
})();

function formatSize(n) {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}

// projectFiles keeps, of a list of {path, data} from outside, the files a
// project holds: Go and other files at the root, and everything under
// data/, art/, scenes/ and pack/NAME/, with one folder wrapping them all
// taken off, as a zip of a directory has.
function projectFiles(files) {
  const clean = files
    .map(({ path, data }) => ({ path: path.replace(/\\/g, "/").replace(/^(\.\/)+/, "").replace(/^\/+/, ""), data }))
    .filter((f) => f.path && !f.path.split("/").some((p) => p === ".." || p.startsWith(".")) && !/(^|\/)__MACOSX\//.test(f.path));
  const tops = new Set(clean.map((f) => f.path.split("/")[0]));
  const wrapped = tops.size === 1 && clean.every((f) => f.path.includes("/"));
  const out = [];
  for (const { path, data } of clean) {
    const p = wrapped ? path.slice(path.indexOf("/") + 1) : path;
    const parts = p.split("/");
    if (parts.length === 1 || ((parts[0] === "data" || parts[0] === "art" || parts[0] === "scenes") && parts.length === 2) || (parts[0] === "pack" && parts.length === 3)) out.push({ path: p, data });
  }
  return out;
}

// sceneStem cleans a name for a scene file: lower case, letters, digits
// and underscores, as scenes/STEM.json.
function sceneStem(name) {
  return name.trim().toLowerCase().replace(/\.json$/, "").replace(/[^a-z0-9_]+/g, "_").replace(/^_+|_+$/g, "");
}

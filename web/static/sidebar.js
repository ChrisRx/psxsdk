// The sidebar: the project's scenes above, its files below, each opened in
// the main panel by a click (files.js). The files are the project's tree
// as a directory listing shows it: assets/, what goes on the disc beside
// the sources the go:generate lines convert it from (convert.js); pack/, a
// directory for each archive mkarc packs; then the Go files at the root.
// Folders fold, every file has the icon of its kind, and a data file
// dragged onto an archive goes into it, or back onto assets/ to come out.
// Each section's + adds to it, and a row's right-click menu holds what
// else can be done with it. Under them video memory is drawn small
// (inspect.js), opening large when clicked.
//
// exampleFiles fetches an example's data as project files, for the program
// the playground starts with; addExampleData adds them to the open project.
"use strict";

// The examples' data, which the site publishes beside their discs, with
// examples.json listing each one's files.
let catalog = [];
const catalogLoaded = (async () => {
  try {
    const res = await fetch(versioned("examples.json"));
    if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
    catalog = (await res.json()).filter((ex) => ex.assets?.length);
  } catch {
    catalog = [];
  }
})();

// exampleFiles fetches the data files of the named example, as {path, data}.
async function exampleFiles(name) {
  await catalogLoaded;
  const ex = catalog.find((e) => e.name === name);
  if (!ex) return [];
  return Promise.all(ex.assets.map(async (path) => ({ path, data: await fetchBytes(`examples/${ex.name}/${path}`) })));
}

// addExampleData puts the named example's data files in the open project,
// replacing any at the same paths without asking.
async function addExampleData(name) {
  const files = await exampleFiles(name);
  if (files.length) projects.current?.setAll(files, "assets");
}

const sidebar = (() => {
  const bar = document.querySelector(".pg-bar");
  const scenes = $("scene-list");
  const tree = $("file-tree");
  const status = $("asset-status");
  const picker = $("asset-files");

  const HINT = "Files dropped on the page go under assets/; pictures, models and sounds are converted there. Drag a file onto an archive to pack it.";
  let selected = null; // the path shown in the main panel

  function project() { return projects.current; }

  function say(text) {
    status.textContent = text;
  }

  // declaration is what a program writes to read the file at path, as the
  // examples write them, which discgen also writes for every file under
  // assets/ when a go:generate line names it.
  function declaration(path) {
    if (/^pack\/[^/]+\/[^/]+$/.test(path)) {
      // A packed file: the archive's declaration and how to reach it.
      const name = path.split("/")[1];
      const archive = convert.archiveOf(name);
      return declaration(archive) + `// a, err := arc.Open(${varName(archive)}.Data())\n// e, ok := a.Lookup("${base(path).toUpperCase()}")\n// words, err := e.Load() // or e.Unpack(buf[:]), or e.Data() for a stored entry\n`;
    }
    return `var ${varName(path)} = disc.Register("${path.toUpperCase()}", "${path}")\n`;
  }
  const base = (path) => path.split("/").pop();
  function varName(path) {
    const b = base(path);
    const dot = b.lastIndexOf(".");
    const stem = (dot > 0 ? b.slice(0, dot) : b).toLowerCase();
    const ext = dot > 0 ? b.slice(dot + 1).toUpperCase() : "";
    let name = (stem + ext).replace(/[^A-Za-z0-9_]/g, "_");
    if (!/^[A-Za-z_]/.test(name)) name = "f" + name;
    return name;
  }

  // The icons, one for each kind of file, drawn in the colour the editor
  // gives the kind's tokens.
  const ICON = {
    folder: '<svg viewBox="0 0 16 16"><path d="M1.5 4a1 1 0 0 1 1-1h3.3l1.5 1.5h6.2a1 1 0 0 1 1 1V12a1 1 0 0 1-1 1h-11a1 1 0 0 1-1-1z"/></svg>',
    open: '<svg viewBox="0 0 16 16"><path d="M1.5 12V4a1 1 0 0 1 1-1h3.3l1.5 1.5h6.2a1 1 0 0 1 1 1V7"/><path d="M1.5 12.5 3.8 7h11.2l-2.3 5.5a1 1 0 0 1-.9.5H1.5z"/></svg>',
    archive: '<svg viewBox="0 0 16 16"><path d="M2 3h12v3H2zM3 6v6.5a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1V6"/><path d="M6.5 9h3"/></svg>',
    go: '<svg viewBox="0 0 16 16"><rect x="1" y="3" width="14" height="10" rx="2" fill="currentColor" stroke="none"/><text x="8" y="11.2" text-anchor="middle" font-size="7.5" font-weight="700" font-family="Barlow, system-ui, sans-serif" fill="var(--code-bg)" stroke="none">GO</text></svg>',
    image: '<svg viewBox="0 0 16 16"><rect x="2" y="2.5" width="12" height="11" rx="1.5"/><circle cx="5.5" cy="6" r="1.3" fill="currentColor" stroke="none"/><path d="m2.5 12.5 3.5-4 2.5 3 2-2 3 3"/></svg>',
    texture: '<svg viewBox="0 0 16 16"><rect x="2" y="2" width="12" height="12" rx="1.5"/><path d="M2 8h12M8 2v12"/><path d="M2 3.5A1.5 1.5 0 0 1 3.5 2H8v6H2zM8 8h6v4.5a1.5 1.5 0 0 1-1.5 1.5H8z" fill="currentColor" stroke="none" opacity=".45"/></svg>',
    model: '<svg viewBox="0 0 16 16"><path d="M8 1.8 2.5 4.9v6.2L8 14.2l5.5-3.1V4.9z"/><path d="M2.5 4.9 8 8l5.5-3.1M8 8v6.2"/></svg>',
    sound: '<svg viewBox="0 0 16 16"><path d="M1.5 8h2l1.5-4 2 8 2-6 1.5 4 1-2h3"/></svg>',
    music: '<svg viewBox="0 0 16 16"><path d="M6 12.5V3.5l7-1.5v9"/><circle cx="4" cy="12.5" r="2"/><circle cx="11" cy="11" r="2"/></svg>',
    data: '<svg viewBox="0 0 16 16"><path d="M4 1.5h5l3.5 3.5v9.5h-8.5z"/><path d="M9 1.5V5h3.5M6 8h4M6 10.5h4"/></svg>',
    json: '<svg viewBox="0 0 16 16"><path d="M6 2.5c-1.5 0-2 .5-2 2v2c0 1-.5 1.5-1.5 1.5 1 0 1.5.5 1.5 1.5v2c0 1.5.5 2 2 2M10 2.5c1.5 0 2 .5 2 2v2c0 1 .5 1.5 1.5 1.5-1 0-1.5.5-1.5 1.5v2c0 1.5-.5 2-2 2"/></svg>',
    scene: '<svg viewBox="0 0 16 16"><path d="M2 6.5h12V13a1 1 0 0 1-1 1H3a1 1 0 0 1-1-1z"/><path d="m2 6.5 1.2-3.3 10.6 1.4-.8 1.9M5.4 3.6 4.5 6.5M9.4 4.1 8.5 6.5"/></svg>',
  };
  // iconFor names the icon and the colour class for a file.
  function iconFor(path, kind) {
    if (kind === "scene") return "scene";
    const ext = path.replace(/^.*\./, "").toLowerCase();
    if (ext === "go") return "go";
    if (/^(png|bmp|gif|jpe?g)$/.test(ext)) return "image";
    if (ext === "tim") return "texture";
    if (/^(tmd|obj)$/.test(ext)) return "model";
    if (/^(vag|vab|vh|vb|wav|aiff?)$/.test(ext)) return "sound";
    if (/^(seq|midi?)$/.test(ext)) return "music";
    if (ext === "arc") return "archive";
    if (ext === "json") return "json";
    return "data";
  }

  // Folders fold; which are folded is kept while the page lasts, and so
  // are the archives made with nothing in them yet, which exist only as
  // a line once a file is in them.
  const closed = new Set();
  const pending = new Set();

  // rows lays the tree out: {path, label, meta, depth, kind, icon, folder,
  // open}, folders first, as a listing has them, the root's files last.
  function rows() {
    const p = project();
    if (!p) return [];
    const out = [];
    const folder = (path, label, depth, meta = "", kind = "folder", icon = null) => {
      const open = !closed.has(path);
      out.push({ path, label, meta, depth, kind, folder: true, open, icon: icon ?? (open ? "open" : "folder") });
      return open;
    };
    const file = (path, depth, kind, meta) => out.push({ path, label: path.replace(/^.*\//, ""), meta, depth, kind, icon: iconFor(path, kind) });
    // Both directories are always there, to drop files on. The archives'
    // own files under assets/ are shown as the archives.
    const packs = [...new Set([...p.packs(), ...pending])].sort();
    const archives = new Set(packs.map(convert.archiveOf));
    const assets = [...p.files.keys()].filter((f) => f.startsWith("assets/") && !archives.has(f)).sort();
    if (folder("assets", "assets", 0, assets.length ? "" : "files for the disc; pictures, models and sounds are converted")) {
      for (const path of assets) {
        if (isSourceFile(path)) {
          const line = convert.lineForSource(path);
          const tool = convert.toolFor(path);
          const product = line && (line.values.get("-o") ?? line.values.get("--output"));
          const packed = product?.startsWith("pack/") ? convert.archiveOf(product.split("/")[1]) : null;
          file(path, 1, "art", line ? (packed ? `${line.tool}, into ${packed.slice(7)}` : `${line.tool}, to ${product.replace(/^.*\//, "")}`) : tool ? "not converted yet" : "read beside another");
        } else {
          const line = convert.lineForOutput(path);
          const from = line?.positional.at(-1)?.replace(/^.*\//, "");
          file(path, 1, "data", `${formatSize(p.get(path).length)}${from ? `, from ${from}` : ""}`);
        }
      }
    }
    if (folder("pack", "pack", 0, packs.length ? "" : "archives; drop a file here")) {
      for (const name of packs) {
        const archive = convert.archiveOf(name);
        const table = p.has(archive) ? convert.readArchive(p.get(archive)) : null;
        const members = p.packFiles().filter((f) => f.startsWith(`pack/${name}/`));
        const raw = members.reduce((n, f) => n + p.get(f).length, 0);
        const meta = !members.length ? "empty; drop files here" : p.has(archive) ? `${archive.slice(7)}, ${formatSize(p.get(archive).length)} (${Math.round(100 * p.get(archive).length / Math.max(1, raw))}%)` : `${archive.slice(7)}, not packed yet`;
        if (!folder(`pack/${name}`, name, 1, meta, "pack", "archive")) continue;
        for (const f of members) {
          const e = table?.find((t) => t.name === f.replace(/^.*\//, "").toUpperCase());
          const how = e ? (e.method === 1 ? `deflate, ${Math.round(100 * e.packed / Math.max(1, e.size))}%` : "stored") : "";
          file(f, 2, "member", `${formatSize(p.get(f).length)}${how ? ", " + how : ""}`);
        }
      }
    }
    for (const path of p.goFiles()) file(path, 0, "go", formatSize(p.get(path).length));
    for (const path of [...p.files.keys()].filter((f) => !f.includes("/") && !f.endsWith(".go")).sort()) file(path, 0, "other", formatSize(p.get(path).length));
    return out;
  }

  // sceneSummary says what a scene holds, for its row.
  function sceneSummary(p, path) {
    try {
      const d = JSON.parse(p.text(path));
      const parts = [];
      const n = d.objects?.length ?? 0, ns = d.sprites?.length ?? 0;
      parts.push(`${n} object${n === 1 ? "" : "s"}`);
      if (ns) parts.push(`${ns} sprite${ns === 1 ? "" : "s"}`);
      if (d.background) parts.push("a tile map");
      return parts.join(", ");
    } catch {
      return "does not parse";
    }
  }

  // isShown is whether a row stands for what the main panel shows: an
  // archive's row for the archive itself.
  function isShown(r) {
    if (r.path === selected) return true;
    return r.kind === "pack" && convert.archiveOf(r.path.slice(5)) === selected;
  }

  // Where a dragged file may go: a data file into an archive, a packed
  // file back under assets/ or into another archive, and onto pack/ itself
  // for an archive made on the spot.
  const DRAG = "text/x-psx-path";
  function canDrop(r, e) {
    const inner = [...e.dataTransfer.types].includes(DRAG);
    const outer = [...e.dataTransfer.types].includes("Files");
    if (r.kind === "pack") return inner || outer;
    if (r.path === "pack" || r.path === "assets") return inner;
    return false;
  }
  function dropped(r, path) {
    const p = project();
    if (!p.has(path)) return;
    let why = "";
    if (r.path === "assets") {
      if (path.startsWith("pack/")) why = convert.takeOut(path);
    } else {
      let name = r.kind === "pack" ? r.path.slice(5) : newArchiveName();
      if (!name) return;
      if (path.startsWith(`pack/${name}/`)) return;
      if (path.startsWith("pack/")) why = convert.takeOut(path);
      if (!why) why = convert.packInto(`assets/${path.replace(/^.*\//, "")}`, name);
      if (!why) {
        pending.delete(name);
        closed.delete("pack");
        closed.delete(`pack/${name}`);
        say(`Packed ${path.replace(/^.*\//, "")} into ${convert.archiveOf(name).slice(7)}.`);
      }
    }
    if (why) say(why);
  }

  // row makes one list item: a name and what is known of it, opened by a
  // click or Enter, with its menu on a right click.
  function row(r, { cls, role, menu, onDelete }) {
    const li = document.createElement("li");
    li.setAttribute("role", role);
    li.dataset.path = r.path;
    li.className = `files-row kind-${r.kind}${r.folder ? " folder" : ""}${cls ? " " + cls : ""}`;
    li.tabIndex = isShown(r) ? 0 : -1;
    li.setAttribute("aria-selected", String(isShown(r)));
    if (r.folder) li.setAttribute("aria-expanded", String(r.open));
    for (let d = 0; d < (r.depth ?? 0); d++) {
      const g = document.createElement("span");
      g.className = "tree-guide";
      li.append(g);
    }
    if (role === "treeitem") {
      const arrow = document.createElement("span");
      arrow.className = `tree-arrow${r.folder ? "" : " none"}`;
      if (r.folder) {
        arrow.innerHTML = '<svg viewBox="0 0 16 16"><path d="m6 3.5 4.5 4.5L6 12.5"/></svg>';
        arrow.addEventListener("click", (e) => {
          e.stopPropagation();
          toggle(r);
        });
      }
      li.append(arrow);
    }
    if (r.icon) {
      const icon = document.createElement("span");
      icon.className = `tree-icon icon-${r.icon}`;
      icon.innerHTML = ICON[r.icon];
      li.append(icon);
    }
    const name = document.createElement("span");
    name.className = "files-name";
    name.textContent = r.label;
    const meta = document.createElement("span");
    meta.className = "muted files-meta";
    meta.textContent = r.meta;
    meta.title = r.meta;
    li.append(name, meta);
    li.addEventListener("click", () => open(r));
    li.addEventListener("contextmenu", (e) => {
      e.preventDefault();
      menu(r, e);
    });
    li.addEventListener("keydown", (e) => {
      if (e.key === "Enter" || e.key === " ") {
        e.preventDefault();
        open(r);
      } else if (e.key === "ArrowDown" || e.key === "ArrowUp") {
        e.preventDefault();
        const items = [...li.parentElement.querySelectorAll(`[role=${role}]`)];
        const i = items.indexOf(li) + (e.key === "ArrowDown" ? 1 : -1);
        items[(i + items.length) % items.length]?.focus();
      } else if (r.folder && ((e.key === "ArrowLeft" && r.open) || (e.key === "ArrowRight" && !r.open))) {
        e.preventDefault();
        toggle(r, true);
      } else if (e.key === "Delete" && onDelete) {
        onDelete(r);
      }
    });
    // Dragging files between assets/ and the archives.
    if (r.kind === "data" || r.kind === "member") {
      li.draggable = true;
      li.addEventListener("dragstart", (e) => {
        e.dataTransfer.setData(DRAG, r.path);
        e.dataTransfer.effectAllowed = "move";
      });
    }
    if (r.kind === "pack" || r.path === "pack" || r.path === "assets") {
      li.addEventListener("dragover", (e) => {
        if (!canDrop(r, e)) return;
        e.preventDefault();
        e.stopPropagation();
        e.dataTransfer.dropEffect = [...e.dataTransfer.types].includes(DRAG) ? "move" : "copy";
        li.classList.add("drop-target");
      });
      li.addEventListener("dragleave", () => li.classList.remove("drop-target"));
      li.addEventListener("drop", (e) => {
        if (!canDrop(r, e)) return;
        e.preventDefault();
        e.stopPropagation();
        li.classList.remove("drop-target");
        bar.classList.remove("dropping");
        const path = e.dataTransfer.getData(DRAG);
        if (path) dropped(r, path);
        else if (r.kind === "pack") files.takeIn(e.dataTransfer.files, r.path);
      });
    }
    return li;
  }

  // toggle folds or unfolds a folder, keeping the focus on it.
  function toggle(r, focus = false) {
    if (closed.has(r.path)) closed.delete(r.path);
    else closed.add(r.path);
    render();
    if (focus) requestAnimationFrame(() => tree.querySelector(`[data-path="${CSS.escape(r.path)}"]`)?.focus());
  }

  // render draws both lists, once a frame however often it is asked, as
  // the scene editor changes its file on every drag.
  let renderQueued = 0;
  function render() {
    if (renderQueued) return;
    renderQueued = requestAnimationFrame(() => {
      renderQueued = 0;
      const p = project();
      renderScenes(p);
      renderTree(p);
    });
  }

  function renderScenes(p) {
    const list = p ? p.sceneFiles() : [];
    if (!list.length) {
      const li = document.createElement("li");
      li.className = "files-group";
      li.setAttribute("role", "presentation");
      li.textContent = "No scene yet; + makes one.";
      scenes.replaceChildren(li);
      return;
    }
    scenes.replaceChildren(...list.map((path) => row(
      { path, label: path.replace(/^scenes\//, "").replace(/\.json$/, ""), meta: sceneSummary(p, path), kind: "scene", icon: "scene" },
      { role: "option", menu: sceneMenu, onDelete: (r) => removeScene(r.path) },
    )));
    // One row can take focus even when none is shown.
    if (!scenes.querySelector('[tabindex="0"]')) scenes.querySelector("[role=option]").tabIndex = 0;
  }

  function renderTree(p) {
    for (const name of pending) if (p?.packFiles().some((f) => f.startsWith(`pack/${name}/`))) pending.delete(name);
    tree.replaceChildren(...rows().map((r) => row(r, {
      role: "treeitem",
      menu: rowMenu,
      onDelete: (r) => {
        if (r.kind === "go" || r.kind === "folder") return;
        if (r.kind === "pack") removePack(r.path.slice(5));
        else remove(r.path);
      },
    })));
    if (!tree.querySelector('[tabindex="0"]')) {
      const first = tree.querySelector("[role=treeitem]");
      if (first) first.tabIndex = 0;
    }
  }

  // open shows a row in the main panel: a folder folds instead, and an
  // archive's row shows the archive, once mkarc has made it.
  function open(r) {
    if (r.kind === "folder") {
      toggle(r);
      return;
    }
    if (r.kind === "pack") {
      const archive = convert.archiveOf(r.path.slice(5));
      if (project()?.has(archive)) files.open(archive);
      else say(pending.has(r.path.slice(5)) ? `${archive.slice(7)} is made once a file is dropped on it.` : `${archive.slice(7)} is packed when the project is built, or by Pack again in its menu.`);
      return;
    }
    files.open(r.path);
  }

  // newArchiveName asks for the name of a new archive, cleaned up, or
  // returns null.
  function newArchiveName() {
    const p = project();
    const taken = new Set([...p.packs(), ...pending]);
    let suggested = "level";
    for (let n = 2; taken.has(suggested); n++) suggested = `level${n}`;
    const name = prompt("Name of the new archive, as pack/NAME and assets/NAME.ARC", suggested);
    if (name === null) return null;
    const clean = sceneStem(name);
    if (!clean) return null;
    if (taken.has(clean)) {
      alert(`There is already an archive ${clean}.`);
      return null;
    }
    if (convert.lineForOutput(convert.archiveOf(clean))) {
      alert(`${convert.archiveOf(clean)} is already made by another line.`);
      return null;
    }
    return clean;
  }
  // newArchive makes an empty archive, which shows in the tree until a
  // file is dropped on it.
  function newArchive() {
    const name = newArchiveName();
    if (!name) return;
    pending.add(name);
    closed.delete("pack");
    render();
    say(`Drop files on ${name} to pack them, or pick them from its menu.`);
  }
  // packMenu lists the data files not yet in an archive, to put one in it.
  function packMenu(name, at) {
    const p = project();
    const loose = p.dataFiles().filter((f) => !/\.arc$/i.test(f) || !convert.lineForPack(f.replace(/^assets\//, "").replace(/\.ARC$/i, "").toLowerCase()));
    if (!loose.length) {
      say("Every file is in an archive already; add files from this computer first.");
      return;
    }
    files.popupMenu(loose.map((f) => [f.slice(7), formatSize(p.get(f).length), () => dropped({ kind: "pack", path: `pack/${name}` }, f)]), at);
  }

  // mark says what the main panel shows, so that its row stands out.
  function mark(path) {
    selected = path;
    render();
  }

  // sceneMenu opens the actions for a scene's row.
  function sceneMenu(r, at) {
    const p = project();
    files.popupMenu([
      ["Download", "The scene as the editor writes it", () => files.download(r.path.replace(/^.*\//, ""), p.get(r.path))],
      ["Rename", "Give the scene another name; its mkscene line and its data file follow", () => renameScene(r.path)],
      ["Remove", "Take the scene out of the project with its mkscene line and its data file; code that loads it needs changing", () => removeScene(r.path)],
    ], at);
  }

  // rowMenu opens the actions for a file's row.
  function rowMenu(r, at) {
    const p = project();
    const line = r.kind === "art" ? convert.lineForSource(r.path) : r.kind === "data" ? convert.lineForOutput(r.path) : null;
    const items = [];
    if (r.kind === "folder") {
      if (r.path === "pack") items.push(["New archive…", "An archive of its own under pack/, packed into assets/NAME.ARC by an mkarc line", newArchive]);
      else items.push(["Add files from this computer…", "Files for the disc as they are; pictures, models and sounds converted by a go:generate line each", () => picker.click()]);
      files.popupMenu(items, at);
      return;
    }
    if (r.kind === "art" && convert.toolFor(r.path)) items.push([line ? "Convert again" : `Convert with ${convert.toolFor(r.path)}`, "Run the converter over it", () => convert.added([r.path])]);
    if (r.kind === "data") items.push(["Copy declaration", "The Go that registers this file, which discgen writes for every file under assets/ when a line names it", async () => {
      try {
        await navigator.clipboard.writeText(declaration(r.path));
        say(`Copied the declaration of ${r.path}.`);
      } catch (e) {
        say(`Could not copy: ${e.message}`);
      }
    }]);
    if (r.kind === "pack") {
      const name = r.path.slice(5);
      const archive = convert.archiveOf(name);
      items.push(["Copy declaration", "The Go that registers the archive, and how a program opens it", async () => {
        try {
          await navigator.clipboard.writeText(declaration(`pack/${name}/x`));
          say(`Copied the declaration of ${archive}.`);
        } catch (e) {
          say(`Could not copy: ${e.message}`);
        }
      }]);
      items.push(["Add a file…", "Move one of the files under assets/ into the archive", () => packMenu(name, at)]);
      items.push(["Pack again", "Run mkarc over the archive's files", () => convert.run(), pending.has(name)]);
      items.push(["Download", "Save the archive to this computer", () => files.download(archive.slice(7), p.get(archive)), !p.has(archive)]);
      items.push(["Unpack all", "Move every file back under assets/, on the disc by itself, and take the archive and its line out", () => {
        for (const f of p.packFiles().filter((x) => x.startsWith(`pack/${name}/`))) {
          const why = convert.takeOut(f);
          if (why) {
            say(why);
            return;
          }
        }
        say(`Unpacked ${archive}.`);
      }]);
      items.push(["Remove", "Take the archive out of the project with every file in it and its line", () => (pending.has(name) ? (pending.delete(name), render()) : removePack(name))]);
      files.popupMenu(items, at);
      return;
    }
    if (r.kind === "member") {
      items.push(["Copy declaration", "The Go that registers the archive, and how a program reaches this file in it", async () => {
        try {
          await navigator.clipboard.writeText(declaration(r.path));
          say(`Copied how to reach ${r.path.replace(/^.*\//, "")}.`);
        } catch (e) {
          say(`Could not copy: ${e.message}`);
        }
      }]);
      items.push(["Take out of the archive", "Move the file back under assets/, on the disc by itself", () => { const why = convert.takeOut(r.path); if (why) say(why); }]);
      items.push(["Download", "Save this file to this computer", () => files.download(r.path.replace(/^.*\//, ""), p.get(r.path))]);
      items.push(["Rename", "Its name in the archive is its file name; take it out to rename it", null, true]);
      items.push(["Remove", "Take the file out of the project", () => remove(r.path)]);
      files.popupMenu(items, at);
      return;
    }
    if (r.kind === "data" && !/\.arc$/i.test(r.path)) {
      for (const name of [...new Set([...p.packs(), ...pending])].sort()) {
        items.push([`Pack into ${name}`, `Move the file under pack/${name}/, packed with the others there into ${convert.archiveOf(name)}`, () => dropped({ kind: "pack", path: `pack/${name}` }, r.path)]);
      }
      items.push(["Pack into a new archive…", "Move the file under pack/NAME/, packed into assets/NAME.ARC by an mkarc line, which the program reads with package arc", () => dropped({ kind: "folder", path: "pack" }, r.path)]);
    }
    items.push(["Download", "Save this file to this computer", () => files.download(r.path.replace(/^.*\//, ""), p.get(r.path))]);
    if (r.kind === "go") {
      const isMain = r.path === "main.go";
      items.push(["Rename", isMain ? "main.go is where the program starts, and keeps its name" : "Give the file another name", () => files.renameFile(r.path), isMain]);
      items.push(["Remove", isMain ? "main.go is where the program starts; a project always has one" : "Take the file out of the project", () => remove(r.path), isMain]);
    } else {
      const fixed = line ? (r.kind === "art" ? "Its go:generate line names it; change the line to rename it" : "Its go:generate line names it; change the line's -o to rename it") : "";
      items.push(["Rename", fixed || "Give the file another name", () => rename(r.path), !!fixed]);
      items.push(["Remove", r.kind === "art" ? "Take the source out, with its go:generate line and the file it made" : "Take the file out of the project", () => remove(r.path)]);
    }
    files.popupMenu(items, at);
  }

  function rename(path) {
    const p = project();
    const dir = path.includes("/") ? path.slice(0, path.lastIndexOf("/") + 1) : "";
    const name = prompt(`Rename ${path} to`, path.slice(dir.length));
    if (name === null) return;
    const clean = name.trim().replace(/^.*[\\/]/, "").replace(/\s+/g, "_");
    if (!clean || clean === path.slice(dir.length)) return;
    if (p.has(dir + clean)) {
      alert(`There is already a ${dir + clean}.`);
      return;
    }
    if (p.rename(path, dir + clean) && selected === path) files.open(dir + clean);
  }

  // Removing: a source goes with its go:generate line and the file it
  // made; everything removed can be put back with Undo in the status
  // line, or Ctrl+Z, for a while.
  const removed = []; // [{files: [{path, data}], lines: [{file, text}]}]
  function remove(path) {
    const p = project();
    if (!p?.has(path)) return;
    const files = [{ path, data: p.get(path) }];
    const lines = [];
    if (isSourceFile(path)) {
      const line = convert.lineForSource(path);
      if (line) {
        lines.push({ file: line.file, text: line.text });
        const product = line.values.get("-o") ?? line.values.get("--output");
        if (product && p.has(product)) files.push({ path: product, data: p.get(product) });
      }
    }
    const what = files.map((f) => f.path).join(" and ") + (lines.length ? ", with its go:generate line" : "");
    if (!confirm(`Remove ${what} from the project?`)) return;
    for (const { file, text } of lines) {
      const rows = p.text(file).split("\n");
      const i = rows.indexOf(text);
      if (i >= 0) {
        rows.splice(i, 1);
        p.set(file, rows.join("\n"), "generate");
      }
    }
    p.deleteAll(files.map((f) => f.path));
    removed.push({ files, lines });
    sayUndo(`Removed ${what}.`);
    if (path.startsWith("pack/")) convert.packChanged(path.split("/")[1]);
  }
  function removePack(name) {
    const p = project();
    const members = p.packFiles().filter((f) => f.startsWith(`pack/${name}/`));
    const archive = convert.archiveOf(name);
    if (!confirm(`Remove ${archive} and the ${members.length} file${members.length === 1 ? "" : "s"} packed in it from the project?`)) return;
    const files = members.map((f) => ({ path: f, data: p.get(f) }));
    if (p.has(archive)) files.push({ path: archive, data: p.get(archive) });
    const lines = [];
    const line = convert.lineForPack(name);
    if (line) {
      lines.push({ file: line.file, text: line.text });
      const rows = p.text(line.file).split("\n");
      const i = rows.indexOf(line.text);
      if (i >= 0) {
        rows.splice(i, 1);
        p.set(line.file, rows.join("\n"), "generate");
      }
    }
    p.deleteAll(files.map((f) => f.path));
    removed.push({ files, lines });
    sayUndo(`Removed ${archive}.`);
  }
  // A scene goes with its mkscene line and the data file the line makes.
  function sceneParts(path) {
    const p = project();
    const line = convert.lineForSource(path);
    const product = line ? (line.values.get("-o") ?? line.values.get("--output")) : null;
    return { line, product: product && p.has(product) ? product : null };
  }
  function removeScene(path) {
    const p = project();
    if (!p?.has(path)) return;
    const { line, product } = sceneParts(path);
    const what = path + (product ? ` and ${product}` : "") + (line ? ", with its mkscene line" : "");
    if (!confirm(`Remove ${what} from the project? Code that loads the scene needs changing by hand.`)) return;
    const files = [{ path, data: p.get(path) }];
    if (product) files.push({ path: product, data: p.get(product) });
    const lines = [];
    if (line) {
      lines.push({ file: line.file, text: line.text });
      const rows = p.text(line.file).split("\n");
      const i = rows.indexOf(line.text);
      if (i >= 0) {
        rows.splice(i, 1);
        p.set(line.file, rows.join("\n"), "generate");
      }
    }
    p.deleteAll(files.map((f) => f.path));
    removed.push({ files, lines });
    sayUndo(`Removed ${what}.`);
  }
  function renameScene(path) {
    const p = project();
    const stem = path.replace(/^scenes\//, "").replace(/\.json$/, "");
    const name = prompt("Rename the scene to", stem);
    if (name === null) return;
    const clean = sceneStem(name);
    if (!clean || clean === stem) return;
    const to = `scenes/${clean}.json`;
    if (p.has(to)) {
      alert(`There is already a scene ${clean}.`);
      return;
    }
    const { line, product } = sceneParts(path);
    const newProduct = convert.outputFor(to, "mkscene");
    if (product && product !== newProduct && p.has(newProduct)) {
      alert(`There is already a ${newProduct}, which the renamed scene would be cooked into.`);
      return;
    }
    if (line) {
      const rows = p.text(line.file).split("\n");
      if (rows[line.index] === line.text) {
        rows[line.index] = line.text.split(/\s+/).map((w) => (w === path ? to : product && w === product ? newProduct : w)).join(" ");
        p.set(line.file, rows.join("\n"), "generate");
      }
    }
    const shown = selected === path;
    p.rename(path, to);
    if (product && product !== newProduct) p.rename(product, newProduct);
    if (shown) files.open(to);
  }
  function undoRemove() {
    const last = removed.pop();
    const p = project();
    if (!last || !p) return;
    p.setAll(last.files, "files");
    for (const { file, text } of last.lines) {
      if (!p.has(file)) continue;
      const rows = p.text(file).split("\n");
      // Back before discgen's line, or at the end of the lines.
      let at = rows.findIndex((r) => /^\/\/go:generate\b.*\bdiscgen\b/.test(r));
      if (at < 0) {
        const all = rows.map((r, i) => (/^\/\/go:generate\b/.test(r) ? i : -1)).filter((i) => i >= 0);
        at = all.length ? all.at(-1) + 1 : rows.findIndex((r) => /^package\s+main\b/.test(r)) + 1;
      }
      rows.splice(at, 0, text);
      p.set(file, rows.join("\n"), "generate");
    }
    say(`Put back ${last.files.map((f) => f.path).join(" and ")}.`);
    if (last.files.some((f) => isSourceFile(f.path))) convert.run();
  }
  // sayUndo says what happened with an Undo beside it.
  function sayUndo(text) {
    status.replaceChildren(document.createTextNode(text + " "));
    const b = document.createElement("button");
    b.type = "button";
    b.className = "small-button";
    b.textContent = "Undo";
    b.addEventListener("click", () => undoRemove());
    status.append(b);
  }
  bar.addEventListener("keydown", (e) => {
    if ((e.ctrlKey || e.metaKey) && (e.key === "z" || e.key === "Z") && removed.length && !e.target.closest("input, textarea")) {
      e.preventDefault();
      undoRemove();
    }
  });

  // Adding. A scene is made by the scene editor and opened; the files
  // menu offers a Go file, files from this computer, which go in as
  // dropped ones do (files.js), and an example's data.
  $("scene-add").addEventListener("click", () => {
    const path = sceneTab.create();
    if (path) files.open(path);
  });
  $("file-add").addEventListener("click", (e) => addMenu(e.currentTarget));
  picker.addEventListener("change", async () => {
    await files.takeIn(picker.files);
    picker.value = "";
  });

  function addMenu(at) {
    files.popupMenu([
      ["New Go file", "A file beside main.go, in the same package", () => files.newFile()],
      ["Add files from this computer…", "Go files go beside main.go; anything else under assets/, where pictures, models and sounds are converted", () => picker.click()],
      ["New archive…", "An archive under pack/, packed into assets/NAME.ARC by an mkarc line; files dropped on it go in", newArchive],
      ["Add an example's data…", "The pictures, models and sounds one of the site's examples uses", () => exampleMenu(at), !catalog.length],
    ], at);
  }
  function exampleMenu(at) {
    files.popupMenu(catalog.map((ex) => [ex.name, `${ex.assets.length} file${ex.assets.length > 1 ? "s" : ""}: ${ex.assets.map((d) => d.replace(/^.*\//, "")).join(", ")}`, () => addFromExample(ex.name)]), at);
  }
  async function addFromExample(name) {
    const p = project();
    if (!name || !p) return;
    say(`Fetching ${name}'s data…`);
    try {
      const added = await exampleFiles(name);
      const replaced = added.filter((f) => p.has(f.path)).map((f) => f.path);
      if (replaced.length && !confirm(`Replace ${replaced.join(", ")}?`)) {
        say(HINT);
        return;
      }
      p.setAll(added, "assets");
      say(`Added ${added.length === 1 ? added[0].path : `${added.length} files`}.`);
    } catch (e) {
      say(`Could not fetch ${name}'s data: ${e.message}`);
    }
  }

  // The divider between the scenes and the files, dragged or moved with
  // the arrow keys; where it sits is remembered.
  const divider = $("side-split");
  const SCENES = "playground-scenes";
  function setScenes(px) {
    const max = bar.clientHeight - 160;
    const h = Math.round(Math.max(72, Math.min(px, max)));
    bar.style.setProperty("--scenes", `${h}px`);
    return h;
  }
  function keepScenes(h) {
    try { localStorage.setItem(SCENES, String(h)); } catch { /* not kept */ }
  }
  try {
    const saved = Number(localStorage.getItem(SCENES));
    if (saved) setScenes(saved);
  } catch { /* the default */ }
  divider.addEventListener("pointerdown", (e) => {
    divider.setPointerCapture(e.pointerId);
    document.body.classList.add("resizing-rows");
    const top = bar.getBoundingClientRect().top;
    const move = (ev) => setScenes(ev.clientY - top - divider.offsetHeight / 2);
    const up = (ev) => {
      divider.removeEventListener("pointermove", move);
      divider.removeEventListener("pointerup", up);
      document.body.classList.remove("resizing-rows");
      keepScenes(setScenes(ev.clientY - top - divider.offsetHeight / 2));
    };
    divider.addEventListener("pointermove", move);
    divider.addEventListener("pointerup", up);
  });
  divider.addEventListener("keydown", (e) => {
    if (e.key !== "ArrowUp" && e.key !== "ArrowDown") return;
    e.preventDefault();
    keepScenes(setScenes($("side-scenes").offsetHeight + (e.key === "ArrowDown" ? 24 : -24)));
  });

  projectEvents.addEventListener("open", () => {
    selected = null;
    closed.clear();
    pending.clear();
    say(HINT);
    render();
  });
  projectEvents.addEventListener("files", render);
  projectEvents.addEventListener("change", ({ detail: { path, source } }) => {
    if (path.startsWith("assets/") || path.startsWith("scenes/") || (path.endsWith(".go") && source !== "editor")) render();
  });
  projectEvents.addEventListener("status", ({ detail: { text } }) => say(text));
  say(HINT);

  return { mark, say, remove, removeScene, renameScene, undoRemove, canUndo: () => removed.length > 0 };
})();

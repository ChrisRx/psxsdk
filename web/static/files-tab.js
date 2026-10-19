// The Files tab: the project's files as one tree, with the preview of the
// one picked beside it (inspect.js). The program's Go files and the scene
// come first; then the sources under art/, each with the data file its
// go:generate line makes nested under it (convert.js); then the rest of
// data/, which goes on the disc as it is, and video memory, where every
// TIM sits. Clicking a row picks it and shows it; Enter or a double click
// opens a Go file or the scene in its own tab; the row's menu holds what
// else can be done with it.
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
    catalog = (await res.json()).filter((ex) => ex.data?.length);
  } catch {
    catalog = [];
  }
})();

// exampleFiles fetches the data files of the named example, as {path, data}.
async function exampleFiles(name) {
  await catalogLoaded;
  const ex = catalog.find((e) => e.name === name);
  if (!ex) return [];
  return Promise.all(ex.data.map(async (path) => ({ path, data: await fetchBytes(`examples/${ex.name}/${path}`) })));
}

// addExampleData puts the named example's data files in the open project,
// replacing any at the same paths without asking.
async function addExampleData(name) {
  const files = await exampleFiles(name);
  if (files.length) projects.current?.setAll(files, "assets");
}

const filesTab = (() => {
  const pane = $("files-pane");
  const tree = $("file-tree");
  const status = $("asset-status");
  const picker = $("asset-files");
  const examples = $("example-data");

  const VRAM = "vram"; // the pseudo-path of the video memory row
  let selected = null; // a path, or VRAM
  let shown = false;

  function project() { return projects.current; }

  function say(text) {
    status.textContent = text;
  }

  // declaration is what a program writes to read the file at path, as the
  // examples write them, which discgen also writes for every file under
  // data/ when a go:generate line names it.
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

  // rows lays the tree out: {path, label, meta, depth, group, kind}.
  function rows() {
    const p = project();
    if (!p) return [];
    const out = [];
    const head = (label) => out.push({ group: label });
    head("Program");
    for (const path of p.goFiles()) out.push({ path, label: path, meta: formatSize(p.get(path).length), kind: "go" });
    for (const path of [...p.files.keys()].filter((f) => !f.includes("/") && !f.endsWith(".go")).sort()) {
      out.push({ path, label: path, meta: formatSize(p.get(path).length), kind: "other" });
    }
    head("Scenes, placed in the Scene tab");
    for (const path of p.sceneFiles()) out.push({ path, label: path.replace(/^scenes\//, "").replace(/\.json$/, ""), meta: sceneSummary(p, path), kind: "scene" });
    out.push({ path: "scenes", label: "New scene…", meta: "", kind: "newscene" });
    const art = p.artFiles();
    const made = new Set();
    if (art.length) head("Sources, converted by the go:generate lines");
    for (const path of art) {
      const line = convert.lineForSource(path);
      const tool = convert.toolFor(path);
      const product = line && (line.values.get("-o") ?? line.values.get("--output"));
      const packed = product?.startsWith("pack/") ? convert.archiveOf(product.split("/")[1]) : null;
      out.push({ path, label: path.slice(4), meta: line ? (packed ? `${line.tool}, into ${packed.slice(5)}` : line.tool) : tool ? "not converted yet" : "read beside another", kind: "art" });
      if (product && p.has(product) && !packed) {
        made.add(product);
        out.push({ path: product, label: product.slice(5), meta: formatSize(p.get(product).length), depth: 1, kind: "data", madeBy: path });
      }
    }
    const packs = p.packs();
    if (packs.length) head("Archives, packed by mkarc");
    for (const name of packs) {
      const archive = convert.archiveOf(name);
      made.add(archive);
      const table = p.has(archive) ? convert.readArchive(p.get(archive)) : null;
      const members = p.packFiles().filter((f) => f.startsWith(`pack/${name}/`));
      const raw = members.reduce((n, f) => n + p.get(f).length, 0);
      const meta = p.has(archive) ? `${members.length} file${members.length === 1 ? "" : "s"}, ${formatSize(p.get(archive).length)} (${Math.round(100 * p.get(archive).length / Math.max(1, raw))}%)` : `${members.length} file${members.length === 1 ? "" : "s"}, not packed yet`;
      out.push({ path: `pack/${name}`, label: archive.slice(5), meta, kind: "pack" });
      for (const f of members) {
        const e = table?.find((t) => t.name === f.replace(/^.*\//, "").toUpperCase());
        const how = e ? (e.method === 1 ? `deflate, ${Math.round(100 * e.packed / Math.max(1, e.size))}%` : "stored") : "";
        out.push({ path: f, label: f.replace(/^.*\//, ""), meta: `${formatSize(p.get(f).length)}${how ? ", " + how : ""}`, depth: 1, kind: "member" });
      }
    }
    const data = p.dataFiles().filter((f) => !made.has(f));
    head(art.length || packs.length ? "Data, on the disc as it is" : "Data, on the disc");
    for (const path of data) out.push({ path, label: path.slice(5), meta: formatSize(p.get(path).length), kind: "data" });
    const tims = p.onDisc().filter((f) => /\.tim$/i.test(f)).length;
    out.push({ path: VRAM, label: "Video memory", meta: `${tims} TIM${tims === 1 ? "" : "s"}`, kind: "vram" });
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

  function render() {
    const list = rows();
    const p = project();
    if (selected && selected !== VRAM && !p?.has(selected)) selected = null;
    tree.replaceChildren(...list.map((r) => {
      if (r.group) {
        const li = document.createElement("li");
        li.className = "files-group";
        li.setAttribute("role", "presentation");
        li.textContent = r.group;
        return li;
      }
      const li = document.createElement("li");
      li.setAttribute("role", "treeitem");
      li.dataset.path = r.path;
      li.className = `files-row kind-${r.kind}${r.depth ? " nested" : ""}`;
      li.tabIndex = r.path === selected ? 0 : -1;
      li.setAttribute("aria-selected", String(r.path === selected));
      const name = document.createElement("span");
      name.className = "files-name";
      name.textContent = r.label;
      const meta = document.createElement("span");
      meta.className = "muted files-meta";
      meta.textContent = r.meta;
      li.append(name, meta);
      if (r.path === selected && r.kind !== "vram") {
        const more = document.createElement("button");
        more.type = "button";
        more.className = "file-more";
        more.setAttribute("aria-label", `Actions for ${r.path}`);
        more.textContent = "⋯";
        more.addEventListener("click", (e) => {
          e.stopPropagation();
          rowMenu(r, more);
        });
        li.append(more);
      }
      li.addEventListener("click", () => {
        if (r.kind === "newscene") {
          const path = sceneTab.create();
          if (path) files.open(path);
          return;
        }
        select(r.path);
      });
      li.addEventListener("dblclick", () => open(r));
      li.addEventListener("contextmenu", (e) => {
        e.preventDefault();
        if (r.kind === "newscene" || r.kind === "vram") return;
        if (r.kind === "pack") select(r.path);
        select(r.path);
        rowMenu(r, e);
      });
      li.addEventListener("keydown", (e) => {
        if (e.key === "Enter") open(r);
        else if (e.key === "ArrowDown" || e.key === "ArrowUp") {
          e.preventDefault();
          const items = [...tree.querySelectorAll("[role=treeitem]")];
          const i = items.indexOf(li) + (e.key === "ArrowDown" ? 1 : -1);
          const next = items[(i + items.length) % items.length];
          select(next.dataset.path);
          tree.querySelector('[aria-selected="true"]')?.focus();
        } else if (e.key === "Delete" && r.kind !== "go" && r.kind !== "vram" && r.kind !== "newscene") {
          if (r.kind === "scene") removeScene(r.path);
          else if (r.kind === "pack") removePack(r.path.slice(5));
          else remove(r.path);
        }
      });
      return li;
    }));
  }

  // select picks a row and shows it; an archive's row shows the archive.
  function select(path) {
    selected = path;
    render();
    inspect.show(/^pack\/[^/]+$/.test(path) ? convert.archiveOf(path.slice(5)) : path);
  }

  // open takes a Go file or the scene to its own tab.
  function open(r) {
    if (r.kind === "go" || r.kind === "scene") files.open(r.path);
  }

  // rowMenu opens the actions for a row under its button.
  function rowMenu(r, at) {
    const p = project();
    const line = r.kind === "art" ? convert.lineForSource(r.path) : r.kind === "data" ? convert.lineForOutput(r.path) : null;
    const items = [];
    if (r.kind === "go" || r.kind === "scene") items.push(["Open", r.kind === "scene" ? "Place it in the Scene tab" : "Edit it in its own tab", () => open(r)]);
    if (r.kind === "scene") {
      items.push(["Download", "The scene as the editor writes it", () => files.download(r.path.replace(/^.*\//, ""), p.get(r.path))]);
      items.push(["Rename", "Give the scene another name; its mkscene line and its data file follow", () => renameScene(r.path)]);
      items.push(["Remove", "Take the scene out of the project with its mkscene line and its data file; code that loads it needs changing", () => removeScene(r.path)]);
      popup(items, at);
      return;
    }
    if (r.kind === "art" && convert.toolFor(r.path)) items.push([line ? "Convert again" : `Convert with ${convert.toolFor(r.path)}`, "Run the converter over it", () => convert.added([r.path])]);
    if (r.kind === "data") items.push(["Copy declaration", "The Go that registers this file, which discgen writes for every file under data/ when a line names it", async () => {
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
      items.push(["Pack again", "Run mkarc over the archive's files", () => convert.run()]);
      items.push(["Download", "Save the archive to this computer", () => files.download(archive.slice(5), p.get(archive)), !p.has(archive)]);
      items.push(["Unpack all", "Move every file back under data/, on the disc by itself, and take the archive and its line out", () => {
        for (const f of p.packFiles().filter((x) => x.startsWith(`pack/${name}/`))) {
          const why = convert.takeOut(f);
          if (why) {
            say(why);
            return;
          }
        }
        say(`Unpacked ${archive}.`);
      }]);
      items.push(["Remove", "Take the archive out of the project with every file in it and its line", () => removePack(name)]);
      popup(items, at);
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
      items.push(["Take out of the archive", "Move the file back under data/, on the disc by itself", () => { const why = convert.takeOut(r.path); if (why) say(why); }]);
      items.push(["Download", "Save this file to this computer", () => files.download(r.path.replace(/^.*\//, ""), p.get(r.path))]);
      items.push(["Rename", "Its name in the archive is its file name; take it out to rename it", null, true]);
      items.push(["Remove", "Take the file out of the project", () => remove(r.path)]);
      popup(items, at);
      return;
    }
    if (r.kind === "data" && !/\.arc$/i.test(r.path)) {
      items.push(["Pack into an archive…", "Move the file under pack/NAME/, packed with the others there into data/NAME.ARC by an mkarc line, which the program reads with package arc", () => {
        const name = prompt("Pack into which archive? A name for it, as pack/NAME and data/NAME.ARC", p.packs()[0] ?? "level");
        if (name === null) return;
        const clean = sceneStem(name);
        if (!clean) return;
        const why = convert.packInto(r.path, clean);
        if (why) say(why);
        else say(`Packed ${r.path.slice(5)} into ${convert.archiveOf(clean)}.`);
      }]);
    }
    items.push(["Download", "Save this file to this computer", () => files.download(r.path.replace(/^.*\//, ""), p.get(r.path))]);
    if (r.kind !== "go" && r.kind !== "scene") {
      const fixed = line ? (r.kind === "art" ? "Its go:generate line names it; change the line to rename it" : "Its go:generate line names it; change the line's -o to rename it") : "";
      items.push(["Rename", fixed || "Give the file another name", () => rename(r.path), !!fixed]);
    }
    if (r.kind !== "go") items.push(["Remove", r.kind === "art" ? "Take the source out, with its go:generate line and the file it made" : "Take the file out of the project", () => remove(r.path)]);
    else if (r.path !== "main.go") items.push(["Remove", "Take the file out of the project", () => remove(r.path)]);
    popup(items, at);
  }

  let menu = null;
  function popup(items, at) {
    closeMenu();
    menu = document.createElement("div");
    menu.className = "pg-popup";
    menu.setAttribute("role", "menu");
    for (const [label, title, fn, disabled] of items) {
      const b = document.createElement("button");
      b.type = "button";
      b.setAttribute("role", "menuitem");
      b.textContent = label;
      b.title = title;
      b.disabled = !!disabled;
      b.addEventListener("click", () => {
        closeMenu();
        fn();
      });
      menu.append(b);
    }
    document.body.append(menu);
    const r = at instanceof Event ? { left: at.clientX, bottom: at.clientY } : at.getBoundingClientRect();
    menu.style.left = `${Math.max(4, Math.min(r.left, innerWidth - menu.offsetWidth - 8))}px`;
    menu.style.top = `${Math.min(r.bottom + 4, innerHeight - menu.offsetHeight - 8)}px`;
    menu.querySelector("button:not(:disabled)")?.focus();
    setTimeout(() => addEventListener("pointerdown", onOutside), 0);
    addEventListener("keydown", onEscape);
  }
  function onOutside(e) { if (menu && !menu.contains(e.target)) closeMenu(); }
  function onEscape(e) { if (e.key === "Escape") closeMenu(); }
  function closeMenu() {
    menu?.remove();
    menu = null;
    removeEventListener("pointerdown", onOutside);
    removeEventListener("keydown", onEscape);
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
    if (p.rename(path, dir + clean)) select(dir + clean);
  }

  // Removing: a source goes with its go:generate line and the file it
  // made; everything removed can be put back with Undo in the status
  // line, or Ctrl+Z here, for a while.
  const removed = []; // [{files: [{path, data}], lines: [{file, text}]}]
  function remove(path) {
    const p = project();
    if (!p?.has(path)) return;
    const files = [{ path, data: p.get(path) }];
    const lines = [];
    if (path.startsWith("art/")) {
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
    p.rename(path, to);
    if (product && product !== newProduct) p.rename(product, newProduct);
    select(to);
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
    if (last.files.some((f) => f.path.startsWith("art/"))) convert.run();
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
  tree.addEventListener("keydown", (e) => {
    if ((e.ctrlKey || e.metaKey) && (e.key === "z" || e.key === "Z") && removed.length) {
      e.preventDefault();
      undoRemove();
    }
  });

  // Adding files: picked here, they go in as dropped ones do (files.js).
  $("add-assets").addEventListener("click", (e) => files.newMenu(e.currentTarget));
  picker.addEventListener("change", async () => {
    await files.takeIn(picker.files);
    picker.value = "";
  });

  catalogLoaded.then(() => {
    for (const ex of catalog) {
      const option = document.createElement("option");
      option.value = ex.name;
      option.textContent = `${ex.name} (${ex.data.length} file${ex.data.length > 1 ? "s" : ""})`;
      examples.append(option);
    }
    examples.disabled = !catalog.length;
  });
  examples.addEventListener("change", async () => {
    const name = examples.value;
    examples.value = "";
    const p = project();
    if (!name || !p) return;
    examples.disabled = true;
    say(`Fetching ${name}'s data…`);
    try {
      const added = await exampleFiles(name);
      const replaced = added.filter((f) => p.has(f.path)).map((f) => f.path);
      if (replaced.length && !confirm(`Replace ${replaced.join(", ")}?`)) return;
      p.setAll(added, "assets");
      say(`Added ${added.length === 1 ? added[0].path : `${added.length} files`}.`);
    } catch (e) {
      say(`Could not fetch ${name}'s data: ${e.message}`);
    } finally {
      examples.disabled = false;
    }
  });

  function show() {
    shown = true;
    pane.hidden = false;
    render();
    if (selected) inspect.show(selected);
    else {
      const first = rows().find((r) => r.kind === "data" || r.kind === "art");
      if (first) select(first.path);
      else inspect.show(VRAM);
    }
  }
  function hide() {
    shown = false;
    pane.hidden = true;
    inspect.hide();
  }

  projectEvents.addEventListener("open", () => {
    selected = null;
    say("Pictures, models and sounds dropped on the page go under art/ to be converted; anything else under data/.");
    if (shown) render();
  });
  projectEvents.addEventListener("files", () => { if (shown) render(); });
  projectEvents.addEventListener("change", ({ detail: { path, source } }) => {
    if (!shown) return;
    if (path.startsWith("data/") || path.startsWith("art/") || (path.endsWith(".go") && source !== "editor")) render();
  });
  projectEvents.addEventListener("status", ({ detail: { text } }) => say(text));

  return { show, hide, shown: () => shown, select, say, remove, removeScene, renameScene, undoRemove, canUndo: () => removed.length > 0 };
})();

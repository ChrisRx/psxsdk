// What the middle of the page shows, and in which layout (layout.js): a Go
// file opens in an editor of its own, with a tab over the editors, closed
// with its cross, in the Code layout; a scene in the scene editor
// (scene-tab.js), in Scenes; a picture under assets/ in the sprite editor
// (sprite.js), and a TIM, a TMD, a source, an archive or video memory in
// the viewer (inspect.js), in Assets, with what they are and their
// converter's options in the column on the right. Each layout remembers
// what it showed last. Also here: the project menu, where projects are
// opened, made, renamed, downloaded as a zip, imported and deleted; the
// page with no project, which offers to make one; and files dropped on
// the panel, which join the project: a zip as a new project, Go files
// beside main.go, and anything else under assets/, pictures, models and
// sounds to be converted there.
//
// playground.js builds and formats through files.active, files.goTexts and
// files.markErrors, and starts everything with projects.start().
"use strict";

// starterList is the examples a project can start from, as the site lists
// them in starters.json: each one's name, what it is, its picture and its
// files.
let starters = null;
function starterList() {
  starters ??= (async () => {
    const res = await fetch(versioned("starters.json"));
    if (!res.ok) throw new Error(`starters.json: ${res.status} ${res.statusText}`);
    return res.json();
  })();
  starters.catch(() => { starters = null; });
  return starters;
}

// startingFiles is the files a new project is made of: the named one of
// the examples the site lists in starters.json, with its assets.
async function startingFiles(name) {
  const list = await starterList();
  const starter = list.find((s) => s.name === name);
  if (!starter) throw new Error(`the site does not offer ${name} to start from`);
  const files = [];
  for (const [path, url] of Object.entries(starter.files)) {
    const r = await fetch(versioned(url));
    if (!r.ok) throw new Error(`${url}: ${r.status} ${r.statusText}`);
    // Files under assets/ and pack/ are bytes.
    files.push({ path, data: path.startsWith("assets/") || path.startsWith("pack/") ? new Uint8Array(await r.arrayBuffer()) : await r.text() });
  }
  try {
    // The example's assets, those the starter does not carry already.
    for (const f of await exampleFiles(starter.name)) if (!files.some((g) => g.path === f.path)) files.push(f);
  } catch { /* the program says what it could not read */ }
  return files;
}

const files = (() => {
  const strip = $("file-tabs");
  const pane = $("editors");
  const code = document.querySelector(".pg-code");
  const saveStatus = $("save-status");
  const viewPane = $("view-pane");
  const vramPane = $("vram-pane");
  const empty = $("pg-empty");

  const editors = new Map(); // path to its GoEditor
  let tabs = [];             // the paths open, in the order of their tabs
  let active = null;         // the path shown
  let api = null;            // the completion index, shared by the editors
  let onRun = () => {};

  // The reference, fetched once for every editor: each package's names,
  // by import path.
  (async () => {
    try {
      const res = await fetch(versioned("api.json"));
      if (!res.ok) return;
      const packages = new Map();
      for (const [path, entries] of Object.entries(await res.json())) {
        packages.set(path, { entries, byName: new Map(entries.map((e) => [e.name, e])) });
      }
      api = { packages };
      for (const ed of editors.values()) {
        ed.api = api;
        ed.render();
      }
    } catch { /* no completion */ }
  })();

  function project() { return projects.current; }

  // kindOf says what opens path: an editor, the scene editor, the sprite
  // editor for a picture under assets/, the map of video memory, or the
  // viewer; LAYOUT is the layout each opens in.
  function kindOf(path) {
    if (path === "vram") return "vram";
    if (/^[^/]+\.go$/.test(path)) return "go";
    if (/^scenes\/[^/]+\.json$/.test(path)) return "scene";
    if (/^assets\/[^/]+\.png$/i.test(path)) return "sprite";
    return "view";
  }
  const LAYOUT = { go: "code", scene: "scenes", sprite: "assets", view: "assets", vram: "assets" };
  const last = { code: null, scenes: null, assets: null }; // what each layout showed last

  function editorFor(path) {
    let ed = editors.get(path);
    if (ed) return ed;
    const root = document.createElement("div");
    root.className = "editor-pane";
    root.hidden = true;
    pane.append(root);
    ed = new GoEditor(root, {
      label: path,
      onRun: () => onRun(),
      onChange: (text) => project()?.set(path, text, "editor"),
    });
    ed.pane = root;
    ed.api = api;
    ed.value = project().text(path) ?? "";
    editors.set(path, ed);
    return ed;
  }

  function dispose(path) {
    const ed = editors.get(path);
    if (!ed) return;
    ed.pane.remove();
    editors.delete(path);
  }

  function hidePanes() {
    for (const other of editors.values()) other.pane.hidden = true;
    sceneTab.hide();
    inspect.hide();
    spriteEditor.hide();
    viewPane.hidden = true;
    vramPane.hidden = true;
    empty.hidden = true;
  }

  // show opens path in the middle, in its layout, with a tab if it is a
  // Go file without one yet.
  function show(path) {
    const p = project();
    if (!p) return;
    if (path !== "vram" && !p.has(path)) return;
    const kind = kindOf(path);
    if (kind === "go" && !tabs.includes(path)) tabs.push(path);
    hidePanes();
    // The layout first, so that what opens is measured where it shows.
    layout.set(LAYOUT[kind]);
    showInspector(kind, path);
    if (kind === "scene") {
      sceneTab.pick(path);
      sceneTab.show();
    } else if (kind === "sprite") {
      spriteEditor.show(path);
    } else if (kind === "go") {
      editorFor(path).pane.hidden = false;
    } else if (kind === "vram") {
      inspect.showVRAM();
    } else {
      viewPane.hidden = false;
      inspect.show(path);
    }
    active = path;
    last[LAYOUT[kind]] = path;
    render();
    sidebar.mark(path);
  }

  // showInspector shows in the right column what the middle needs: the
  // scene editor's inspector, or an asset's name, what it is and its
  // converter's options, the sprite editor's palette, or what is under
  // the pointer on the map of video memory.
  function showInspector(kind, path = null) {
    $("insp-scene").hidden = kind !== "scene";
    const asset = kind === "sprite" || kind === "view" || kind === "vram";
    $("insp-asset").hidden = !asset;
    $("insp-view").hidden = kind !== "view";
    $("insp-sprite").hidden = kind !== "sprite";
    $("insp-vram").hidden = kind !== "vram";
    if (!asset) return;
    $("insp-asset-title").textContent = kind === "vram" ? "Video memory" : path.replace(/^.*\//, "");
    $("insp-asset-title").title = kind === "vram" ? "" : path;
    const icon = $("insp-asset-icon");
    icon.className = `insp-icon tree-icon icon-${kind === "vram" ? "vram" : sidebar.iconFor(path)}`;
    icon.innerHTML = kind === "vram" ? '<svg class="i"><use href="#i-vram"/></svg>' : sidebar.icon(path);
  }

  // showEmpty fills the middle when a layout has nothing to show, with
  // what would give it something: with no project, a project.
  function showEmpty(mode) {
    hidePanes();
    const text = $("pg-empty-text");
    const actions = $("pg-empty-actions");
    actions.replaceChildren();
    const button = (label, fn, primary = false) => {
      const b = document.createElement("button");
      b.type = "button";
      b.textContent = label;
      if (primary) b.className = "primary";
      b.addEventListener("click", fn);
      actions.append(b);
    };
    if (!project()) {
      text.textContent = "No project yet. A project is a program and its files, kept in this browser. Start one from an example, or bring one in from this computer.";
      button("New project…", () => newProject(), true);
      button("Open folder…", () => openFolder());
      button("Import zip…", () => picker.click());
    } else if (mode === "scenes") {
      text.textContent = "No scene yet. A scene places models, sprites and lights in front of a camera; a room is a map of cells with heights. Either is cooked into a file the program loads with package scene.";
      button("New scene", async () => { const path = await sceneTab.create(); if (path) show(path); }, true);
      button("New room", async () => { const path = await sceneTab.createRoom(); if (path) show(path); });
    } else if (mode === "assets") {
      text.textContent = "Nothing on the disc yet. Pictures, models and sounds dropped on the page go under assets/, converted for the console by a line each.";
      button("New sprite", () => newSprite(), true);
      button("Add files…", () => $("asset-files").click());
    } else {
      text.textContent = "Open a Go file from the left.";
    }
    empty.hidden = false;
    active = null;
    layout.set(mode);
    showInspector(null);
    render();
    sidebar.mark(null);
  }

  // openLayout goes to a layout, showing what it showed last, or what
  // there is to show in it.
  function openLayout(mode) {
    const p = project();
    if (!p) return;
    const fits = (path) => !!path && (path === "vram" || p.has(path)) && LAYOUT[kindOf(path)] === mode;
    let path = last[mode];
    if (!fits(path)) {
      if (mode === "code") path = tabs[0] ?? "main.go";
      else if (mode === "scenes") path = sceneTab.current();
      else path = p.onDisc().length ? "vram" : null;
    }
    if (fits(path)) show(path);
    else showEmpty(mode);
  }

  // close takes a Go file's tab away, showing its neighbor when it was
  // shown.
  function close(path) {
    const i = tabs.indexOf(path);
    if (i < 0) return;
    tabs.splice(i, 1);
    if (active !== path) {
      render();
      return;
    }
    const next = tabs[i] ?? tabs[i - 1];
    if (next) {
      show(next);
      return;
    }
    last.code = null;
    showEmpty("code");
  }

  // label is a tab's text: the file's own name.
  const label = (path) => path.replace(/^.*\//, "");

  function render() {
    strip.replaceChildren(...tabs.map((path) => {
      const tab = document.createElement("button");
      tab.type = "button";
      tab.role = "tab";
      tab.className = "tab-go";
      tab.textContent = label(path);
      tab.title = path;
      tab.setAttribute("aria-selected", String(path === active));
      tab.tabIndex = path === active ? 0 : -1;
      tab.addEventListener("click", () => {
        show(path);
        editors.get(path)?.focus();
      });
      tab.addEventListener("auxclick", (e) => { if (e.button === 1) close(path); });
      tab.addEventListener("contextmenu", (e) => {
        e.preventDefault();
        show(path);
        tabMenu(path, e);
      });
      tab.addEventListener("keydown", (e) => {
        if (e.key === "Delete" || e.key === "Backspace") {
          close(path);
          strip.querySelector('[aria-selected="true"]')?.focus();
          return;
        }
        if (e.key !== "ArrowLeft" && e.key !== "ArrowRight") return;
        const i = tabs.indexOf(path) + (e.key === "ArrowLeft" ? -1 : 1);
        show(tabs[(i + tabs.length) % tabs.length]);
        strip.querySelector('[aria-selected="true"]')?.focus();
      });
      const x = document.createElement("button");
      x.type = "button";
      x.className = "tab-close";
      x.setAttribute("aria-label", `Close ${label(path)}`);
      x.title = "Close";
      x.textContent = "×";
      x.tabIndex = -1;
      x.addEventListener("click", (e) => {
        e.stopPropagation();
        close(path);
      });
      const wrap = document.createElement("span");
      wrap.className = "file-tab";
      wrap.append(tab, x);
      return wrap;
    }));
    // The strip scrolls to the tab shown when there are more than fit.
    strip.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: "nearest", inline: "nearest" });
  }

  // tabMenu is a tab's right-click menu: closing, and what the sidebar's
  // menu offers for a Go file.
  function tabMenu(path, at) {
    const isMain = path === "main.go";
    popupMenu([
      ["Close", "Close this tab; the file stays in the project", () => close(path)],
      ["Close others", "Close every other tab", () => { for (const t of tabs.slice()) if (t !== path) close(t); }, tabs.length < 2],
      ["Rename", isMain ? "main.go is where the program starts, and keeps its name" : "Give the file another name", () => renameFile(path), isMain],
      ["Download", "Save this file to this computer", () => download(path, project().get(path))],
      ["Remove", isMain ? "main.go is where the program starts; a project always has one" : "Take the file out of the project", () => sidebar.remove(path), isMain],
    ], at);
  }

  // popupMenu shows items, [label, title, fn, disabled], under an element
  // or at a pointer event.
  function popupMenu(items, at) {
    closeMenu();
    const menu = document.createElement("div");
    menu.className = "pg-popup";
    menu.role = "menu";
    for (const [label, title, fn, disabled] of items) {
      const b = document.createElement("button");
      b.type = "button";
      b.role = "menuitem";
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
    menu.style.top = `${Math.max(4, Math.min(r.bottom + 4, innerHeight - menu.offsetHeight - 8))}px`;
    menu.querySelector("button:not(:disabled)")?.focus();
    popup = menu;
    setTimeout(() => addEventListener("pointerdown", onOutside), 0);
    addEventListener("keydown", onEscape);
  }
  let popup = null;
  function onOutside(e) { if (popup && !popup.contains(e.target)) closeMenu(); }
  function onEscape(e) { if (e.key === "Escape") closeMenu(); }
  function closeMenu() {
    popup?.remove();
    popup = null;
    removeEventListener("pointerdown", onOutside);
    removeEventListener("keydown", onEscape);
  }

  // goName cleans up a name for a Go file beside main.go, and goProblem
  // says what is wrong with one, if anything; a name taken by a file other
  // than except is.
  function goName(name) {
    let n = (name ?? "").trim().replace(/^.*[\\/]/, "");
    if (n && !n.endsWith(".go")) n += ".go";
    return n;
  }
  function goProblem(name, except = null) {
    const n = goName(name);
    if (!/^[A-Za-z0-9_][A-Za-z0-9_.-]*\.go$/.test(n)) return "A file's name is letters, digits, underscores, dots and dashes, ending in .go.";
    if (/_test\.go$/.test(n)) return "A _test.go file is not part of the program.";
    if (n !== except && project()?.has(n)) return `There is already a ${n}.`;
    return "";
  }

  // newSprite makes a picture under assets/ and opens it in the sprite
  // editor.
  async function newSprite() {
    const path = await spriteEditor.create();
    if (path) show(path);
  }

  async function newFile() {
    const p = project();
    if (!p) return;
    const answer = await dialogs.ask({ title: "New Go file", text: "A file beside main.go, in the same package.", value: "game.go", ok: "Create", check: (v) => goProblem(v, goName(v)) });
    if (!answer) return;
    const name = goName(answer);
    if (p.has(name)) {
      show(name);
      return;
    }
    p.set(name, "package main\n", "files");
    show(name);
    editors.get(name)?.goTo(2, 1);
  }

  async function renameFile(path) {
    const p = project();
    const answer = await dialogs.ask({ title: `Rename ${path}`, value: path, ok: "Rename", check: (v) => goProblem(v, path) });
    if (!answer) return;
    const name = goName(answer);
    if (name !== path) p.rename(path, name);
  }

  // What the project does to the files shown.
  projectEvents.addEventListener("open", ({ detail: { project: p } }) => {
    for (const path of [...editors.keys()]) dispose(path);
    tabs = [];
    active = null;
    last.code = last.scenes = last.assets = null;
    if (!p) {
      showEmpty("code");
      return;
    }
    // A project from before scenes/ had its one scene at the root.
    if (p.has("scene.json") && !p.has("scenes/scene.json")) {
      const line = convert.lineForSource("scene.json");
      if (line) {
        const rows = p.text(line.file).split("\n");
        if (rows[line.index] === line.text) {
          rows[line.index] = line.text.replace(/(\s)scene\.json(\s*)$/, "$1scenes/scene.json$2");
          p.set(line.file, rows.join("\n"), "generate");
        }
      }
      p.rename("scene.json", "scenes/scene.json");
    }
    // A project from before assets/ kept its sources under art/ and its
    // disc files under data/: they move, with the lines and the scenes
    // that name them.
    const old = [...p.files.keys()].filter((f) => f.startsWith("data/") || f.startsWith("art/"));
    if (old.length) {
      for (const f of old) p.rename(f, "assets/" + f.replace(/^[^/]+\//, ""));
      for (const g of p.goFiles()) {
        const text = p.text(g);
        const next = text.replace(/\b(?:data|art)\//g, "assets/").replace(/discgen data\b/g, "discgen assets").replace(/mkdir -p data\b/g, "mkdir -p assets").replace(/--textures data\b/g, "--textures assets").replace(/"DATA\//g, '"ASSETS/');
        if (next !== text) p.set(g, next, "generate");
      }
      for (const sc of p.sceneFiles()) {
        const text = p.text(sc);
        const next = text.replace(/"DATA\//g, '"ASSETS/');
        if (next !== text) p.set(sc, next, "scene");
      }
    }
    if (!p.has("main.go")) p.set("main.go", "package main\n\nfunc main() {\n}\n", "files");
    show("main.go");
  });
  projectEvents.addEventListener("files", ({ detail: { project: p, renamed } }) => {
    if (p !== project()) return;
    if (renamed) {
      const ed = editors.get(renamed.from);
      if (ed) {
        editors.delete(renamed.from);
        editors.set(renamed.to, ed);
        ed.ta.setAttribute("aria-label", renamed.to);
        ed.options.onChange = (text) => project()?.set(renamed.to, text, "editor");
      }
      tabs = tabs.map((t) => (t === renamed.from ? renamed.to : t));
      if (active === renamed.from) active = renamed.to;
    }
    for (const path of [...editors.keys()]) if (!p.has(path)) dispose(path);
    // Tabs of files gone from the project close; the one shown gives way
    // to its neighbor, or to what its layout has left to show.
    if (renamed) for (const mode of Object.keys(last)) if (last[mode] === renamed.from) last[mode] = renamed.to;
    const gone = tabs.filter((t) => !p.has(t));
    for (const t of gone) if (t !== active) tabs.splice(tabs.indexOf(t), 1);
    if (active && active !== "vram" && !p.has(active)) {
      if (kindOf(active) === "go") close(active);
      else {
        const mode = LAYOUT[kindOf(active)];
        last[mode] = null;
        openLayout(mode);
      }
    } else render();
  });
  projectEvents.addEventListener("change", ({ detail: { project: p, path, source } }) => {
    if (p !== project() || source === "editor") return;
    const ed = editors.get(path);
    if (!ed) return;
    const text = p.text(path);
    // Formatting, and a generate line changed, are edits, which undo takes
    // back; anything else is a new text.
    if (source === "format" || source === "generate") ed.replaceText(text);
    else ed.value = text;
  });

  // Saving, as it happens.
  let savedTimer = 0;
  projectEvents.addEventListener("saved", ({ detail: { kept, why } }) => {
    clearTimeout(savedTimer);
    if (!kept) {
      saveStatus.textContent = `Not kept: ${why || "this browser keeps no files, so the project lasts as long as the page"}.`;
      return;
    }
    saveStatus.textContent = "Saved";
    savedTimer = setTimeout(() => { saveStatus.textContent = ""; }, 1500);
  });

  // download saves bytes to this computer under name.
  function download(name, bytes, type = "application/octet-stream") {
    const url = URL.createObjectURL(bytes instanceof Blob ? bytes : new Blob([bytes], { type }));
    const a = document.createElement("a");
    a.href = url;
    a.download = name;
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }

  // Files dropped on the panel join the project, rather than being opened
  // by the browser in place of the page.
  const carriesFiles = (e) => [...(e.dataTransfer?.types ?? [])].includes("Files");
  for (const target of [code, document.querySelector(".pg-bar")]) {
    target.addEventListener("dragover", (e) => {
      if (!carriesFiles(e)) return;
      e.preventDefault();
      e.dataTransfer.dropEffect = "copy";
      target.classList.add("dropping");
    });
    target.addEventListener("dragleave", (e) => {
      if (!target.contains(e.relatedTarget)) target.classList.remove("dropping");
    });
    target.addEventListener("drop", async (e) => {
      if (!carriesFiles(e)) return;
      e.preventDefault();
      target.classList.remove("dropping");
      // A folder dropped is a project of its own; files are added here.
      const entries = [...(e.dataTransfer.items ?? [])].map((it) => it.webkitGetAsEntry?.()).filter(Boolean);
      const dirs = entries.filter((en) => en.isDirectory);
      if (dirs.length === 1 && entries.length === 1) {
        await importFiles(dirs[0].name, await readEntry(dirs[0], ""));
        return;
      }
      takeIn(e.dataTransfer.files);
    });
  }

  // readEntry reads a dropped directory entry and everything under it,
  // as {path, data} with paths under prefix.
  async function readEntry(entry, prefix) {
    if (entry.isFile) {
      const file = await new Promise((ok, fail) => entry.file(ok, fail));
      return [{ path: prefix + entry.name, data: new Uint8Array(await file.arrayBuffer()) }];
    }
    const reader = entry.createReader();
    const out = [];
    for (;;) {
      const batch = await new Promise((ok, fail) => reader.readEntries(ok, fail));
      if (!batch.length) break;
      for (const en of batch) out.push(...await readEntry(en, prefix + entry.name + "/"));
    }
    return out;
  }

  // openFolder asks for a folder on this computer and makes it a project:
  // through the directory picker where the browser has one, and
  // otherwise through the file input that takes a directory.
  async function openFolder() {
    menu.open = false;
    if (!window.showDirectoryPicker) {
      folderPicker.click();
      return;
    }
    let dir;
    try {
      dir = await window.showDirectoryPicker({ mode: "read" });
    } catch (e) {
      if (e.name === "AbortError") return;
      folderPicker.click();
      return;
    }
    await importFiles(dir.name, await readHandle(dir, ""));
  }

  // readHandle reads a directory handle and everything under it, as
  // readEntry does, skipping hidden entries and what a build left.
  async function readHandle(dir, prefix) {
    const out = [];
    for await (const [name, handle] of dir.entries()) {
      if (name.startsWith(".") || name === "node_modules" || name === "out") continue;
      if (handle.kind === "file") {
        const file = await handle.getFile();
        out.push({ path: prefix + name, data: new Uint8Array(await file.arrayBuffer()) });
      } else {
        out.push(...await readHandle(handle, prefix + name + "/"));
      }
    }
    return out;
  }

  // importFiles makes a project named name from files laid out as a
  // project is, however deep they were, and opens it.
  async function importFiles(name, files) {
    try {
      const entries = projectFiles(files);
      if (!entries.length) throw new Error("it holds no Go files and no assets/ directory");
      const p = await projects.create(await projects.uniqueName(name), entries);
      await projects.open(p);
    } catch (e) {
      await dialogs.tell({ title: `Could not open ${name}`, text: sentence(e.message) });
    }
  }

  // takeIn adds files from this computer: a zip becomes a project of its
  // own and is opened, Go files go beside main.go, and the rest under
  // assets/, or under the archive's directory given as into, except
  // pictures, models and sounds, which go under assets/ to be converted.
  // Files already in the project are replaced once that is confirmed.
  async function takeIn(fileList, into = "assets") {
    const p = project();
    const added = [];
    for (const file of fileList) {
      const name = file.name.replace(/^.*[\\/]/, "").replace(/\s+/g, "_");
      if (!name) continue;
      const bytes = new Uint8Array(await file.arrayBuffer());
      if (name.toLowerCase().endsWith(".zip")) {
        await importZip(name, bytes);
        continue;
      }
      const path = name.endsWith(".go") ? name : convert.isSource(name) ? "assets/" + name : `${into}/${name}`;
      added.push({ path, data: bytes });
    }
    if (!added.length) return;
    if (!p) {
      await dialogs.tell({ title: "No project to add them to", text: "Make a project with New project first, or drop a zip or a folder to bring one in." });
      return;
    }
    const replaced = added.filter((f) => p.has(f.path)).map((f) => f.path);
    if (replaced.length && !(await confirmReplace(replaced))) return;
    p.setAll(added, "files");
    show((added.find((f) => f.path.endsWith(".go")) ?? added[0]).path);
    const art = added.filter((f) => convert.isSource(f.path)).map((f) => f.path);
    if (art.length) convert.added(art);
    // Files put straight into an archive are packed, with its line added
    // when it is new.
    if (into.startsWith("pack/") && added.some((f) => f.path.startsWith(into + "/"))) {
      convert.ensureLine("mkarc", ["-o", convert.archiveOf(into.slice(5)), into], into);
      convert.packChanged(into.slice(5));
    }
  }

  // importZip makes a project from a zip, as Download writes one or as a
  // project's folder was zipped, named after the zip.
  async function importZip(name, bytes) {
    let files;
    try {
      files = await zip.read(bytes);
    } catch (e) {
      await dialogs.tell({ title: `Could not import ${name}`, text: sentence(e.message) });
      return;
    }
    await importFiles(name.replace(/\.zip$/i, ""), files);
  }

  // The project menu: the projects kept in this browser, the ways to start
  // or bring in another, and what can be done with this one, the two steps
  // that lose work last, apart and in red.
  const menu = $("project-menu");
  const nameEl = $("project-name");
  const list = $("project-list");

  // With no project the menu offers only the ways to start or bring one
  // in: this project's group goes until there is one.
  function setName() {
    const p = project();
    nameEl.textContent = p?.name ?? "No project";
    document.title = p ? `${p.name} - psxsdk playground` : "psxsdk playground";
    $("project-this").hidden = !p;
    // Start over goes back to the example the project was started from,
    // which a project brought in, or made before projects kept it, has
    // not.
    const over = $("project-start-over");
    over.disabled = !p?.starter;
    over.title = p?.starter ? `Replace this project's files with the ${p.starter} example's, as New project made them` : "This project was not started from one of the examples here, so there is nothing to start it over from";
  }
  projectEvents.addEventListener("open", setName);
  projectEvents.addEventListener("name", setName);

  function when(ms) {
    const d = new Date(ms);
    const today = new Date();
    const sameDay = d.toDateString() === today.toDateString();
    return sameDay ? d.toLocaleTimeString([], { hour: "numeric", minute: "2-digit" }) : d.toLocaleDateString([], { month: "short", day: "numeric", year: d.getFullYear() === today.getFullYear() ? undefined : "numeric" });
  }

  async function renderList() {
    const metas = await projects.list();
    const current = project()?.id;
    $("project-kept").hidden = !metas.length;
    list.replaceChildren(...metas.map((m) => {
      const li = document.createElement("li");
      const b = document.createElement("button");
      b.type = "button";
      b.className = "pg-item";
      b.setAttribute("role", "menuitemradio");
      b.setAttribute("aria-checked", String(m.id === current));
      b.innerHTML = m.id === current ? '<svg class="i"><use href="#i-check"/></svg>' : '<span class="pg-item-blank"></span>';
      const name = document.createElement("span");
      name.className = "pg-item-name";
      name.textContent = m.name;
      const meta = document.createElement("span");
      meta.className = "pg-item-meta";
      meta.textContent = `${when(m.modified)} · ${formatSize(m.size)}`;
      b.append(name, meta);
      b.addEventListener("click", async () => {
        menu.open = false;
        if (m.id === current) return;
        const p = await projects.load(m.id);
        if (p) await projects.open(p);
      });
      li.append(b);
      return li;
    }));
  }
  menu.addEventListener("toggle", () => { if (menu.open) renderList(); });
  addEventListener("pointerdown", (e) => { if (menu.open && !menu.contains(e.target)) menu.open = false; });
  addEventListener("keydown", (e) => { if (e.key === "Escape" && menu.open) menu.open = false; });

  // New project: a dialog with the name filled in and a card for each
  // example a project can start from, its picture and what it is, with
  // the ways to bring a project in beside them.
  const npDialog = $("new-project");
  const npName = $("np-name");
  const npCards = $("np-starters");
  const npError = $("np-error");
  const npCreate = $("np-create");
  let npChoice = ""; // the first, until one is picked

  function chooseStarter(name, focus = false) {
    npChoice = name;
    for (const card of npCards.children) {
      const on = card.dataset.starter === name;
      card.setAttribute("aria-checked", String(on));
      card.tabIndex = on ? 0 : -1;
      if (on && focus) card.focus();
    }
  }

  async function newProject() {
    menu.open = false;
    let list;
    try {
      list = await starterList();
    } catch (e) {
      await dialogs.tell({ title: "Could not list the examples to start from", text: sentence(e.message) });
      return;
    }
    npCards.replaceChildren(...list.map((s) => {
      const card = document.createElement("button");
      card.type = "button";
      card.className = "np-card";
      card.setAttribute("role", "radio");
      card.dataset.starter = s.name;
      card.title = s.description ?? ""; // the whole of it, which the card cuts short
      if (s.picture) {
        const img = document.createElement("img");
        img.src = versioned(s.picture);
        img.alt = "";
        card.append(img);
      } else {
        const blank = document.createElement("span");
        blank.className = "np-blank";
        card.append(blank);
      }
      const name = document.createElement("b");
      name.textContent = s.name;
      const what = document.createElement("span");
      what.textContent = s.description ?? "";
      card.append(name, what);
      card.addEventListener("click", () => chooseStarter(s.name));
      card.addEventListener("dblclick", () => npDialog.querySelector("form").requestSubmit());
      return card;
    }));
    if (!list.some((s) => s.name === npChoice)) npChoice = list[0]?.name ?? "";
    chooseStarter(npChoice);
    npError.textContent = "";
    npName.value = await projects.uniqueName("Untitled");
    npCreate.disabled = false;
    npCreate.textContent = "Create";
    npDialog.showModal();
    npName.select();
  }
  npCards.addEventListener("keydown", (e) => {
    const step = { ArrowLeft: -1, ArrowUp: -1, ArrowRight: 1, ArrowDown: 1 }[e.key];
    if (!step) return;
    e.preventDefault();
    const cards = [...npCards.children];
    const i = cards.findIndex((c) => c.dataset.starter === npChoice);
    chooseStarter(cards[(i + step + cards.length) % cards.length].dataset.starter, true);
  });
  $("np-cancel").addEventListener("click", () => npDialog.close());
  $("np-folder").addEventListener("click", () => {
    npDialog.close();
    openFolder();
  });
  $("np-zip").addEventListener("click", () => {
    npDialog.close();
    picker.click();
  });
  $("np-form").addEventListener("submit", async (e) => {
    e.preventDefault();
    const name = npName.value.trim() || "Untitled";
    npCreate.disabled = true;
    npCreate.textContent = "Making…";
    npError.textContent = "";
    try {
      const p = await projects.create(name, await startingFiles(npChoice), npChoice);
      npDialog.close();
      await projects.open(p);
      // A project that starts with a scene opens on it.
      const scene = p.sceneFiles()[0];
      if (scene) show(scene);
    } catch (err) {
      npError.textContent = `Could not make the project: ${sentence(err.message)}`;
    } finally {
      npCreate.disabled = false;
      npCreate.textContent = "Create";
    }
  });

  async function duplicateProject() {
    menu.open = false;
    const p = project();
    const name = await dialogs.ask({ title: `Duplicate ${p.name}`, text: "A copy of the project, kept beside it in this browser.", value: await projects.uniqueName(`${p.name} copy`), ok: "Duplicate" });
    if (!name) return;
    const copy = await projects.create(name, p.list().map((f) => ({ path: f.path, data: f.data.slice() })), p.starter);
    await projects.open(copy);
  }

  // Renaming happens where the name is: the menu's button gives way to a
  // field, which Enter or leaving it keeps and Escape lets go.
  const renameInput = $("project-rename-input");
  function renameProject() {
    menu.open = false;
    const p = project();
    if (!p) return;
    menu.hidden = true;
    renameInput.hidden = false;
    renameInput.value = p.name;
    renameInput.style.width = `${Math.min(32, Math.max(10, p.name.length + 3))}ch`;
    renameInput.focus();
    renameInput.select();
  }
  function endRename(keep) {
    if (renameInput.hidden) return;
    const name = renameInput.value.trim();
    renameInput.hidden = true;
    menu.hidden = false;
    if (keep && name && name !== project()?.name) project().setName(name);
  }
  renameInput.addEventListener("keydown", (e) => {
    if (e.key === "Enter") {
      e.preventDefault();
      endRename(true);
    } else if (e.key === "Escape") {
      e.preventDefault();
      endRename(false);
    }
  });
  renameInput.addEventListener("blur", () => endRename(true));
  menu.querySelector("summary").addEventListener("dblclick", (e) => {
    e.preventDefault();
    renameProject();
  });

  async function deleteProject() {
    menu.open = false;
    const p = project();
    const sure = await dialogs.confirm({
      title: `Delete ${p.name}?`,
      text: "It is kept only in this browser, so it cannot be brought back. Download it first to keep a copy.",
      ok: "Delete",
      danger: true,
      extra: ["Download zip", downloadProject],
    });
    if (!sure) return;
    await projects.remove(p.id);
    const metas = (await projects.list()).filter((m) => m.id !== p.id);
    const next = metas.length ? await projects.load(metas[0].id) : null;
    if (next) await projects.open(next);
    else await projects.close();
  }

  async function downloadProject() {
    menu.open = false;
    const p = project();
    await projects.flush();
    const dir = p.name.replace(/[\\/:*?"<>|]+/g, "-").trim() || "project";
    const blob = await zip.write(p.list().map((f) => ({ path: `${dir}/${f.path}`, data: f.data })));
    download(`${dir}.zip`, blob);
  }

  // startOver puts back the files of the example the project was started
  // from, as New project made them.
  async function startOver() {
    menu.open = false;
    const p = project();
    const sure = await dialogs.confirm({
      title: `Start ${p.name} over?`,
      text: `Its files are replaced with the ${p.starter} example's, as New project made them, and its assets. Download it first to keep a copy.`,
      ok: "Start over",
      danger: true,
      extra: ["Download zip", downloadProject],
    });
    if (!sure) return;
    try {
      const fresh = await startingFiles(p.starter);
      p.deleteAll([...p.files.keys()].filter((path) => !fresh.some((f) => f.path === path)));
      p.setAll(fresh, "files");
      show("main.go");
      editors.get("main.go")?.focus();
    } catch (e) {
      await dialogs.tell({ title: `Could not load the ${p.starter} example`, text: sentence(e.message) });
    }
  }

  const picker = $("project-files");
  const folderPicker = $("project-folder");
  $("project-new").addEventListener("click", newProject);
  $("project-duplicate").addEventListener("click", duplicateProject);
  $("project-rename").addEventListener("click", renameProject);
  $("project-download").addEventListener("click", downloadProject);
  $("project-open-folder").addEventListener("click", openFolder);
  $("project-import").addEventListener("click", () => { menu.open = false; picker.click(); });
  $("project-start-over").addEventListener("click", startOver);
  $("project-delete").addEventListener("click", deleteProject);
  picker.addEventListener("change", async () => {
    await takeIn(picker.files);
    picker.value = "";
  });
  // The directory input gives each file its path under the folder
  // picked, the folder's name first.
  folderPicker.addEventListener("change", async () => {
    const list = [...folderPicker.files];
    folderPicker.value = "";
    if (!list.length) return;
    const name = (list[0].webkitRelativePath || list[0].name).split("/")[0];
    const files = [];
    for (const f of list) {
      const path = f.webkitRelativePath || f.name;
      if (path.split("/").some((p) => p.startsWith("."))) continue;
      files.push({ path, data: new Uint8Array(await f.arrayBuffer()) });
    }
    await importFiles(name, files);
  });

  return {
    // active is what is shown and its path, with its editor for a Go file.
    active() { return active ? { path: active, editor: editors.get(active) } : null; },
    tabs() { return tabs.slice(); },
    popupMenu,
    newFile,
    newSprite,
    newProject,
    renameFile,
    close,
    openLayout,
    kindOf,
    // open shows path, at a line and column if given.
    open(path, line, col) {
      show(path);
      const ed = editors.get(path);
      if (line) ed?.goTo(line, col);
      else ed?.focus();
    },
    // goTexts is the program's Go files as {path, text}.
    goTexts() {
      const p = project();
      return p ? p.goFiles().map((path) => ({ path, text: p.text(path) })) : [];
    },
    // markErrors marks each file's lines with the errors against it, a
    // list of {file, line, message}, and clears the rest.
    markErrors(errors) {
      const byFile = new Map();
      for (const e of errors) {
        if (!byFile.has(e.file)) byFile.set(e.file, []);
        byFile.get(e.file).push(e);
      }
      for (const [path, ed] of editors) ed.markErrors(byFile.get(path) ?? []);
      // An error in a file not open yet is shown when the file is.
      for (const [path, list] of byFile) if (!editors.has(path) && project()?.has(path)) editorFor(path).markErrors(list);
    },
    onRun(fn) { onRun = fn; },
    takeIn,
    download,
  };
})();

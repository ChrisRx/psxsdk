// What the main panel shows: a tab for everything opened from the sidebar
// (sidebar.js), closed with its cross. A Go file opens in an editor of its
// own, a scene in the scene editor (scene-tab.js), a TIM, a TMD, a source
// under assets/ or anything else in the viewer (inspect.js); video memory
// opens over the page from the sidebar. Also here: the project menu, where
// projects are opened, made, renamed, downloaded as a zip, imported and
// deleted; and files dropped on the panel, which join the project: a zip
// as a new project, Go files beside main.go, and anything else under
// assets/, pictures, models and sounds to be converted there.
//
// playground.js builds and formats through files.active, files.goTexts and
// files.markErrors, and starts everything with projects.start(startingFiles).
"use strict";

// startingFiles is the files a new project is made of: one of the
// examples the site lists in starters.json, the scene example unless
// another is named, with its assets.
async function startingFiles(name = "scene") {
  const res = await fetch(versioned("starters.json"));
  if (!res.ok) throw new Error(`starters.json: ${res.status} ${res.statusText}`);
  const starters = await res.json();
  const starter = starters.find((s) => s.name === name) ?? starters[0];
  if (!starter) throw new Error("the site lists no program to start from");
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

  // kindOf says what opens path: an editor, the scene editor or the viewer.
  function kindOf(path) {
    if (/^[^/]+\.go$/.test(path)) return "go";
    if (/^scenes\/[^/]+\.json$/.test(path)) return "scene";
    return "view";
  }

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

  // show opens path in the panel, with a tab if it has none yet.
  function show(path) {
    const p = project();
    if (!p) return;
    if (!p.has(path)) return;
    const kind = kindOf(path);
    if (!tabs.includes(path)) tabs.push(path);
    for (const other of editors.values()) other.pane.hidden = true;
    sceneTab.hide();
    inspect.hide();
    viewPane.hidden = kind !== "view";
    empty.hidden = true;
    if (kind === "scene") {
      sceneTab.pick(path);
      sceneTab.show();
    } else if (kind === "go") {
      editorFor(path).pane.hidden = false;
    } else {
      inspect.show(path);
    }
    active = path;
    render();
    sidebar.mark(path);
  }

  // close takes path's tab away, showing its neighbour when it was shown.
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
    for (const other of editors.values()) other.pane.hidden = true;
    sceneTab.hide();
    inspect.hide();
    viewPane.hidden = true;
    empty.hidden = false;
    active = null;
    render();
    sidebar.mark(null);
  }

  // label is a tab's text: the file's own name, a scene's stem.
  function label(path) {
    if (kindOf(path) === "scene") return path.replace(/^scenes\//, "").replace(/\.json$/, "");
    return path.replace(/^.*\//, "");
  }

  function render() {
    strip.replaceChildren(...tabs.map((path) => {
      const kind = kindOf(path);
      const tab = document.createElement("button");
      tab.type = "button";
      tab.role = "tab";
      tab.className = `tab-${kind}`;
      tab.textContent = label(path);
      tab.title = kind === "scene" ? `${path}, placed by hand` : path;
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

  // tabMenu is a tab's right-click menu: closing, and for a Go file what
  // the sidebar's menu offers.
  function tabMenu(path, at) {
    const items = [
      ["Close", "Close this tab; the file stays in the project", () => close(path)],
      ["Close others", "Close every other tab", () => { for (const t of tabs.slice()) if (t !== path) close(t); }, tabs.length < 2],
    ];
    if (kindOf(path) === "go") {
      const isMain = path === "main.go";
      items.push(
        ["Rename", isMain ? "main.go is where the program starts, and keeps its name" : "Give the file another name", () => renameFile(path), isMain],
        ["Download", "Save this file to this computer", () => download(path, project().get(path))],
        ["Remove", isMain ? "main.go is where the program starts; a project always has one" : "Take the file out of the project", () => sidebar.remove(path), isMain],
      );
    }
    popupMenu(items, at);
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

  // goName checks a name for a Go file beside main.go, returning it
  // cleaned up, or null with the reason said.
  function goName(name) {
    let n = (name ?? "").trim().replace(/^.*[\\/]/, "");
    if (!n) return null;
    if (!n.endsWith(".go")) n += ".go";
    if (!/^[A-Za-z0-9_][A-Za-z0-9_.-]*\.go$/.test(n)) {
      alert("A file's name is letters, digits, underscores, dots and dashes, ending in .go.");
      return null;
    }
    if (/_test\.go$/.test(n)) {
      alert("A _test.go file is not part of the program.");
      return null;
    }
    return n;
  }

  function newFile() {
    const p = project();
    if (!p) return;
    const name = goName(prompt("Name of the new Go file", "game.go"));
    if (!name) return;
    if (p.has(name)) {
      show(name);
      return;
    }
    p.set(name, "package main\n", "files");
    show(name);
    editors.get(name)?.goTo(2, 1);
  }

  function renameFile(path) {
    const p = project();
    const name = goName(prompt(`Rename ${path} to`, path));
    if (!name || name === path) return;
    if (p.has(name)) {
      alert(`There is already a ${name}.`);
      return;
    }
    p.rename(path, name);
  }

  // What the project does to the files shown.
  projectEvents.addEventListener("open", ({ detail: { project: p } }) => {
    for (const path of [...editors.keys()]) dispose(path);
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
    tabs = [];
    active = null;
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
    // to its neighbour.
    const gone = tabs.filter((t) => !p.has(t));
    for (const t of gone) if (t !== active) tabs.splice(tabs.indexOf(t), 1);
    if (active && !p.has(active)) close(active);
    else render();
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
    target.addEventListener("drop", (e) => {
      if (!carriesFiles(e)) return;
      e.preventDefault();
      target.classList.remove("dropping");
      takeIn(e.dataTransfer.files);
    });
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
    const replaced = added.filter((f) => p.has(f.path)).map((f) => f.path);
    if (replaced.length && !confirm(`Replace ${replaced.join(", ")}?`)) return;
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

  async function importZip(name, bytes) {
    try {
      const entries = projectFiles(await zip.read(bytes));
      if (!entries.length) throw new Error("it holds no Go files and no assets/ directory");
      const p = await projects.create(await projects.uniqueName(name.replace(/\.zip$/i, "")), entries);
      await projects.open(p);
    } catch (e) {
      alert(`Could not import ${name}: ${e.message}`);
    }
  }

  // The project menu.
  const menu = $("project-menu");
  const nameEl = $("project-name");
  const list = $("project-list");

  function setName() {
    const p = project();
    nameEl.textContent = p?.name ?? "";
    document.title = p ? `${p.name} - psxsdk playground` : "psxsdk playground";
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
    list.replaceChildren(...metas.map((m) => {
      const li = document.createElement("li");
      const b = document.createElement("button");
      b.type = "button";
      if (m.id === current) b.setAttribute("aria-current", "true");
      const name = document.createElement("span");
      name.className = "pg-project-name";
      name.textContent = m.name;
      const meta = document.createElement("span");
      meta.className = "muted";
      meta.textContent = `${when(m.modified)}, ${formatSize(m.size)}`;
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

  async function newProject(starter = "square") {
    menu.open = false;
    const name = prompt("Name of the new project", await projects.uniqueName("Untitled"));
    if (name === null) return;
    try {
      const p = await projects.create(name.trim() || "Untitled", await startingFiles(starter));
      await projects.open(p);
      // A project made from the scene example opens on its scene.
      const scene = starter === "scene" && p.sceneFiles()[0];
      if (scene) show(scene);
    } catch (e) {
      alert(`Could not make the project: ${e.message}`);
    }
  }

  async function duplicateProject() {
    menu.open = false;
    const p = project();
    const name = prompt("Name of the copy", await projects.uniqueName(`${p.name} copy`));
    if (name === null) return;
    const copy = await projects.create(name.trim() || `${p.name} copy`, p.list().map((f) => ({ path: f.path, data: f.data.slice() })));
    await projects.open(copy);
  }

  function renameProject() {
    menu.open = false;
    const p = project();
    const name = prompt("Rename the project to", p.name);
    if (name === null || !name.trim() || name.trim() === p.name) return;
    p.setName(name.trim());
  }

  async function deleteProject() {
    menu.open = false;
    const p = project();
    if (!confirm(`Delete ${p.name}? Download it first if you want to keep it.`)) return;
    await projects.remove(p.id);
    const metas = (await projects.list()).filter((m) => m.id !== p.id);
    const next = metas.length ? await projects.load(metas[0].id) : null;
    if (next) {
      await projects.open(next);
    } else {
      await projects.open(await projects.create("Untitled", await startingFiles()));
    }
  }

  async function downloadProject() {
    menu.open = false;
    const p = project();
    await projects.flush();
    const dir = p.name.replace(/[\\/:*?"<>|]+/g, "-").trim() || "project";
    const blob = await zip.write(p.list().map((f) => ({ path: `${dir}/${f.path}`, data: f.data })));
    download(`${dir}.zip`, blob);
  }

  async function startOver() {
    menu.open = false;
    const p = project();
    if (!confirm("Replace this project's files with the program the playground starts with, and its data files?")) return;
    try {
      const fresh = await startingFiles();
      p.deleteAll([...p.files.keys()].filter((path) => !fresh.some((f) => f.path === path)));
      p.setAll(fresh, "files");
      show("main.go");
      editors.get("main.go")?.focus();
    } catch (e) {
      alert(`Could not load the starting program: ${e.message}`);
    }
  }

  const picker = $("project-files");
  $("project-new").addEventListener("click", () => newProject("square"));
  $("project-new-scene").addEventListener("click", () => newProject("scene"));
  $("project-duplicate").addEventListener("click", duplicateProject);
  $("project-rename").addEventListener("click", renameProject);
  $("project-download").addEventListener("click", downloadProject);
  $("project-import").addEventListener("click", () => { menu.open = false; picker.click(); });
  $("project-start-over").addEventListener("click", startOver);
  $("project-delete").addEventListener("click", deleteProject);
  picker.addEventListener("change", async () => {
    await takeIn(picker.files);
    picker.value = "";
  });

  return {
    // active is what is shown and its path, with its editor for a Go file.
    active() { return active ? { path: active, editor: editors.get(active) } : null; },
    tabs() { return tabs.slice(); },
    popupMenu,
    newFile,
    renameFile,
    close,
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

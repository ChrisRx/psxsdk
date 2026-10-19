// The program's files on the page: the Files tab (files-tab.js), a tab
// for each Go file of the open project over the editor, with an editor
// for each that has been opened, and the Scene tab (scene-tab.js);
// the project menu in the top bar, where projects are opened, made,
// renamed, downloaded as a zip, imported and deleted; and files dropped on
// the pane, which join the project: a zip as a new project, Go files
// beside main.go, and anything else under data/.
//
// playground.js builds and formats through files.active, files.goTexts and
// files.markErrors, and starts everything with projects.start(startingFiles).
"use strict";

// startingFiles is the files a new project is made of: one of the
// examples the site lists in starters.json, the scene example unless
// another is named, its sources and its data files.
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
    // Sources under art/ and files under pack/ are bytes.
    files.push({ path, data: path.startsWith("art/") || path.startsWith("pack/") ? new Uint8Array(await r.arrayBuffer()) : await r.text() });
  }
  try {
    files.push(...await exampleFiles(starter.name));
  } catch { /* the program says what it could not read */ }
  return files;
}

const files = (() => {
  const strip = $("file-tabs");
  const pane = $("editors");
  const code = document.querySelector(".pg-code");
  const saveStatus = $("save-status");

  const editors = new Map(); // path to its GoEditor
  let active = null;         // the path shown
  let api = null;            // the completion index, shared by the editors
  let onRun = () => {};

  // The completion index, fetched once for every editor.
  (async () => {
    try {
      const res = await fetch(versioned("yaroze.json"));
      if (!res.ok) return;
      const entries = await res.json();
      api = { entries, byName: new Map(entries.map((e) => [e.name, e])) };
      for (const ed of editors.values()) {
        ed.api = api;
        ed.render();
      }
    } catch { /* no completion */ }
  })();

  function project() { return projects.current; }

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

  // show opens path's editor in the pane, the Scene tab for a scene or
  // for "scenes", or the Files tab.
  function show(path) {
    if (path.startsWith("scenes/")) {
      if (!project()?.has(path)) return;
      sceneTab.pick(path);
      path = SCENE;
    }
    if (path === SCENE && !project()?.sceneFiles().length) return;
    if (path !== FILES && path !== SCENE && !project()?.has(path)) return;
    for (const other of editors.values()) other.pane.hidden = true;
    if (path === SCENE) {
      filesTab.hide();
      sceneTab.show();
    } else if (path === FILES) {
      sceneTab.hide();
      filesTab.show();
    } else {
      sceneTab.hide();
      filesTab.hide();
      editorFor(path).pane.hidden = false;
    }
    active = path;
    render();
  }

  const SCENE = "scenes"; // the Scene tab, for whichever scene is picked
  const FILES = "files";

  function render() {
    const p = project();
    const paths = p ? [FILES, ...p.goFiles()] : [];
    if (p?.sceneFiles().length) paths.push(SCENE);
    strip.replaceChildren(...paths.map((path) => {
      const tab = document.createElement("button");
      tab.type = "button";
      tab.role = "tab";
      tab.textContent = path === SCENE ? "Scene" : path === FILES ? "Files" : path;
      if (path === SCENE) tab.title = "The scenes under scenes/, placed by hand";
      if (path === FILES) tab.title = "Every file of the project: the program, the sources under art/ and the data on the disc";
      tab.setAttribute("aria-selected", String(path === active));
      tab.tabIndex = path === active ? 0 : -1;
      tab.addEventListener("click", () => {
        show(path);
        editors.get(path)?.focus();
      });
      if (path !== FILES && path !== SCENE) {
        tab.addEventListener("contextmenu", (e) => {
          e.preventDefault();
          show(path);
          fileMenu(path, e);
        });
      }
      tab.addEventListener("keydown", (e) => {
        if (e.key !== "ArrowLeft" && e.key !== "ArrowRight") return;
        const i = paths.indexOf(path) + (e.key === "ArrowLeft" ? -1 : 1);
        const next = paths[(i + paths.length) % paths.length];
        show(next);
        strip.querySelector('[aria-selected="true"]')?.focus();
      });
      if (path === active && path !== FILES) {
        const more = document.createElement("button");
        more.type = "button";
        more.className = "file-more";
        more.setAttribute("aria-label", `Actions for ${path}`);
        more.title = "Rename, download or remove this file";
        more.textContent = "⋯";
        more.addEventListener("click", (e) => {
          e.stopPropagation();
          fileMenu(path, more);
        });
        const wrap = document.createElement("span");
        wrap.className = "file-tab";
        wrap.append(tab, more);
        return wrap;
      }
      return tab;
    }));
  }

  // fileMenu opens the actions for path under its button, or at the
  // pointer for a right click.
  function fileMenu(path, at) {
    const isMain = path === "main.go";
    const fixed = isMain || path === SCENE;
    const items = [
      ["Rename", isMain ? "main.go is where the program starts, and keeps its name" : fixed ? "The scene is scene.json, which mkscene's line names" : "Give the file another name", () => renameFile(path), fixed],
      ["Download", "Save this file to this computer", () => download(path, project().get(path))],
      ["Remove", isMain ? "main.go is where the program starts; a project always has one" : path === SCENE ? "Take the scene out of the project, with its mkscene line" : "Take the file out of the project", () => removeFile(path), isMain],
    ];
    popupMenu(items, at);
  }

  // newMenu offers what can be added: a Go file, a scene, files from
  // this computer, an example's data.
  function newMenu(at) {
    const p = project();
    const items = [
      ["New Go file", "A file beside main.go, in the same package", newFile],
      ["New scene", "A scene under scenes/, placed by hand in the Scene tab and cooked into a data file the program loads with package scene", () => { const path = sceneTab.create(); if (path) show(path); }],
      ["Add files from this computer…", "Pictures, models and sounds go under art/ to be converted; Go files beside main.go; anything else under data/", () => $("asset-files").click()],
    ];
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
    menu.style.top = `${Math.min(r.bottom + 4, innerHeight - menu.offsetHeight - 8)}px`;
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

  function removeFile(path) {
    if (path.startsWith("scenes/")) {
      filesTab.removeScene(path);
      return;
    }
    filesTab.remove(path);
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
      if (active === renamed.from) active = renamed.to;
    }
    for (const path of [...editors.keys()]) if (!p.has(path)) dispose(path);
    if (active !== FILES && !p.has(active)) show("main.go");
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
  projectEvents.addEventListener("saved", ({ detail: { kept } }) => {
    clearTimeout(savedTimer);
    if (!kept) {
      saveStatus.textContent = "Not kept: this browser keeps no files, so the project lasts as long as the page.";
      return;
    }
    saveStatus.textContent = "Saved";
    savedTimer = setTimeout(() => { saveStatus.textContent = ""; }, 1500);
  });

  $("file-add").addEventListener("click", (e) => newMenu(e.currentTarget));

  // download saves bytes to this computer under name.
  function download(name, bytes, type = "application/octet-stream") {
    const url = URL.createObjectURL(bytes instanceof Blob ? bytes : new Blob([bytes], { type }));
    const a = document.createElement("a");
    a.href = url;
    a.download = name;
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  }

  // Files dropped on the pane join the project, rather than being opened
  // by the browser in place of the page.
  const carriesFiles = (e) => [...(e.dataTransfer?.types ?? [])].includes("Files");
  code.addEventListener("dragover", (e) => {
    if (!carriesFiles(e)) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = "copy";
    code.classList.add("dropping");
  });
  code.addEventListener("dragleave", (e) => {
    if (!code.contains(e.relatedTarget)) code.classList.remove("dropping");
  });
  code.addEventListener("drop", (e) => {
    if (!carriesFiles(e)) return;
    e.preventDefault();
    code.classList.remove("dropping");
    takeIn(e.dataTransfer.files);
  });

  // takeIn adds files from this computer: a zip becomes a project of its
  // own and is opened, Go files go beside main.go, pictures, models and
  // sounds under art/ to be converted, and the rest under data/. Files
  // already in the project are replaced once that is confirmed.
  async function takeIn(fileList) {
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
      const path = name.endsWith(".go") ? name : convert.isSource(name) ? "art/" + name : "data/" + name;
      added.push({ path, data: bytes });
    }
    if (!added.length) return;
    const replaced = added.filter((f) => p.has(f.path)).map((f) => f.path);
    if (replaced.length && !confirm(`Replace ${replaced.join(", ")}?`)) return;
    p.setAll(added, "files");
    const go = added.find((f) => f.path.endsWith(".go"));
    if (go) show(go.path);
    else {
      show(FILES);
      filesTab.select(added[0].path);
    }
    const art = added.filter((f) => f.path.startsWith("art/")).map((f) => f.path);
    if (art.length) convert.added(art);
  }

  async function importZip(name, bytes) {
    try {
      const entries = projectFiles(await zip.read(bytes));
      if (!entries.length) throw new Error("it holds no Go files and no data/ directory");
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
      if (starter === "scene") show(SCENE);
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
    // active is the editor shown and its path.
    active() { return active ? { path: active, editor: editors.get(active) } : null; },
    popupMenu,
    newMenu,
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

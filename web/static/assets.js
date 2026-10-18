// The playground's data files: the files beside the program, under data/,
// that it registers with package disc and the build puts on the disc. They
// come from this computer, picked or dropped on the editor, or are copied
// from an example's data, and are kept in this browser, in IndexedDB,
// until they are removed.
//
// playground.js sends playgroundAssets() with each build, and calls
// addExampleData for the data of the program it starts with.
"use strict";

const assetFiles = new Map(); // path, such as data/CUBE.TIM, to its bytes

function playgroundAssets() {
  return [...assetFiles].map(([path, data]) => ({ path, data }));
}

// addExampleData puts the named example's data files beside the program,
// replacing any at the same paths without asking. It is set below.
let addExampleData = async (name) => {};

(() => {
  const DB = "playground";
  const STORE = "assets";
  const OPEN = "playground-assets-open";

  const panel = $("assets");
  const list = $("asset-list");
  const count = $("asset-count");
  const status = $("asset-status");
  const picker = $("asset-files");
  const examples = $("example-data");

  // The store, or null where the browser keeps none, as in some private
  // windows: the files then last as long as the page.
  let db = null;
  function openDB() {
    db ??= new Promise((resolve) => {
      try {
        const req = indexedDB.open(DB, 1);
        req.onupgradeneeded = () => req.result.createObjectStore(STORE, { keyPath: "path" });
        req.onsuccess = () => resolve(req.result);
        req.onerror = () => resolve(null);
      } catch {
        resolve(null);
      }
    });
    return db;
  }

  // keep runs fn over the store in one transaction, and resolves once it
  // is written, to the last request's result, or to null if nothing is kept.
  async function keep(mode, fn) {
    const d = await openDB();
    if (!d) return null;
    return new Promise((resolve) => {
      try {
        const tx = d.transaction(STORE, mode);
        const req = fn(tx.objectStore(STORE));
        tx.oncomplete = () => resolve(req?.result ?? null);
        tx.onerror = tx.onabort = () => resolve(null);
      } catch {
        resolve(null);
      }
    });
  }

  function say(text) {
    status.textContent = text;
  }

  function size(n) {
    if (n < 1024) return `${n} B`;
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`;
    return `${(n / 1024 / 1024).toFixed(1)} MB`;
  }

  // declaration is what a program writes to read the file at path: the
  // registration, as the examples write them, which Load reads into memory
  // it allocates.
  function declaration(path) {
    const base = path.split("/").pop();
    const dot = base.lastIndexOf(".");
    const stem = (dot > 0 ? base.slice(0, dot) : base).toLowerCase();
    const ext = dot > 0 ? base.slice(dot + 1).toUpperCase() : "";
    let name = (stem + ext).replace(/[^A-Za-z0-9_]/g, "_");
    if (!/^[A-Za-z_]/.test(name)) name = "f" + name;
    return `var ${name} = disc.Register("${path.toUpperCase()}", "${path}")\n`;
  }

  function render() {
    const paths = [...assetFiles.keys()].sort();
    count.textContent = paths.length ? `(${paths.length})` : "";
    $("clear-assets").hidden = !paths.length;
    list.replaceChildren(...paths.map((path) => {
      const data = assetFiles.get(path);
      const li = document.createElement("li");
      const name = document.createElement("code");
      name.textContent = path;
      const bytes = document.createElement("span");
      bytes.className = "muted";
      bytes.textContent = size(data.length);
      const copy = document.createElement("button");
      copy.type = "button";
      copy.className = "small-button";
      copy.textContent = "Copy declaration";
      copy.title = "Copy the Go that registers this file and reads it into memory";
      copy.addEventListener("click", async () => {
        try {
          await navigator.clipboard.writeText(declaration(path));
          say(`Copied the declaration of ${path}. The program imports go.chrisrx.dev/psx/disc, calls Load on it, and finds the file in its Data.`);
        } catch (e) {
          say(`Could not copy: ${e.message}`);
        }
      });
      const remove = document.createElement("button");
      remove.type = "button";
      remove.className = "small-button";
      remove.textContent = "Remove";
      remove.addEventListener("click", async () => {
        assetFiles.delete(path);
        render();
        await keep("readwrite", (s) => s.delete(path));
        say(`Removed ${path}.`);
      });
      li.append(name, bytes, copy, remove);
      return li;
    }));
  }

  // add puts files, a list of {path, data}, beside the program, replacing
  // those at the same paths once that is confirmed, or at once if confirmed
  // is true.
  async function add(files, confirmed = false) {
    if (!files.length) return;
    const replaced = files.filter((f) => assetFiles.has(f.path)).map((f) => f.path);
    if (!confirmed && replaced.length && !confirm(`Replace ${replaced.join(", ")}?`)) return;
    for (const { path, data } of files) assetFiles.set(path, data);
    render();
    panel.open = true;
    const kept = await keep("readwrite", (s) => {
      let last = null;
      for (const f of files) last = s.put(f);
      return last;
    });
    const what = files.length === 1 ? files[0].path : `${files.length} files`;
    say(kept === null ? `Added ${what}, for as long as this page is open: this browser keeps no files for it.` : `Added ${what}.`);
  }

  // fromComputer reads files the browser was given, picked or dropped,
  // into data/ under their own names.
  async function fromComputer(fileList) {
    const files = [];
    for (const file of fileList) {
      const name = file.name.replace(/^.*[\\/]/, "");
      if (!name) continue;
      files.push({ path: "data/" + name, data: new Uint8Array(await file.arrayBuffer()) });
    }
    await add(files);
  }

  $("add-assets").addEventListener("click", () => picker.click());
  picker.addEventListener("change", async () => {
    await fromComputer(picker.files);
    picker.value = "";
  });

  $("clear-assets").addEventListener("click", async () => {
    if (!confirm("Remove every data file?")) return;
    assetFiles.clear();
    render();
    await keep("readwrite", (s) => s.clear());
    say("Removed every data file.");
  });

  // Files dropped on the editor are added, rather than opened by the
  // browser in place of the page.
  const code = document.querySelector(".pg-code");
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
    fromComputer(e.dataTransfer.files);
  });

  // The examples' data, which the site publishes beside their discs, with
  // examples.json listing each one's files.
  let catalog = [];
  const catalogLoaded = (async () => {
    try {
      const res = await fetch("examples.json");
      if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
      catalog = (await res.json()).filter((ex) => ex.data?.length);
    } catch {
      catalog = [];
    }
    for (const ex of catalog) {
      const option = document.createElement("option");
      option.value = ex.name;
      option.textContent = `${ex.name} (${ex.data.length} file${ex.data.length > 1 ? "s" : ""})`;
      examples.append(option);
    }
    examples.disabled = !catalog.length;
  })();

  // fromExample fetches the data of the example ex, an entry of the
  // catalog, and adds it.
  async function fromExample(ex, confirmed) {
    examples.disabled = true;
    say(`Fetching ${ex.name}'s data…`);
    try {
      const files = await Promise.all(ex.data.map(async (path) => ({ path, data: await fetchBytes(`examples/${ex.name}/${path}`) })));
      await add(files, confirmed);
    } catch (e) {
      say(`Could not fetch ${ex.name}'s data: ${e.message}`);
    } finally {
      examples.disabled = false;
    }
  }

  examples.addEventListener("change", async () => {
    const ex = catalog.find((e) => e.name === examples.value);
    examples.value = "";
    if (ex) await fromExample(ex, false);
  });

  addExampleData = async (name) => {
    await catalogLoaded;
    const ex = catalog.find((e) => e.name === name);
    if (ex) await fromExample(ex, true);
  };

  // Whether the panel is open is kept in this browser too.
  try { panel.open = localStorage.getItem(OPEN) === "1"; } catch { /* closed */ }
  panel.addEventListener("toggle", () => {
    try { localStorage.setItem(OPEN, panel.open ? "1" : "0"); } catch { /* not kept */ }
  });

  (async () => {
    const kept = await keep("readonly", (s) => s.getAll());
    for (const { path, data } of kept ?? []) {
      if (!assetFiles.has(path)) assetFiles.set(path, data);
    }
    render();
  })();
})();

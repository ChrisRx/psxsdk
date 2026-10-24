// The converters in the page: a picture, a model or a sound dropped on the
// playground goes under assets/, a //go:generate line naming the converter
// that makes its data file goes into main.go, as the guide's pipeline has
// them, and the converter runs in the compiler's worker, putting the file
// beside it under assets/. The line is the record: its flags are the
// options shown for the source, and the build runs the same lines, so a
// downloaded project remakes its data with go generate.
"use strict";

const convert = (() => {
  const TOOLS = {
    png: "png2tim", bmp: "png2tim", gif: "png2tim", jpg: "png2tim", jpeg: "png2tim",
    obj: "obj2tmd",
    wav: "wav2vag", aif: "wav2vag", aiff: "wav2vag",
    mid: "smf2seq", midi: "smf2seq",
  };
  const OUTPUT = { png2tim: "TIM", obj2tmd: "TMD", wav2vag: "VAG", smf2seq: "SEQ", mkscene: "DAT", mkarc: "ARC" };
  // The flags that take no value, per tool; every other flag takes the
  // next word.
  const BOOLS = {
    png2tim: new Set(["--semi"]),
    obj2tmd: new Set(["--smooth", "--unlit", "--both"]),
    wav2vag: new Set(["-L", "--loop", "-1", "--no-loop", "-E", "--big-endian"]),
    smf2seq: new Set(["-V", "--verbose"]),
    mkvab: new Set(["--split", "-r"]),
  };
  // Sources that are read by a converter beside another, with no line of
  // their own.
  const COMPANION = /\.(mtl|def)$/i;

  const ext = (path) => path.replace(/^.*\./, "").toLowerCase();
  const toolFor = (path) => TOOLS[ext(path)] ?? null;
  const isSource = (path) => toolFor(path) !== null || COMPANION.test(path) || isSourceFile(path);

  function project() { return projects.current; }

  // outputFor is the data file a source becomes: its name in capitals, as
  // the disc holds it, cut to eight characters.
  function outputFor(path, tool) {
    const base = path.replace(/^.*\//, "").replace(/\.[^.]*$/, "");
    let stem = base.toUpperCase().replace(/[^A-Z0-9_]/g, "_").slice(0, 8);
    if (!stem) stem = "FILE";
    return `assets/${stem}.${OUTPUT[tool]}`;
  }

  // The generate lines across the project's Go files, main.go first.
  const LINE = /^\/\/go:generate[ \t]+(.+?)[ \t]*$/;
  function parseLine(text) {
    const m = text.match(LINE);
    if (!m) return null;
    const words = m[1].split(/\s+/);
    let tool = words[0];
    let args = words.slice(1);
    if (words[0] === "go" && words[1] === "run" && /^go\.chrisrx\.dev\/psx\/tools\/[\w-]+(@\S+)?$/.test(words[2] ?? "")) {
      tool = words[2].replace(/^.*\//, "").replace(/@.*$/, "");
      args = words.slice(3);
    } else if (words[0] === "go" && words[1] === "tool" && words[2]) {
      tool = words[2];
      args = words.slice(3);
    }
    return { tool, args };
  }

  function lines() {
    const p = project();
    const out = [];
    if (!p) return out;
    for (const file of p.goFiles()) {
      const text = p.text(file);
      text.split("\n").forEach((line, index) => {
        const parsed = parseLine(line);
        if (parsed) out.push({ file, index, text: line, ...parsed, ...parseArgs(parsed.tool, parsed.args) });
      });
    }
    return out;
  }

  // parseArgs splits a tool's arguments into flag values, flags set, and
  // the positional arguments, keeping the order the flags came in.
  function parseArgs(tool, args) {
    const bools = BOOLS[tool] ?? new Set();
    const values = new Map();
    const set = new Set();
    const positional = [];
    for (let i = 0; i < args.length; i++) {
      const a = args[i];
      if (a.startsWith("-") && a.length > 1 && !/^-\d/.test(a)) {
        const eq = a.indexOf("=");
        if (eq > 0) {
          values.set(a.slice(0, eq), a.slice(eq + 1));
        } else if (bools.has(a)) {
          set.add(a);
        } else {
          values.set(a, args[++i] ?? "");
        }
      } else {
        positional.push(a);
      }
    }
    return { values, set, positional };
  }

  function joinArgs({ values, set, positional }) {
    const out = [];
    for (const [k, v] of values) out.push(k, v);
    out.push(...set, ...positional);
    return out;
  }

  // lineFor finds the line that reads a source, or writes an output.
  function lineForSource(path) { return lines().find((l) => l.positional.includes(path)) ?? null; }
  function lineForOutput(path) {
    return lines().find((l) => (l.values.get("-o") ?? l.values.get("--output")) === path) ?? null;
  }

  // writeLine puts a line's text back in its file.
  function writeLine(entry, text) {
    const p = project();
    const rows = p.text(entry.file).split("\n");
    if (rows[entry.index] !== entry.text) return false; // the file moved on
    rows[entry.index] = text;
    p.set(entry.file, rows.join("\n"), "generate");
    return true;
  }

  function lineText(tool, args) {
    return `//go:generate go run go.chrisrx.dev/psx/tools/${tool} ${args.join(" ")}`;
  }

  // addLine puts a new generate line in main.go: before discgen's, which
  // must come last, else after the last one, else after the package clause.
  function addLine(tool, args) {
    const p = project();
    const text = lineText(tool, args);
    const rows = p.text("main.go").split("\n");
    const all = lines().filter((l) => l.file === "main.go");
    const discgen = all.find((l) => l.tool === "discgen");
    let at;
    if (discgen) {
      at = discgen.index;
      rows.splice(at, 0, text);
    } else if (all.length) {
      at = all.at(-1).index + 1;
      rows.splice(at, 0, text);
    } else {
      const pkg = rows.findIndex((r) => /^package\s+main\b/.test(r));
      at = pkg + 1;
      rows.splice(at, 0, "", "// The files on the disc are made from the sources beside them under assets/ by go generate.", text);
    }
    p.set("main.go", rows.join("\n"), "generate");
  }

  // Where a new picture goes in video memory: the first texture page from
  // 640 that no TIM's pixels touch, and the first palette row from 480
  // that no TIM's palette uses.
  function freePlace() {
    const tims = [];
    for (const path of project().onDisc()) {
      if (!/\.tim$/i.test(path)) continue;
      try { tims.push(preview.decodeTIM(project().get(path))); } catch { /* not one */ }
    }
    const overlaps = (b, x, y, w, h) => b && b.x < x + w && x < b.x + b.w && b.y < y + h && y < b.y + b.h;
    let at = "640,0";
    pages: for (const py of [0, 256]) {
      for (let px = 640; px < 1024; px += 64) {
        if (!tims.some((t) => overlaps(t.pixels, px, py, 64, 256))) {
          at = `${px},${py}`;
          break pages;
        }
      }
    }
    let cy = 480;
    while (cy < 512 && tims.some((t) => overlaps(t.clut, 0, cy, 1024, 1))) cy++;
    return { at, clut: `0,${Math.min(cy, 511)}` };
  }

  function defaults(tool, source) {
    const out = outputFor(source, tool);
    switch (tool) {
      case "png2tim": {
        const { at, clut } = freePlace();
        return ["--at", at, "--clut", clut, "-o", out, source];
      }
      case "obj2tmd":
        return ["--scale", "100", "--textures", "assets", "-o", out, source];
      default:
        return ["-o", out, source];
    }
  }

  // Running the converters: the worker gets the whole project and sends
  // back the files the tools made or changed.
  let running = null;
  let again = false;
  async function run() {
    if (running) {
      again = true;
      return running;
    }
    running = (async () => {
      const p = project();
      say("Converting…");
      let log = "";
      const reply = await request("convert", { files: p.list() }, (text) => {
        log += text;
        appendLog(text);
      });
      if (p !== project()) return false;
      if (reply.files?.length) p.setAll(reply.files, "convert");
      if (!reply.ok) {
        const why = reply.error ?? log.trim().split("\n").filter((l) => l).at(-1) ?? "the converter failed";
        say(`Could not convert: ${why}. The compiler's tab has the output.`);
        return false;
      }
      const made = (reply.files ?? []).map((f) => f.path);
      say(made.length ? `Made ${made.join(", ")}.` : "Nothing to remake.");
      return true;
    })().finally(() => {
      running = null;
      if (again) {
        again = false;
        run();
      }
    });
    return running;
  }

  function say(text) {
    projectEvents.dispatchEvent(new CustomEvent("status", { detail: { text } }));
  }

  // added gives each new source a line, when it has none, and converts.
  async function added(paths) {
    let any = false;
    for (const path of paths) {
      const tool = toolFor(path);
      if (!tool || lineForSource(path)) continue;
      addLine(tool, defaults(tool, path));
      any = true;
    }
    if (any || paths.some((p) => lineForSource(p) || COMPANION.test(p))) await run();
  }

  // The options shown for a source: each a flag of its line.
  const FIELDS = {
    png2tim: [
      { flag: "--at", label: "Pixels at", type: "text", hint: "x,y in video memory; a texture lies within one 64 by 256 page" },
      { flag: "--clut", label: "Palette at", type: "text", hint: "x,y in video memory, x a multiple of 16" },
      { flag: "--bits", label: "Bits", type: "select", options: [["0", "as the picture"], ["4", "4"], ["8", "8"], ["16", "16"], ["24", "24"]] },
      { flag: "--black", label: "Black", type: "select", options: [["opaque", "drawn"], ["transparent", "transparent"]] },
      { flag: "--semi", label: "Semitransparent", type: "bool", hint: "STP on every drawn color" },
    ],
    obj2tmd: [
      { flag: "--scale", label: "Scale", type: "number", hint: "coordinates are multiplied by this and rounded to integers" },
      { flag: "--smooth", label: "Smooth", type: "bool", hint: "gouraud shading with the vertex normals" },
      { flag: "--unlit", label: "Unlit", type: "bool", hint: "colors and textures as they are" },
      { flag: "--both", label: "Both sides", type: "bool", hint: "a second face back to back with each" },
    ],
    wav2vag: [
      { flag: "loop", label: "Loop", type: "select", options: [["", "as the file marks"], ["-L", "the whole sound"], ["-1", "play once"]] },
    ],
    smf2seq: [],
  };

  // renderOptions fills form with the source's options and sets info.
  function renderOptions(path, form, info) {
    form.replaceChildren();
    const tool = toolFor(path);
    if (!tool) {
      info.textContent = COMPANION.test(path) ? `${path} is read beside the model or the bank that names it.` : `${path} has no converter here.`;
      return;
    }
    const entry = lineForSource(path);
    if (!entry) {
      const b = document.createElement("button");
      b.type = "button";
      b.className = "small-button";
      b.textContent = `Convert with ${tool}`;
      b.addEventListener("click", () => added([path]));
      form.append(b);
      info.textContent = `No go:generate line in main.go names ${path} yet.`;
      return;
    }
    const out = entry.values.get("-o") ?? entry.values.get("--output") ?? "";
    for (const f of FIELDS[tool]) {
      const label = document.createElement("label");
      label.title = f.hint ?? "";
      const span = document.createElement("span");
      span.textContent = f.label;
      let input;
      if (f.type === "select") {
        input = document.createElement("select");
        for (const [v, name] of f.options) input.append(new Option(name, v));
        input.value = f.flag === "loop" ? (entry.set.has("-L") || entry.set.has("--loop") ? "-L" : entry.set.has("-1") || entry.set.has("--no-loop") ? "-1" : "") : (entry.values.get(f.flag) ?? entry.values.get(f.flag.slice(1, 3)) ?? f.options[0][0]);
      } else if (f.type === "bool") {
        input = document.createElement("input");
        input.type = "checkbox";
        input.checked = entry.set.has(f.flag);
      } else {
        input = document.createElement("input");
        input.type = f.type;
        if (f.type === "number") input.step = "any";
        input.value = entry.values.get(f.flag) ?? entry.values.get(f.flag.slice(1, 3)) ?? "";
        input.size = 10;
      }
      input.addEventListener("change", () => {
        const fresh = lineForSource(path);
        if (!fresh) return;
        if (f.flag === "loop") {
          for (const k of ["-L", "--loop", "-1", "--no-loop"]) fresh.set.delete(k);
          if (input.value) fresh.set.add(input.value);
        } else if (f.type === "bool") {
          if (input.checked) fresh.set.add(f.flag);
          else fresh.set.delete(f.flag);
        } else if (input.value === "" || (f.type === "select" && input.value === f.options[0][0] && f.flag === "--bits")) {
          fresh.values.delete(f.flag);
        } else {
          fresh.values.set(f.flag, input.value.replace(/\s+/g, ""));
        }
        if (writeLine(fresh, lineText(tool, joinArgs(fresh)))) run();
      });
      label.append(span, input);
      form.append(label);
    }
    const line = document.createElement("code");
    line.className = "pg-line";
    line.textContent = entry.text;
    form.append(line);
    info.textContent = `Makes ${out || "a file beside it"}; the line is in ${entry.file}, and the build runs it too.`;
  }

  // ensureLine adds a line for tool when none reads source yet; it
  // returns whether it added one.
  function ensureLine(tool, args, source) {
    if (lines().some((l) => l.tool === tool && l.positional.includes(source))) return false;
    addLine(tool, args);
    return true;
  }

  // Archives: the files under pack/NAME/ are packed by one mkarc line into
  // assets/NAME.ARC. A data file moves in and out; a file a converter makes
  // moves by its line's -o.
  const packDir = (name) => `pack/${name}`;
  const archiveOf = (name) => outputFor(packDir(name), "mkarc");
  function lineForPack(name) {
    return lines().find((l) => l.tool === "mkarc" && l.positional.includes(packDir(name))) ?? null;
  }
  // retarget points the line that makes path at dest instead, and says
  // whether there was one.
  function retarget(path, dest) {
    const entry = lineForOutput(path);
    if (!entry) return false;
    entry.values.set(entry.values.has("--output") ? "--output" : "-o", dest);
    return writeLine(entry, lineText(entry.tool, joinArgs(entry)));
  }
  // dropLine takes a line out of its file.
  function dropLine(entry) {
    const p = project();
    const rows = p.text(entry.file).split("\n");
    if (rows[entry.index] !== entry.text) return false;
    rows.splice(entry.index, 1);
    p.set(entry.file, rows.join("\n"), "generate");
    return true;
  }
  // packInto moves a data file into the archive called name, adding the
  // archive's line when it is the first, and packs; it returns what went
  // wrong, or "".
  function packInto(path, name) {
    const p = project();
    const dest = `${packDir(name)}/${path.replace(/^.*\//, "")}`;
    if (p.has(dest)) return `There is already a ${dest}.`;
    const key = dest.replace(/^.*\//, "").toUpperCase().replace(/\.[^.]*$/, "");
    if (p.packFiles().some((f) => f.startsWith(packDir(name) + "/") && f.replace(/^.*\//, "").toUpperCase().replace(/\.[^.]*$/, "") === key && f.replace(/^.*\./, "").toUpperCase() === dest.replace(/^.*\./, "").toUpperCase())) return `The archive already has a ${dest.replace(/^.*\//, "").toUpperCase()}.`;
    if (lineForOutput(archiveOf(name)) && !lineForPack(name)) return `${archiveOf(name)} is already made by another line.`;
    retarget(path, dest);
    p.rename(path, dest);
    ensureLine("mkarc", ["-o", archiveOf(name), packDir(name)], packDir(name));
    run();
    return "";
  }
  // takeOut moves a packed file back under assets/, and when the archive is
  // then empty takes its line and its file out too.
  function takeOut(path) {
    const p = project();
    const name = path.split("/")[1];
    const dest = `assets/${path.replace(/^.*\//, "")}`;
    if (p.has(dest)) return `There is already a ${dest}.`;
    retarget(path, dest);
    p.rename(path, dest);
    packChanged(name);
    return "";
  }
  // packChanged packs the archive again, or takes it out when its
  // directory is empty.
  function packChanged(name) {
    const p = project();
    if (p.packFiles().some((f) => f.startsWith(packDir(name) + "/"))) {
      run();
      return;
    }
    const entry = lineForPack(name);
    if (entry) dropLine(entry);
    if (p.has(archiveOf(name))) p.delete(archiveOf(name));
  }
  // readArchive reads an archive's table: [{name, method, size, packed}].
  function readArchive(data) {
    if (!data || data.length < 16) return null;
    const v = new DataView(data.buffer, data.byteOffset, data.byteLength);
    if (v.getUint32(0, true) !== 0x01435241) return null;
    const n = v.getUint32(4, true);
    const out = [];
    for (let i = 0; i < n; i++) {
      const at = 16 + 32 * i;
      if (at + 32 > data.length) return null;
      let name = "";
      for (let k = 0; k < 16 && data[at + k]; k++) name += String.fromCharCode(data[at + k]);
      out.push({ name, method: v.getUint32(at + 16, true), size: v.getUint32(at + 20, true), packed: v.getUint32(at + 24, true) });
    }
    return out;
  }

  // moveTIM moves a data file's picture or palette by its line, when a
  // png2tim line makes it, and converts again; it returns whether it did.
  function moveTIM(dataPath, kind, x, y) {
    const entry = lineForOutput(dataPath);
    if (!entry || entry.tool !== "png2tim") return false;
    entry.values.set(kind === "clut" ? "--clut" : "--at", `${x},${y}`);
    if (!writeLine(entry, lineText("png2tim", joinArgs(entry)))) return false;
    run();
    return true;
  }

  // A project opened without what its lines make, a new one from a
  // starter or one brought in without its data, is converted at once.
  projectEvents.addEventListener("open", ({ detail: { project: p } }) => {
    const missing = lines().some((l) => {
      const out = l.values.get("-o") ?? l.values.get("--output");
      return out && !p.has(out) && l.positional.some((src) => p.has(src));
    });
    if (missing) run();
  });

  return { isSource, toolFor, outputFor, added, run, renderOptions, moveTIM, ensureLine, lineForSource, lineForOutput, lines, archiveOf, lineForPack, packInto, takeOut, packChanged, readArchive };
})();

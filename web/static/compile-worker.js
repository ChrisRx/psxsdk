// The playground's build thread. It fetches the compiler, the disc tools and
// the sysroot the first time it is asked to build, keeps them, and builds
// each project it is sent with pipeline.js.
//
// The page sends {type: "build", id, files}, the project's files as
// {path, data}, its Go files at the root and its data files under assets/, or
// {type: "load", id} to fetch the compiler ahead of the first build,
// answered by {id, ok} or {id, ok: false, error}. While loading or
// building, the worker sends {id, event: "log", text} for each line of
// output, and when done {id, ok, files, elf} with prog.cue and prog.bin and
// the executable, or {id, ok: false, error}.
//
// {type: "format", id, files} runs gofmt over the Go files sent as {path,
// text}, which needs only gofmt.wasm, not the compiler, and is answered by
// {id, ok, files} with those that changed, or {id, ok: false, error} with
// gofmt's complaints, which name the files as they were sent.
//
// {type: "convert", id, files} runs the project's go:generate lines other
// than discgen's, the converters that make assets/ from assets/, which needs
// only their modules, and is answered by {id, ok, files}, the files under
// assets/ and assets/ they made or changed, with the tools' output sent as log
// events meanwhile.
import { Dir, FileSystem, WASI } from "./wasi.js";
import { Pipeline, readTar } from "./pipeline.js";

let pipeline = null; // a promise of the Pipeline, once loading has started

// The build's stamp, which playground.js put after this worker's name,
// carried on to the tools and the sysroot, which go with the pipeline; see
// versioned in app.js. The compiler itself is not stamped: it is large,
// changes on its own schedule, and speaks WASI whatever the build.
const BUILD = new URL(import.meta.url).searchParams.get("v") ?? "";
const versioned = (url) => (BUILD ? `${url}?v=${BUILD}` : url);

async function fetchModule(url) {
	const res = await fetch(url);
	if (!res.ok) throw new Error(`${url}: ${res.status} ${res.statusText}`);
	return WebAssembly.compileStreaming(res);
}

// The converters, each fetched the first time a generate line names it.
const tools = new Map(); // name to a promise of its module
function loadTool(name) {
	if (!tools.has(name)) {
		tools.set(name, fetchModule(versioned(`tools/${name}.wasm`)).catch((err) => {
			tools.delete(name);
			throw err;
		}));
	}
	return tools.get(name);
}

// converters is a Pipeline without the compiler, for convert: the
// converters alone, loaded as they are named.
let converters = null;
function converterPipeline() {
	converters ??= new Pipeline({}, new Dir(), {}, loadTool);
	return converters;
}

// fetchTar fetches a gzipped tarball and returns its bytes.
async function fetchTar(url) {
	const res = await fetch(url);
	if (!res.ok) throw new Error(`${url}: ${res.status} ${res.statusText}`);
	const stream = res.body.pipeThrough(new DecompressionStream("gzip"));
	return new Uint8Array(await new Response(stream).arrayBuffer());
}

async function load(log) {
	log("Loading the compiler…\n");
	const [tinygo, discgen, mips1fix, eco2exe, mkpsxiso, tar] = await Promise.all([
		fetchModule("tinygo.wasm"),
		fetchModule(versioned("tools/discgen.wasm")),
		fetchModule(versioned("tools/mips1fix.wasm")),
		fetchModule(versioned("tools/eco2exe.wasm")),
		fetchModule(versioned("tools/mkpsxiso.wasm")),
		fetchTar(versioned("sysroot.tar.gz")),
	]);
	const fs = new FileSystem();
	readTar(fs, tar, "/");
	const sysroot = fs.lookup([], "/sysroot").node;
	const env = JSON.parse(new TextDecoder().decode(fs.get("/sysroot/env.json")));
	return new Pipeline({ tinygo, discgen, mips1fix, eco2exe, mkpsxiso }, sysroot, env, loadTool);
}

let gofmt = null; // a promise of the compiled gofmt.wasm

// format runs gofmt over each of files, {path, text}, and returns the ones
// it changed, or every complaint it had. It runs once per file, printing to
// its output, rather than once with -w over them all: writing in place
// goes through a backup file, which the file system here has no need to
// support.
async function format(files) {
	gofmt ??= fetchModule(versioned("tools/gofmt.wasm")).catch((err) => {
		gofmt = null;
		throw err;
	});
	const module = await gofmt;
	const encoder = new TextEncoder();
	const changed = [];
	let errors = "";
	for (const { path, text } of files) {
		const fs = new FileSystem();
		fs.put("/work/" + path, encoder.encode(text));
		let stdout = "";
		let stderr = "";
		const wasi = new WASI({
			args: ["gofmt", path],
			env: { PWD: "/work" },
			fs,
			stdout: (s) => { stdout += s; },
			stderr: (s) => { stderr += s; },
		});
		const code = wasi.start(await WebAssembly.instantiate(module, wasi.imports));
		if (code !== 0) errors += stderr || `gofmt failed on ${path} with exit code ${code}\n`;
		else if (stdout !== text) changed.push({ path, text: stdout });
	}
	if (errors) return { ok: false, error: errors };
	return { ok: true, files: changed };
}

self.onmessage = async (e) => {
	const msg = e.data;
	if (msg.type === "format") {
		try {
			self.postMessage({ id: msg.id, ...(await format(msg.files)) });
		} catch (err) {
			self.postMessage({ id: msg.id, ok: false, error: err.message });
		}
		return;
	}
	const log = (text) => self.postMessage({ id: msg.id, event: "log", text });
	if (msg.type === "convert") {
		try {
			const result = await converterPipeline().convert(msg.files, log);
			self.postMessage({ id: msg.id, ok: result.ok, files: result.files }, result.files.map((f) => f.data.buffer));
		} catch (err) {
			self.postMessage({ id: msg.id, ok: false, error: err.message });
		}
		return;
	}
	if (msg.type !== "build" && msg.type !== "load") return;
	try {
		pipeline ??= load(log);
		const loaded = await pipeline;
		if (msg.type === "load") {
			self.postMessage({ id: msg.id, ok: true });
			return;
		}
		const result = await loaded.build(msg.files, log);
		if (!result.ok) {
			self.postMessage({ id: msg.id, ok: false, error: "the build failed" });
			return;
		}
		const transfer = [...Object.values(result.files), result.elf].map((f) => f.buffer);
		self.postMessage({ id: msg.id, ok: true, files: result.files, elf: result.elf }, transfer);
	} catch (err) {
		pipeline = null;
		self.postMessage({ id: msg.id, ok: false, error: err.message });
	}
};

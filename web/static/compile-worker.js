// The playground's build thread. It fetches the compiler, the disc tools and
// the sysroot the first time it is asked to build, keeps them, and builds
// each program it is sent with pipeline.js.
//
// The page sends {type: "build", id, source, assets}, the text of a main
// package and the files beside it as {path, data}, or {type: "load", id} to fetch the compiler ahead of the first build,
// answered by {id, ok} or {id, ok: false, error}. While loading or
// building, the worker sends {id, event: "log", text} for each line of
// output, and when done {id, ok, files, elf} with prog.cue and prog.bin and
// the executable, or {id, ok: false, error}.
//
// {type: "format", id, source} runs gofmt over source, which needs only
// gofmt.wasm, not the compiler, and is answered by {id, ok, source}, or
// {id, ok: false, error} with gofmt's complaints, which name the file
// main.go.
import { FileSystem, WASI } from "./wasi.js";
import { Pipeline, readTar } from "./pipeline.js";

let pipeline = null; // a promise of the Pipeline, once loading has started

async function fetchModule(url) {
	const res = await fetch(url);
	if (!res.ok) throw new Error(`${url}: ${res.status} ${res.statusText}`);
	return WebAssembly.compileStreaming(res);
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
		fetchModule("tools/discgen.wasm"),
		fetchModule("tools/mips1fix.wasm"),
		fetchModule("tools/eco2exe.wasm"),
		fetchModule("tools/mkpsxiso.wasm"),
		fetchTar("sysroot.tar.gz"),
	]);
	const fs = new FileSystem();
	readTar(fs, tar, "/");
	const sysroot = fs.lookup([], "/sysroot").node;
	const env = JSON.parse(new TextDecoder().decode(fs.get("/sysroot/env.json")));
	return new Pipeline({ tinygo, discgen, mips1fix, eco2exe, mkpsxiso }, sysroot, env);
}

let gofmt = null; // a promise of the compiled gofmt.wasm

async function format(source) {
	gofmt ??= fetchModule("tools/gofmt.wasm").catch((err) => {
		gofmt = null;
		throw err;
	});
	const module = await gofmt;
	const fs = new FileSystem();
	fs.mkdir("/work");
	fs.put("/work/main.go", new TextEncoder().encode(source));
	let stdout = "";
	let stderr = "";
	const wasi = new WASI({
		args: ["gofmt", "main.go"],
		env: { PWD: "/work" },
		fs,
		stdout: (s) => { stdout += s; },
		stderr: (s) => { stderr += s; },
	});
	const code = wasi.start(await WebAssembly.instantiate(module, wasi.imports));
	if (code !== 0) return { ok: false, error: stderr || `gofmt failed with exit code ${code}` };
	return { ok: true, source: stdout };
}

self.onmessage = async (e) => {
	const msg = e.data;
	if (msg.type === "format") {
		try {
			self.postMessage({ id: msg.id, ...(await format(msg.source)) });
		} catch (err) {
			self.postMessage({ id: msg.id, ok: false, error: err.message });
		}
		return;
	}
	if (msg.type !== "build" && msg.type !== "load") return;
	const log = (text) => self.postMessage({ id: msg.id, event: "log", text });
	try {
		pipeline ??= load(log);
		const loaded = await pipeline;
		if (msg.type === "load") {
			self.postMessage({ id: msg.id, ok: true });
			return;
		}
		const result = await loaded.build(msg.source, log, msg.assets ?? []);
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

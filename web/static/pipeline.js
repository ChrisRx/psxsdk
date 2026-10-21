// The build pipeline the playground runs in a worker: TinyGo, compiled to
// WebAssembly, builds the program for the Net Yaroze, then the disc tools,
// also WebAssembly, fix the executable, convert it and write it onto a disc
// image, all against one file system held in memory.
//
// It is the same pipeline as the Taskfile's disc task, after go generate,
// which here runs the lines naming this repo's converters and discgen:
//
//	png2tim ..., obj2tmd ..., discgen assets   (the //go:generate lines, in
//	                                          order, in any of the Go files)
//	tinygo build -target=playstation-yaroze -o prog.elf ./prog
//	mips1fix prog.elf
//	eco2exe prog.elf prog.exe
//	mkpsxiso -o prog.bin --volume-id prog.exe --boot MAIN.EXE --file MAIN.EXE=prog.exe --src prog
//
// It runs in a browser worker or in Node, so it takes the module loader and
// the fetched bytes from its caller rather than fetching itself.
import { Dir, ExitError, File, FileSystem, WASI } from "./wasi.js";

// readTar adds the files of a tar archive (ustar or GNU, uncompressed) to
// fs under prefix.
export function readTar(fs, bytes, prefix) {
	const decoder = new TextDecoder();
	const str = (at, len) => {
		const end = bytes.indexOf(0, at);
		return decoder.decode(bytes.subarray(at, end < 0 || end > at + len ? at + len : end));
	};
	let pos = 0;
	let longName = null;
	while (pos + 512 <= bytes.length) {
		if (bytes[pos] === 0) break; // the end-of-archive blocks
		let name = str(pos, 100);
		const size = parseInt(str(pos + 124, 12), 8) || 0;
		const type = String.fromCharCode(bytes[pos + 156]);
		if (str(pos + 257, 6) === "ustar") {
			const dirPrefix = str(pos + 345, 155);
			if (dirPrefix) name = dirPrefix + "/" + name;
		}
		const data = bytes.subarray(pos + 512, pos + 512 + size);
		pos += 512 + Math.ceil(size / 512) * 512;
		if (type === "L") {
			longName = str(pos - Math.ceil(size / 512) * 512 - 512 + 512, size).replace(/\0+$/, "");
			longName = decoder.decode(data).replace(/\0+$/, "");
			continue;
		}
		if (longName !== null) {
			name = longName;
			longName = null;
		}
		name = name.replace(/^\.\//, "");
		if (type === "5" || name.endsWith("/")) {
			fs.mkdir(prefix + "/" + name);
		} else if (type === "0" || type === "\0") {
			fs.put(prefix + "/" + name, data);
		}
	}
}

// The tools a //go:generate line may name that the page has as WebAssembly:
// discgen, whose output, the declarations of the program's data files, the
// program needs beside it, and the converters that make those files from
// the sources under assets/.
export const GENERATORS = new Set(["discgen", "png2tim", "obj2tmd", "wav2vag", "mkvab", "smf2seq", "mkscene", "mkarc"]);

// generateLines returns the //go:generate lines of a Go source, in order,
// as {tool, args, line}: tool is what the line runs, "go run
// go.chrisrx.dev/psx/tools/NAME" or "go tool NAME" giving NAME, and any
// other command as its first word; args are the rest, split on spaces.
export function generateLines(source) {
	const out = [];
	for (const m of source.matchAll(/^\/\/go:generate[ \t]+(.+?)[ \t]*$/gm)) {
		const line = m[1];
		const words = line.split(/\s+/);
		let tool = words[0];
		let args = words.slice(1);
		if (words[0] === "go" && words[1] === "run" && /^go\.chrisrx\.dev\/psx\/tools\/[\w-]+(@\S+)?$/.test(words[2] ?? "")) {
			tool = words[2].replace(/^.*\//, "").replace(/@.*$/, "");
			args = words.slice(3);
		} else if (words[0] === "go" && words[1] === "tool" && words[2]) {
			tool = words[2];
			args = words.slice(3);
		}
		out.push({ tool, args, line });
	}
	return out;
}

// isGoSource says whether a project file, by its path, is one of the
// program's Go files: at the root, beside main.go.
export function isGoSource(path) {
	return /^[^/]+\.go$/.test(path);
}

// projectGenerateLines gathers the generate lines of a project's Go files,
// main.go first and the rest by name, as go generate orders them.
export function projectGenerateLines(project) {
	const decoder = new TextDecoder();
	const gos = project.filter((f) => isGoSource(f.path)).sort((a, b) => (a.path === "main.go" ? -1 : b.path === "main.go" ? 1 : a.path.localeCompare(b.path)));
	const out = [];
	for (const f of gos) out.push(...generateLines(decoder.decode(f.data)));
	return out;
}

// A Pipeline holds what every build shares: the compiled modules, the
// sysroot, and the compiler's cache. The cache comes filled in the sysroot,
// with the standard library and the C the target needs already compiled,
// since the compiler has no clang of its own; what a build adds stays for
// the next one.
export class Pipeline {
	// modules: {tinygo, discgen, mips1fix, eco2exe, mkpsxiso} as
	// WebAssembly.Module; loadModule: a function from a tool's name to a
	// promise of its module, for the converters, fetched when first named;
	// sysroot: the Dir the sysroot tarball was read into; env: its env.json.
	constructor(modules, sysroot, env, loadModule = null) {
		this.modules = modules;
		this.sysroot = sysroot;
		this.env = env;
		this.loadModule = loadModule;
		this.cache = sysroot.children.get("cache") ?? new Dir();
		// The file system every build runs in: the sysroot and the cache
		// stay, /work is the project's and is put there for each build.
		this.fs = new FileSystem();
		this.fs.root.children.set("sysroot", this.sysroot);
		this.fs.mkdir("/tmp/cache");
		this.fs.lookup([], "/tmp/cache").node.children.set("tinygo", this.cache);
		this.compiler = modules.tinygo ? new Compiler(modules.tinygo, this.fs, this.environment()) : null;
	}

	// environment is what the compiler runs with.
	environment() {
		return {
			GOROOT: "/sysroot/" + this.env.GOROOT,
			GOVERSION: this.env.GOVERSION,
			GOTOOLTAGS: this.env.GOTOOLTAGS,
			TINYGOROOT: "/sysroot/" + this.env.TINYGOROOT,
			GOMODCACHE: "/sysroot/" + this.env.GOMODCACHE,
			HOME: "/tmp",
			XDG_CACHE_HOME: "/tmp/cache",
			TMPDIR: "/tmp",
			PWD: "/work",
			// The cache is keyed by the compiler the host filled it with.
			TINYGO_BUILD_ID: this.env.buildID ?? "",
		};
	}

	// stage puts a project's files, {path, data}, under /work/prog in a new
	// file system, with assets/ made, and returns it.
	stage(project) {
		const fs = new FileSystem();
		fs.mkdir("/work/prog/assets");
		for (const { path, data } of project) fs.put("/work/prog/" + path, data);
		return fs;
	}

	// generate runs the project's generate lines in fs, as go generate would
	// in the package directory, those naming the tools the page has; the
	// rest are noted and skipped. It returns whether every one succeeded.
	async generate(project, fs, log) {
		for (const { tool, args, line } of projectGenerateLines(project)) {
			if (tool === "mkdir") continue; // assets/ is made already
			if (!GENERATORS.has(tool)) {
				log(`skipped: ${line} (not a tool the page runs)\n`);
				continue;
			}
			const started = performance.now();
			const code = await this.run(tool, [tool, ...args], { PWD: "/work/prog", GOPACKAGE: "main", GOFILE: "main.go" }, fs, log);
			const took = ((performance.now() - started) / 1000).toFixed(1);
			if (code !== 0) {
				log(`${tool} failed with exit code ${code} after ${took}s\n`);
				return false;
			}
			log(`${tool}: ${took}s\n`);
		}
		return true;
	}

	// build compiles a project, a list of {path, data} with the program's Go
	// files at the root, its data files under assets/ and its sources under
	// assets/, and returns {ok, files, elf}, where files holds prog.cue and
	// prog.bin when ok, and elf is the executable, for its symbols. log is
	// called with each line of output as it comes. The data files, such as
	// assets/CUBE.TIM, are what the program registers with package disc and
	// mkpsxiso puts on the disc.
	//
	// With hot, the program is built to take patches (see patch): its main
	// package's functions are called through slots and memory is reserved
	// for the patches.
	async build(project, log, { hot = false } = {}) {
		const { fs, env } = this.workspace(project);
		if (!(await this.generate(project, fs, log))) return { ok: false };
		const tinygo = ["tinygo", "-target=" + this.env.target];
		if (hot) tinygo.push("-hot");
		const steps = [
			["tinygo", [...tinygo, "-o", "/work/prog.elf", "/work/prog"]],
			["mips1fix", ["mips1fix", "-q", "/work/prog.elf"]],
			["eco2exe", ["eco2exe", "/work/prog.elf", "/work/prog.exe"]],
			["mkpsxiso", ["mkpsxiso", "-o", "/work/prog.bin", "--volume-id", "prog.exe", "--boot", "MAIN.EXE", "--file", "MAIN.EXE=/work/prog.exe", "--src", "/work/prog"]],
		];
		if (!(await this.steps(steps, env, fs, log))) return { ok: false };
		return {
			ok: true,
			files: {
				"prog.cue": fs.get("/work/prog.cue"),
				"prog.bin": fs.get("/work/prog.bin"),
			},
			elf: fs.get("/work/prog.elf"),
		};
	}

	// patch compiles a project as a hot reload patch for the program whose
	// ELF is base, built with hot and running, linked at the address at
	// inside the program's region for patches, and returns {ok, elf}, the
	// patch's ELF for the emulator to load. The compiler says on the log,
	// in a line starting "hot:", when the program has to be built and
	// started again instead: a variable that changed size, say.
	async patch(project, base, at, log) {
		const { fs, env } = this.workspace(project);
		fs.put("/work/base.elf", base);
		if (!(await this.generate(project, fs, log))) return { ok: false };
		const steps = [
			["tinygo", ["tinygo", "-target=" + this.env.target, "-hot-base=/work/base.elf", "-hot-at=0x" + at.toString(16), "-o", "/work/patch.elf", "/work/prog"]],
			["mips1fix", ["mips1fix", "-q", "/work/patch.elf"]],
		];
		if (!(await this.steps(steps, env, fs, log))) return { ok: false };
		return { ok: true, elf: fs.get("/work/patch.elf") };
	}

	// workspace stages a project with the sysroot, the compiler's cache and
	// a go.mod naming the modules, and returns the file system and the
	// environment the compiler runs in.
	workspace(project) {
		const fs = this.fs;
		const staged = this.stage(project);
		fs.root.children.set("work", staged.root.children.get("work"));
		const replaces = this.env.modules.map((m) => `replace ${m.path} => /sysroot/${m.dir}`).join("\n");
		const requires = this.env.modules.map((m) => `require ${m.path} v0.0.0`).join("\n");
		fs.put("/work/go.mod", new TextEncoder().encode(`module play\n\ngo ${this.env.GOVERSION.replace(/^go/, "").split(".").slice(0, 2).join(".")}\n\n${requires}\n\n${replaces}\n`));
		return { fs, env: this.environment() };
	}

	// steps runs each of steps, [name, args], in turn, logging how long each
	// took, and returns whether every one succeeded.
	async steps(steps, env, fs, log) {
		for (const [name, args] of steps) {
			const started = performance.now();
			const code = name === "tinygo" && this.compiler ? await this.compiler.build(args.slice(1), log) : await this.run(name, args, env, fs, log);
			const took = ((performance.now() - started) / 1000).toFixed(1);
			if (code !== 0) {
				log(`${name} failed with exit code ${code} after ${took}s\n`);
				return false;
			}
			log(`${name}: ${took}s\n`);
		}
		return true;
	}

	// convert runs the project's generate lines other than discgen's, and
	// returns {ok, files}: the files under assets/ and the archives'
	// directories under pack/ that the tools made or changed, as {path,
	// data}, for the page to keep.
	async convert(project, log) {
		const fs = this.stage(project);
		const lines = project.filter((f) => !isGoSource(f.path));
		const sources = new Map(lines.map((f) => [f.path, f.data]));
		const ok = await this.generate(project.map((f) => (isGoSource(f.path) ? { path: f.path, data: withoutDiscgen(f.data) } : f)), fs, log);
		const files = [];
		const collect = (dir) => {
			const node = fs.lookup([], "/work/prog/" + dir).node;
			if (!node) return;
			for (const [name, child] of node.children) {
				if (!(child instanceof File)) continue;
				const path = `${dir}/${name}`;
				const data = child.data.slice();
				const before = sources.get(path);
				if (before && before.length === data.length && before.every((b, i) => b === data[i])) continue;
				files.push({ path, data });
			}
		};
		collect("assets");
		const pack = fs.lookup([], "/work/prog/pack").node;
		for (const [name, child] of pack?.children ?? []) {
			if (child instanceof Dir) collect(`pack/${name}`);
		}
		return { ok, files };
	}

	async run(name, args, env, fs, log) {
		let module = this.modules[name];
		if (!module && this.loadModule) module = this.modules[name] = await this.loadModule(name);
		if (!module) throw new Error(`no ${name} to run`);
		const wasi = new WASI({ args, env, fs, stdout: log, stderr: log });
		const instance = await WebAssembly.instantiate(module, wasi.imports);
		return wasi.start(instance);
	}
}

// A Compiler is the compiler's module loaded once and asked to build as
// often as needed, rather than started afresh for each build: between
// builds it keeps the standard library parsed and type-checked, which is
// most of a build's time outside LLVM, and the browser keeps the code it
// has compiled of it. The module is a reactor: the host writes the
// arguments where tinygo_args says and calls tinygo_build.
class Compiler {
	constructor(module, fs, env) {
		this.module = module;
		this.fs = fs;
		this.env = env;
		this.wasi = null;
	}

	async ready() {
		if (this.wasi) return;
		const wasi = new WASI({ args: ["tinygo"], env: this.env, fs: this.fs, stdout: () => {}, stderr: () => {} });
		const instance = await WebAssembly.instantiate(this.module, wasi.imports);
		wasi.initialize(instance);
		this.wasi = wasi;
		this.argsAt = wasi.call("tinygo_args");
	}

	// build runs one build with the arguments given, after the command's
	// name, and returns its exit status. A build that crashes the module
	// loses it; the next build loads it again.
	async build(args, log) {
		await this.ready();
		this.wasi.setStreams(log, log);
		const bytes = new TextEncoder().encode(args.join("\0"));
		this.wasi.bytes().set(bytes, this.argsAt);
		try {
			return this.wasi.call("tinygo_build", bytes.length);
		} catch (e) {
			this.wasi = null;
			if (e instanceof ExitError) return e.code || 1;
			log(`the compiler failed: ${e.message}\n`);
			return 1;
		}
	}
}

// withoutDiscgen takes the discgen line out of a Go source's bytes, for
// convert, which makes the data files and not the declarations.
function withoutDiscgen(data) {
	const text = new TextDecoder().decode(data);
	return new TextEncoder().encode(text.replace(/^\/\/go:generate[ \t]+go[ \t]+(?:run[ \t]+go\.chrisrx\.dev\/psx\/tools\/discgen(?:@\S+)?|tool[ \t]+discgen)\b.*$/gm, ""));
}

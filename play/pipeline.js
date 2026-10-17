// The build pipeline the playground runs in a worker: TinyGo, compiled to
// WebAssembly, builds the program for the Net Yaroze, then the disc tools,
// also WebAssembly, fix the executable, convert it and write it onto a disc
// image, all against one file system held in memory.
//
// It is the same pipeline as the Taskfile's disc task:
//
//	tinygo build -target=playstation-yaroze -o prog.elf ./prog
//	mips1fix prog.elf
//	eco2exe prog.elf prog.exe
//	mkpsxiso -o prog.bin --volume-id prog.exe --boot MAIN.EXE --file MAIN.EXE=prog.exe --src prog
//
// It runs in a browser worker or in Node, so it takes the module loader and
// the fetched bytes from its caller rather than fetching itself.
import { Dir, File, FileSystem, WASI } from "./wasi.js";

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

// A Pipeline holds what every build shares: the compiled modules, the
// sysroot, and the compiler's cache. The cache comes filled in the sysroot,
// with the standard library and the C the target needs already compiled,
// since the compiler has no clang of its own; what a build adds stays for
// the next one.
export class Pipeline {
	// modules: {tinygo, mips1fix, eco2exe, mkpsxiso} as WebAssembly.Module;
	// sysroot: the Dir the sysroot tarball was read into; env: its env.json.
	constructor(modules, sysroot, env) {
		this.modules = modules;
		this.sysroot = sysroot;
		this.env = env;
		this.cache = sysroot.children.get("cache") ?? new Dir();
	}

	// build compiles source, the text of a main package in one file, and
	// returns {ok, log, files, elf}, where files holds prog.cue and prog.bin
	// when ok, and elf is the executable, for its symbols. log is called
	// with each line of output as it comes.
	async build(source, log) {
		const fs = new FileSystem();
		fs.root.children.set("sysroot", this.sysroot);
		fs.mkdir("/tmp");
		fs.mkdir("/tmp/cache");
		fs.lookup([], "/tmp/cache").node.children.set("tinygo", this.cache);
		fs.mkdir("/work/prog");
		fs.put("/work/prog/main.go", new TextEncoder().encode(source));
		const replaces = this.env.modules.map((m) => `replace ${m.path} => /sysroot/${m.dir}`).join("\n");
		const requires = this.env.modules.map((m) => `require ${m.path} v0.0.0`).join("\n");
		fs.put("/work/go.mod", new TextEncoder().encode(`module play\n\ngo ${this.env.GOVERSION.replace(/^go/, "").split(".").slice(0, 2).join(".")}\n\n${requires}\n\n${replaces}\n`));

		const env = {
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
		const steps = [
			["tinygo", ["tinygo", "-target=" + this.env.target, "-o", "/work/prog.elf", "/work/prog"]],
			["mips1fix", ["mips1fix", "-q", "/work/prog.elf"]],
			["eco2exe", ["eco2exe", "/work/prog.elf", "/work/prog.exe"]],
			["mkpsxiso", ["mkpsxiso", "-o", "/work/prog.bin", "--volume-id", "prog.exe", "--boot", "MAIN.EXE", "--file", "MAIN.EXE=/work/prog.exe", "--src", "/work/prog"]],
		];
		for (const [name, args] of steps) {
			const started = performance.now();
			const code = await this.run(name, args, env, fs, log);
			const took = ((performance.now() - started) / 1000).toFixed(1);
			if (code !== 0) {
				log(`${name} failed with exit code ${code} after ${took}s\n`);
				return { ok: false };
			}
			log(`${name}: ${took}s\n`);
		}
		return {
			ok: true,
			files: {
				"prog.cue": fs.get("/work/prog.cue"),
				"prog.bin": fs.get("/work/prog.bin"),
			},
			elf: fs.get("/work/prog.elf"),
		};
	}

	async run(name, args, env, fs, log) {
		const wasi = new WASI({ args, env, fs, stdout: log, stderr: log });
		const instance = await WebAssembly.instantiate(this.modules[name], wasi.imports);
		return wasi.start(instance);
	}
}

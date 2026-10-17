// A WASI preview 1 host for running a command-line program against a file
// system held in memory, in a browser worker or in Node.
//
// It is what the compiler needs and no more: files and directories under one
// preopened root, standard output and error delivered as text, arguments,
// environment, clocks and randomness. There are no sockets, no signals and
// no processes, which is what the program expects of WASI.

export const ERRNO = {
	SUCCESS: 0, E2BIG: 1, EACCES: 2, EBADF: 8, EEXIST: 20, EINVAL: 28,
	EIO: 29, EISDIR: 31, ENAMETOOLONG: 37, ENOENT: 44, ENOSYS: 52,
	ENOTDIR: 54, ENOTEMPTY: 55, ENOTSUP: 58, EPERM: 63, ESPIPE: 70,
};

const FILETYPE = { UNKNOWN: 0, BLOCK_DEVICE: 1, CHARACTER_DEVICE: 2, DIRECTORY: 3, REGULAR_FILE: 4, SYMBOLIC_LINK: 7 };
const OFLAGS = { CREAT: 1, DIRECTORY: 2, EXCL: 4, TRUNC: 8 };
const FDFLAGS = { APPEND: 1 };
const RIGHTS_ALL = 0x1fffffffn;
const WHENCE = { SET: 0, CUR: 1, END: 2 };
const CLOCK = { REALTIME: 0, MONOTONIC: 1 };
const PREOPEN_FD = 3;

class ExitError extends Error {
	constructor(code) {
		super("exit " + code);
		this.code = code;
	}
}

// A file system node: a directory with named children, or a file holding
// bytes. Files keep their bytes in a buffer that grows by doubling, so that
// appending a few bytes at a time, which the compiler's writers do, stays
// cheap.
export class Dir {
	constructor() {
		this.children = new Map();
		this.mtime = BigInt(Date.now()) * 1000000n;
	}
	get type() { return FILETYPE.DIRECTORY; }
}

export class File {
	constructor(data) {
		this.buf = data ? new Uint8Array(data) : new Uint8Array(0);
		this.size = this.buf.length;
		this.mtime = BigInt(Date.now()) * 1000000n;
	}
	get type() { return FILETYPE.REGULAR_FILE; }
	get data() { return this.buf.subarray(0, this.size); }
	write(at, bytes) {
		const end = at + bytes.length;
		if (end > this.buf.length) {
			let cap = Math.max(this.buf.length * 2, 4096);
			while (cap < end) cap *= 2;
			const grown = new Uint8Array(cap);
			grown.set(this.buf.subarray(0, this.size));
			this.buf = grown;
		}
		this.buf.set(bytes, at);
		if (end > this.size) this.size = end;
		this.mtime = BigInt(Date.now()) * 1000000n;
	}
	truncate(size) {
		if (size > this.buf.length) {
			const grown = new Uint8Array(size);
			grown.set(this.buf.subarray(0, this.size));
			this.buf = grown;
		}
		this.size = size;
	}
}

// A file system is a root directory and the operations on paths within it.
export class FileSystem {
	constructor() {
		this.root = new Dir();
	}

	// Returns the parent directory and final name of path, creating no
	// directories. Paths are absolute or relative to base, with . and ..
	// resolved.
	resolveParent(base, path) {
		const parts = (path.startsWith("/") ? [] : base.slice()).concat(path.split("/").filter((p) => p !== "" && p !== "."));
		const names = [];
		for (const p of parts) {
			if (p === "..") names.pop();
			else names.push(p);
		}
		const name = names.pop();
		let dir = this.root;
		for (const n of names) {
			const next = dir.children.get(n);
			if (!next) return { errno: ERRNO.ENOENT };
			if (next.type !== FILETYPE.DIRECTORY) return { errno: ERRNO.ENOTDIR };
			dir = next;
		}
		return { dir, name, names };
	}

	lookup(base, path) {
		const r = this.resolveParent(base, path);
		if (r.errno) return r;
		if (r.name === undefined) return { node: this.root, names: [] };
		const node = r.dir.children.get(r.name);
		if (!node) return { errno: ERRNO.ENOENT };
		return { node, names: r.names.concat(r.name) };
	}

	// Puts data at path, making the directories on the way: how the host
	// fills the file system before the program starts.
	put(path, data) {
		const parts = path.split("/").filter((p) => p !== "");
		let dir = this.root;
		for (const p of parts.slice(0, -1)) {
			let next = dir.children.get(p);
			if (!next) {
				next = new Dir();
				dir.children.set(p, next);
			}
			dir = next;
		}
		dir.children.set(parts[parts.length - 1], new File(data));
	}

	mkdir(path) {
		const parts = path.split("/").filter((p) => p !== "");
		let dir = this.root;
		for (const p of parts) {
			let next = dir.children.get(p);
			if (!next) {
				next = new Dir();
				dir.children.set(p, next);
			}
			dir = next;
		}
		return dir;
	}

	// Returns the bytes of the file at path, or null.
	get(path) {
		const r = this.lookup([], path);
		if (r.errno || r.node.type !== FILETYPE.REGULAR_FILE) return null;
		return r.node.data;
	}
}

// An open file description.
class FD {
	constructor(node, names, fdflags) {
		this.node = node;
		this.names = names; // path components, for relative opens from a directory
		this.pos = 0;
		this.fdflags = fdflags || 0;
	}
}

export class WASI {
	// options: args, env (object), fs (FileSystem), stdout(text), stderr(text)
	constructor(options) {
		this.args = options.args || [];
		this.env = Object.entries(options.env || {}).map(([k, v]) => k + "=" + v);
		this.fs = options.fs;
		this.stdout = options.stdout || ((s) => console.log(s));
		this.stderr = options.stderr || ((s) => console.error(s));
		this.fds = new Map();
		this.fds.set(0, { stdin: true });
		this.fds.set(1, { stream: this.stdout, pending: "" });
		this.fds.set(2, { stream: this.stderr, pending: "" });
		this.fds.set(PREOPEN_FD, new FD(this.fs.root, [], 0));
		this.nextFD = PREOPEN_FD + 1;
		this.exitCode = null;
		this.decoder = new TextDecoder();
		this.encoder = new TextEncoder();
		this.started = performance.now();
	}

	// Runs the instantiated module's _start and returns its exit code.
	start(instance) {
		this.instance = instance;
		this.memory = instance.exports.memory;
		try {
			instance.exports._start();
		} catch (e) {
			if (!(e instanceof ExitError)) throw e;
		}
		this.flush(1);
		this.flush(2);
		return this.exitCode === null ? 0 : this.exitCode;
	}

	flush(fd) {
		const f = this.fds.get(fd);
		if (f && f.pending) {
			f.stream(f.pending);
			f.pending = "";
		}
	}

	view() { return new DataView(this.memory.buffer); }
	bytes() { return new Uint8Array(this.memory.buffer); }
	string(ptr, len) { return this.decoder.decode(this.bytes().subarray(ptr, ptr + len)); }

	allocFD(fd) {
		const n = this.nextFD++;
		this.fds.set(n, fd);
		return n;
	}

	// The imports object for WebAssembly.instantiate.
	get imports() {
		const self = this;
		const fn = (name, impl) => { wasi[name] = (...args) => { try { return impl.apply(self, args); } catch (e) { if (e instanceof ExitError) throw e; self.stderr("wasi " + name + ": " + (e.stack || e)); return ERRNO.EIO; } }; };
		const wasi = {};
		fn("args_sizes_get", this.args_sizes_get);
		fn("args_get", this.args_get);
		fn("environ_sizes_get", this.environ_sizes_get);
		fn("environ_get", this.environ_get);
		fn("clock_res_get", this.clock_res_get);
		fn("clock_time_get", this.clock_time_get);
		fn("fd_advise", () => ERRNO.SUCCESS);
		fn("fd_allocate", () => ERRNO.ENOTSUP);
		fn("fd_close", this.fd_close);
		fn("fd_datasync", () => ERRNO.SUCCESS);
		fn("fd_fdstat_get", this.fd_fdstat_get);
		fn("fd_fdstat_set_flags", this.fd_fdstat_set_flags);
		fn("fd_fdstat_set_rights", () => ERRNO.SUCCESS);
		fn("fd_filestat_get", this.fd_filestat_get);
		fn("fd_filestat_set_size", this.fd_filestat_set_size);
		fn("fd_filestat_set_times", () => ERRNO.SUCCESS);
		fn("fd_pread", this.fd_pread);
		fn("fd_prestat_get", this.fd_prestat_get);
		fn("fd_prestat_dir_name", this.fd_prestat_dir_name);
		fn("fd_pwrite", this.fd_pwrite);
		fn("fd_read", this.fd_read);
		fn("fd_readdir", this.fd_readdir);
		fn("fd_renumber", this.fd_renumber);
		fn("fd_seek", this.fd_seek);
		fn("fd_sync", () => ERRNO.SUCCESS);
		fn("fd_tell", this.fd_tell);
		fn("fd_write", this.fd_write);
		fn("path_create_directory", this.path_create_directory);
		fn("path_filestat_get", this.path_filestat_get);
		fn("path_filestat_set_times", () => ERRNO.SUCCESS);
		fn("path_link", () => ERRNO.ENOTSUP);
		fn("path_open", this.path_open);
		fn("path_readlink", () => ERRNO.EINVAL);
		fn("path_remove_directory", this.path_remove_directory);
		fn("path_rename", this.path_rename);
		fn("path_symlink", () => ERRNO.ENOTSUP);
		fn("path_unlink_file", this.path_unlink_file);
		fn("poll_oneoff", this.poll_oneoff);
		fn("proc_exit", this.proc_exit);
		fn("proc_raise", () => ERRNO.ENOSYS);
		fn("sched_yield", () => ERRNO.SUCCESS);
		fn("random_get", this.random_get);
		fn("sock_accept", () => ERRNO.ENOSYS);
		fn("sock_recv", () => ERRNO.ENOSYS);
		fn("sock_send", () => ERRNO.ENOSYS);
		fn("sock_shutdown", () => ERRNO.ENOSYS);
		return { wasi_snapshot_preview1: wasi };
	}

	writeStrings(strings, ptrsPtr, bufPtr) {
		const view = this.view();
		const bytes = this.bytes();
		for (const s of strings) {
			view.setUint32(ptrsPtr, bufPtr, true);
			ptrsPtr += 4;
			const b = this.encoder.encode(s);
			bytes.set(b, bufPtr);
			bytes[bufPtr + b.length] = 0;
			bufPtr += b.length + 1;
		}
	}
	sizeStrings(strings, countPtr, sizePtr) {
		const view = this.view();
		view.setUint32(countPtr, strings.length, true);
		view.setUint32(sizePtr, strings.reduce((n, s) => n + this.encoder.encode(s).length + 1, 0), true);
		return ERRNO.SUCCESS;
	}
	args_sizes_get(countPtr, sizePtr) { return this.sizeStrings(this.args, countPtr, sizePtr); }
	args_get(ptrs, buf) { this.writeStrings(this.args, ptrs, buf); return ERRNO.SUCCESS; }
	environ_sizes_get(countPtr, sizePtr) { return this.sizeStrings(this.env, countPtr, sizePtr); }
	environ_get(ptrs, buf) { this.writeStrings(this.env, ptrs, buf); return ERRNO.SUCCESS; }

	clock_res_get(id, ptr) {
		this.view().setBigUint64(ptr, 1000000n, true);
		return ERRNO.SUCCESS;
	}
	clock_time_get(id, precision, ptr) {
		let ns;
		if (id === CLOCK.REALTIME) ns = BigInt(Date.now()) * 1000000n;
		else ns = BigInt(Math.round((performance.now() - this.started) * 1e6));
		this.view().setBigUint64(ptr, ns, true);
		return ERRNO.SUCCESS;
	}

	fd_close(fd) {
		if (!this.fds.has(fd)) return ERRNO.EBADF;
		if (fd <= 2) return ERRNO.SUCCESS;
		this.fds.delete(fd);
		return ERRNO.SUCCESS;
	}

	fd_fdstat_get(fd, ptr) {
		const f = this.fds.get(fd);
		if (!f) return ERRNO.EBADF;
		const view = this.view();
		let type = FILETYPE.CHARACTER_DEVICE;
		if (f.node) type = f.node.type;
		view.setUint8(ptr, type);
		view.setUint16(ptr + 2, f.fdflags || 0, true);
		view.setBigUint64(ptr + 8, RIGHTS_ALL, true);
		view.setBigUint64(ptr + 16, RIGHTS_ALL, true);
		return ERRNO.SUCCESS;
	}
	fd_fdstat_set_flags(fd, flags) {
		const f = this.fds.get(fd);
		if (!f) return ERRNO.EBADF;
		f.fdflags = flags;
		return ERRNO.SUCCESS;
	}

	writeFilestat(ptr, node) {
		const view = this.view();
		view.setBigUint64(ptr, 1n, true); // device
		view.setBigUint64(ptr + 8, BigInt(this.inode(node)), true);
		view.setUint8(ptr + 16, node.type);
		view.setBigUint64(ptr + 24, 1n, true); // links
		view.setBigUint64(ptr + 32, BigInt(node.type === FILETYPE.REGULAR_FILE ? node.size : 4096), true);
		view.setBigUint64(ptr + 40, node.mtime, true);
		view.setBigUint64(ptr + 48, node.mtime, true);
		view.setBigUint64(ptr + 56, node.mtime, true);
	}
	inode(node) {
		if (!this.inodes) { this.inodes = new WeakMap(); this.nextInode = 1; }
		let n = this.inodes.get(node);
		if (!n) { n = this.nextInode++; this.inodes.set(node, n); }
		return n;
	}
	fd_filestat_get(fd, ptr) {
		const f = this.fds.get(fd);
		if (!f) return ERRNO.EBADF;
		if (!f.node) {
			const view = this.view();
			new Uint8Array(this.memory.buffer, ptr, 64).fill(0);
			view.setUint8(ptr + 16, FILETYPE.CHARACTER_DEVICE);
			return ERRNO.SUCCESS;
		}
		this.writeFilestat(ptr, f.node);
		return ERRNO.SUCCESS;
	}
	fd_filestat_set_size(fd, size) {
		const f = this.fds.get(fd);
		if (!f || !f.node) return ERRNO.EBADF;
		if (f.node.type !== FILETYPE.REGULAR_FILE) return ERRNO.EISDIR;
		f.node.truncate(Number(size));
		return ERRNO.SUCCESS;
	}

	iovecs(ptr, count) {
		const view = this.view();
		const out = [];
		for (let i = 0; i < count; i++) {
			out.push({ ptr: view.getUint32(ptr + i * 8, true), len: view.getUint32(ptr + i * 8 + 4, true) });
		}
		return out;
	}
	readAt(f, pos, iovs, nreadPtr) {
		const bytes = this.bytes();
		const data = f.node.data;
		let n = 0;
		for (const iov of iovs) {
			const chunk = data.subarray(pos + n, pos + n + iov.len);
			bytes.set(chunk, iov.ptr);
			n += chunk.length;
			if (chunk.length < iov.len) break;
		}
		this.view().setUint32(nreadPtr, n, true);
		return n;
	}
	fd_read(fd, iovsPtr, iovsLen, nreadPtr) {
		const f = this.fds.get(fd);
		if (!f) return ERRNO.EBADF;
		if (f.stdin) { this.view().setUint32(nreadPtr, 0, true); return ERRNO.SUCCESS; }
		if (!f.node) return ERRNO.EBADF;
		if (f.node.type !== FILETYPE.REGULAR_FILE) return ERRNO.EISDIR;
		f.pos += this.readAt(f, f.pos, this.iovecs(iovsPtr, iovsLen), nreadPtr);
		return ERRNO.SUCCESS;
	}
	fd_pread(fd, iovsPtr, iovsLen, offset, nreadPtr) {
		const f = this.fds.get(fd);
		if (!f || !f.node) return ERRNO.EBADF;
		if (f.node.type !== FILETYPE.REGULAR_FILE) return ERRNO.EISDIR;
		this.readAt(f, Number(offset), this.iovecs(iovsPtr, iovsLen), nreadPtr);
		return ERRNO.SUCCESS;
	}
	writeAt(f, pos, iovs, nwrittenPtr) {
		const bytes = this.bytes();
		let n = 0;
		for (const iov of iovs) {
			f.node.write(pos + n, bytes.subarray(iov.ptr, iov.ptr + iov.len));
			n += iov.len;
		}
		this.view().setUint32(nwrittenPtr, n, true);
		return n;
	}
	fd_write(fd, iovsPtr, iovsLen, nwrittenPtr) {
		const f = this.fds.get(fd);
		if (!f) return ERRNO.EBADF;
		const iovs = this.iovecs(iovsPtr, iovsLen);
		if (f.stream) {
			const bytes = this.bytes();
			let n = 0;
			let text = "";
			for (const iov of iovs) {
				text += this.decoder.decode(bytes.subarray(iov.ptr, iov.ptr + iov.len), { stream: true });
				n += iov.len;
			}
			f.pending += text;
			const nl = f.pending.lastIndexOf("\n");
			if (nl >= 0) {
				f.stream(f.pending.slice(0, nl + 1));
				f.pending = f.pending.slice(nl + 1);
			}
			this.view().setUint32(nwrittenPtr, n, true);
			return ERRNO.SUCCESS;
		}
		if (!f.node) return ERRNO.EBADF;
		if (f.node.type !== FILETYPE.REGULAR_FILE) return ERRNO.EISDIR;
		if (f.fdflags & FDFLAGS.APPEND) f.pos = f.node.size;
		f.pos += this.writeAt(f, f.pos, iovs, nwrittenPtr);
		return ERRNO.SUCCESS;
	}
	fd_pwrite(fd, iovsPtr, iovsLen, offset, nwrittenPtr) {
		const f = this.fds.get(fd);
		if (!f || !f.node) return ERRNO.EBADF;
		if (f.node.type !== FILETYPE.REGULAR_FILE) return ERRNO.EISDIR;
		this.writeAt(f, Number(offset), this.iovecs(iovsPtr, iovsLen), nwrittenPtr);
		return ERRNO.SUCCESS;
	}
	fd_seek(fd, offset, whence, newPtr) {
		const f = this.fds.get(fd);
		if (!f) return ERRNO.EBADF;
		if (!f.node) return ERRNO.ESPIPE;
		if (f.node.type !== FILETYPE.REGULAR_FILE) return ERRNO.EISDIR;
		let pos;
		if (whence === WHENCE.SET) pos = Number(offset);
		else if (whence === WHENCE.CUR) pos = f.pos + Number(offset);
		else if (whence === WHENCE.END) pos = f.node.size + Number(offset);
		else return ERRNO.EINVAL;
		if (pos < 0) return ERRNO.EINVAL;
		f.pos = pos;
		this.view().setBigUint64(newPtr, BigInt(pos), true);
		return ERRNO.SUCCESS;
	}
	fd_tell(fd, ptr) {
		const f = this.fds.get(fd);
		if (!f || !f.node) return ERRNO.EBADF;
		this.view().setBigUint64(ptr, BigInt(f.pos), true);
		return ERRNO.SUCCESS;
	}
	fd_renumber(from, to) {
		const f = this.fds.get(from);
		if (!f || !this.fds.has(to)) return ERRNO.EBADF;
		this.fds.set(to, f);
		this.fds.delete(from);
		return ERRNO.SUCCESS;
	}

	fd_prestat_get(fd, ptr) {
		if (fd !== PREOPEN_FD) return ERRNO.EBADF;
		const view = this.view();
		view.setUint8(ptr, 0); // a directory
		view.setUint32(ptr + 4, 1, true); // length of "/"
		return ERRNO.SUCCESS;
	}
	fd_prestat_dir_name(fd, ptr, len) {
		if (fd !== PREOPEN_FD) return ERRNO.EBADF;
		this.bytes()[ptr] = 0x2f; // "/"
		return ERRNO.SUCCESS;
	}

	fd_readdir(fd, bufPtr, bufLen, cookie, usedPtr) {
		const f = this.fds.get(fd);
		if (!f || !f.node) return ERRNO.EBADF;
		if (f.node.type !== FILETYPE.DIRECTORY) return ERRNO.ENOTDIR;
		const entries = [".", ".."].concat(Array.from(f.node.children.keys()).sort());
		const bytes = this.bytes();
		const view = this.view();
		let used = 0;
		for (let i = Number(cookie); i < entries.length; i++) {
			const name = entries[i];
			const node = name === "." || name === ".." ? f.node : f.node.children.get(name);
			const nameBytes = this.encoder.encode(name);
			const entryLen = 24 + nameBytes.length;
			if (used + entryLen > bufLen) {
				// A partial entry tells the caller the buffer was too small
				// for this one, and to come back with the cookie.
				const room = bufLen - used;
				const tmp = new Uint8Array(entryLen);
				const tv = new DataView(tmp.buffer);
				tv.setBigUint64(0, BigInt(i + 1), true);
				tv.setBigUint64(8, BigInt(this.inode(node)), true);
				tv.setUint32(16, nameBytes.length, true);
				tv.setUint8(20, node.type);
				tmp.set(nameBytes, 24);
				bytes.set(tmp.subarray(0, room), bufPtr + used);
				used = bufLen;
				break;
			}
			view.setBigUint64(bufPtr + used, BigInt(i + 1), true);
			view.setBigUint64(bufPtr + used + 8, BigInt(this.inode(node)), true);
			view.setUint32(bufPtr + used + 16, nameBytes.length, true);
			view.setUint8(bufPtr + used + 20, node.type);
			bytes.set(nameBytes, bufPtr + used + 24);
			used += entryLen;
		}
		view.setUint32(usedPtr, used, true);
		return ERRNO.SUCCESS;
	}

	// Directory fds give relative paths their base.
	baseOf(fd) {
		const f = this.fds.get(fd);
		if (!f || !f.node) return null;
		if (f.node.type !== FILETYPE.DIRECTORY) return null;
		return f;
	}

	path_open(dirfd, dirflags, pathPtr, pathLen, oflags, rightsBase, rightsInheriting, fdflags, fdPtr) {
		const base = this.baseOf(dirfd);
		if (!base) return ERRNO.EBADF;
		const path = this.string(pathPtr, pathLen);
		const r = this.fs.resolveParent(base.names, path);
		if (r.errno) return r.errno;
		let node;
		let names;
		if (r.name === undefined) {
			node = this.fs.root;
			names = [];
		} else {
			node = r.dir.children.get(r.name);
			names = r.names.concat(r.name);
			if (node && (oflags & OFLAGS.CREAT) && (oflags & OFLAGS.EXCL)) return ERRNO.EEXIST;
			if (!node) {
				if (!(oflags & OFLAGS.CREAT)) return ERRNO.ENOENT;
				node = new File();
				r.dir.children.set(r.name, node);
			}
		}
		if ((oflags & OFLAGS.DIRECTORY) && node.type !== FILETYPE.DIRECTORY) return ERRNO.ENOTDIR;
		if ((oflags & OFLAGS.TRUNC) && node.type === FILETYPE.REGULAR_FILE) node.truncate(0);
		const fd = this.allocFD(new FD(node, names, fdflags));
		this.view().setUint32(fdPtr, fd, true);
		return ERRNO.SUCCESS;
	}
	path_filestat_get(dirfd, flags, pathPtr, pathLen, ptr) {
		const base = this.baseOf(dirfd);
		if (!base) return ERRNO.EBADF;
		const r = this.fs.lookup(base.names, this.string(pathPtr, pathLen));
		if (r.errno) return r.errno;
		this.writeFilestat(ptr, r.node);
		return ERRNO.SUCCESS;
	}
	path_create_directory(dirfd, pathPtr, pathLen) {
		const base = this.baseOf(dirfd);
		if (!base) return ERRNO.EBADF;
		const r = this.fs.resolveParent(base.names, this.string(pathPtr, pathLen));
		if (r.errno) return r.errno;
		if (r.name === undefined || r.dir.children.has(r.name)) return ERRNO.EEXIST;
		r.dir.children.set(r.name, new Dir());
		return ERRNO.SUCCESS;
	}
	path_remove_directory(dirfd, pathPtr, pathLen) {
		const base = this.baseOf(dirfd);
		if (!base) return ERRNO.EBADF;
		const r = this.fs.resolveParent(base.names, this.string(pathPtr, pathLen));
		if (r.errno) return r.errno;
		const node = r.name === undefined ? null : r.dir.children.get(r.name);
		if (!node) return ERRNO.ENOENT;
		if (node.type !== FILETYPE.DIRECTORY) return ERRNO.ENOTDIR;
		if (node.children.size > 0) return ERRNO.ENOTEMPTY;
		r.dir.children.delete(r.name);
		return ERRNO.SUCCESS;
	}
	path_unlink_file(dirfd, pathPtr, pathLen) {
		const base = this.baseOf(dirfd);
		if (!base) return ERRNO.EBADF;
		const r = this.fs.resolveParent(base.names, this.string(pathPtr, pathLen));
		if (r.errno) return r.errno;
		const node = r.name === undefined ? null : r.dir.children.get(r.name);
		if (!node) return ERRNO.ENOENT;
		if (node.type === FILETYPE.DIRECTORY) return ERRNO.EISDIR;
		r.dir.children.delete(r.name);
		return ERRNO.SUCCESS;
	}
	path_rename(oldfd, oldPtr, oldLen, newfd, newPtr, newLen) {
		const oldBase = this.baseOf(oldfd);
		const newBase = this.baseOf(newfd);
		if (!oldBase || !newBase) return ERRNO.EBADF;
		const from = this.fs.resolveParent(oldBase.names, this.string(oldPtr, oldLen));
		if (from.errno) return from.errno;
		const to = this.fs.resolveParent(newBase.names, this.string(newPtr, newLen));
		if (to.errno) return to.errno;
		if (from.name === undefined || to.name === undefined) return ERRNO.EINVAL;
		const node = from.dir.children.get(from.name);
		if (!node) return ERRNO.ENOENT;
		const existing = to.dir.children.get(to.name);
		if (existing && existing.type === FILETYPE.DIRECTORY && existing.children.size > 0) return ERRNO.ENOTEMPTY;
		from.dir.children.delete(from.name);
		to.dir.children.set(to.name, node);
		return ERRNO.SUCCESS;
	}

	poll_oneoff(inPtr, outPtr, nsubscriptions, neventsPtr) {
		// The only thing waited on is time passing, and since nothing else
		// can happen meanwhile, every subscription is reported ready.
		const view = this.view();
		for (let i = 0; i < nsubscriptions; i++) {
			const sub = inPtr + i * 48;
			const userdata = view.getBigUint64(sub, true);
			const type = view.getUint8(sub + 8);
			const ev = outPtr + i * 32;
			view.setBigUint64(ev, userdata, true);
			view.setUint16(ev + 8, 0, true);
			view.setUint8(ev + 10, type);
		}
		view.setUint32(neventsPtr, nsubscriptions, true);
		return ERRNO.SUCCESS;
	}
	proc_exit(code) {
		this.exitCode = code;
		throw new ExitError(code);
	}
	random_get(ptr, len) {
		const bytes = this.bytes().subarray(ptr, ptr + len);
		for (let i = 0; i < len; i += 65536) {
			crypto.getRandomValues(bytes.subarray(i, Math.min(i + 65536, len)));
		}
		return ERRNO.SUCCESS;
	}
}

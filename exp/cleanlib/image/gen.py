#!/usr/bin/env python3
# gen.py REPO OUTDIR EXPORTS [MAP [MAP2 EXE]]: writes the image target's link script, stub
# table and variable storage from the resident address table. Without MAP it
# lays everything out in one region, to learn the input sections' sizes; with
# the map of that link it packs them into the gaps between the entry points.
# An entry point the library lacks gets a jump to libps_image_missing, which
# parks with the entry's low address in t9. One the library exports as
# libps_va_NAME takes C's variable arguments: it goes through a routine that
# stores a0 to a3 in the home area the caller reserves above sp, and passes sp.
# One exported as libps_c_NAME jumps there: for a name the Go runtime takes, as
# malloc, or the compiler treats specially, as strlen. A routine in IN_SLOT sits
# at its entry point itself, with no jump, once the map gives its size.
# One in HEAD_IN_SLOT keeps its jump, with its first instruction in the jump's delay
# slot, once the packed link's map and flattened image, MAP2 and EXE, give that word.
import json, os, re, sys

repo, out, exports_file = sys.argv[1:4]
mapfile = sys.argv[4] if len(sys.argv) > 4 else None
T = os.path.join(repo, "internal/toolchain/_tinygo/targets")

BASE, TOP, INIT = 0x80010000, 0x80090000, 0x80010030

# A jump and its nop cost 4 cycles over the routine's own time, the nop 2 of them.
# Everything's time is set with those counted, so a routine goes in one of these
# only when it's called often and too slow by just that much. IN_SLOT also has to fit
# in the room before the next entry point.
IN_SLOT = {"GsGetActiveBuff", "GsGetWorkBase", "GetClut"}
HEAD_IN_SLOT = {"GsSetWorkBase", "GsInitCoordinate2", "GetTPage", "TransMatrix", "RotMatrix", "bcopy", "rand"}

# Every resident entry point a program's link reaches.
syms = {}
for line in open(os.path.join(T, "playstation-yaroze-symusr-orig.S")):
    m = re.match(r"\s*\.set\s+([A-Za-z_][A-Za-z0-9_]*)_orig,\s*(0x[0-9A-Fa-f]+)", line)
    if m:
        syms[m.group(1)] = int(m.group(2), 16)

exports = set(open(exports_file).read().split())
# The C routines of the target's own C library.
libc = set(re.findall(r"(?m)^([a-z_][a-z0-9_]*)\(", open(os.path.join(T, "playstation-libc.c")).read()))
# Hand-written assembly entries, exp/cleanlib/image/clib.S.
clib = open(os.path.join(repo, "exp/cleanlib/image/clib.S")).read()
libc |= set(re.findall(r"(?m)^\s*\.globl\s+([A-Za-z_][A-Za-z0-9_]*)", clib))
cvars = {}
for m in re.finditer(r"VAR\((\w+),\s*(\d+)\)", open(os.path.join(T, "playstation-yaroze-libps-data.c")).read()):
    cvars[m.group(1)] = int(m.group(2))

# The size of each routine in IN_SLOT, from the first link's map.
slot_size = {}
if mapfile:
    for line in open(mapfile):
        m = re.match(r"\s*[0-9a-f]+\s+[0-9a-f]+\s+([0-9a-f]+)\s+\d+\s+\S+:\(\.text\.(\S+)\)$", line)
        if m and m.group(2) in IN_SLOT:
            slot_size[m.group(2)] = int(m.group(1), 16)
    assert set(slot_size) == IN_SLOT, IN_SLOT - set(slot_size)

# The first instruction of each routine in HEAD_IN_SLOT.
head = {}
if len(sys.argv) > 6:
    want = {}
    for line in open(sys.argv[5]):
        f = line.split()
        if len(f) == 5 and f[4] in HEAD_IN_SLOT and f[4] not in want:
            want[f[4]] = int(f[0], 16)
    assert set(want) == HEAD_IN_SLOT, HEAD_IN_SLOT - set(want)
    exe = open(sys.argv[6], "rb").read()
    origin = int.from_bytes(exe[0x18:0x1c], "little")
    for name, at in want.items():
        word = int.from_bytes(exe[at - origin + 0x800:][:4], "little")
        op, funct = word >> 26, word & 63
        # Branches and jumps can't go in a delay slot.
        assert not (1 <= op <= 7 or op == 0 and funct in (8, 9)), (name, hex(word))
        head[name] = word

fixed = [("init", INIT, 8, "code")]
missing = []
for name, addr in sorted(syms.items(), key=lambda kv: kv[1]):
    if name in cvars:
        fixed.append((name, addr, cvars[name], "var"))
    else:
        fixed.append((name, addr, slot_size.get(name, 8), "code"))
        if not {name, "libps_va_" + name, "libps_c_" + name} & exports and name not in libc:
            missing.append(name)
fixed.sort(key=lambda f: f[1])
for a, b in zip(fixed, fixed[1:]):
    assert a[1] + a[2] <= b[1], (a, b)

mem, secs, gaps = [], [], []
cur = BASE
for i, (name, addr, size, kind) in enumerate(fixed):
    if addr > cur:
        gaps.append((cur, addr - cur))
    mem.append(f"  f{i} (rwx) : ORIGIN = 0x{addr:08X}, LENGTH = {size}")
    if kind == "var":
        secs.append(f"  .fix.{name} : {{ KEEP(*(.bss.{name})) }} > f{i}")
    elif name in slot_size:
        secs.append(f"  .fix.{name} : {{ KEEP(*(.text.{name})) }} > f{i}")
    else:
        secs.append(f"  .fix.{name} : {{ KEEP(*(.libps_stub.{name})) }} > f{i}")
    cur = addr + size
gaps.append((cur, TOP - cur))

if mapfile is None:
    mem.append("  big (rwx) : ORIGIN = 0x80100000, LENGTH = 0x100000")
    pat = "*(.text .text.*) *(.rodata .rodata.* .rdata .rdata.*) *(.data .data.* .sdata .sdata.*) *(.sbss .sbss.* .bss .bss.* COMMON .scommon)"
    secs.append(f"  .all : {{ {pat} }} > big")
    secs.append("  .got : { *(.got) } > big")
else:
    insec = []
    for line in open(mapfile):
        m = re.match(r"\s*([0-9a-f]+)\s+[0-9a-f]+\s+([0-9a-f]+)\s+(\d+)\s+\S+:\((\S.*)\)$", line)
        if not m:
            continue
        name, size, align = m.group(4), int(m.group(2), 16), int(m.group(3))
        if name.startswith(".libps_stub.") or (name.startswith(".bss.") and name[len(".bss."):] in cvars):
            continue
        if name.startswith(".text.") and name[len(".text."):] in slot_size:
            continue
        if not name.startswith((".text", ".rodata", ".rdata", ".data", ".sdata", ".sbss", ".bss", "COMMON", ".scommon")):
            continue
        if size:
            insec.append((name, size, max(align, 1)))
    # Sections of one name, a static function in several C files, match one
    # pattern, so they are placed as one, end to end.
    grouped = {}
    for name, size, align in insec:
        if name in grouped:
            g = grouped[name]
            at = (g[1] + align - 1) & -align
            grouped[name] = (name, at + size, max(g[2], align))
        else:
            grouped[name] = (name, size, align)
    insec = list(grouped.values())
    # Code first in each gap and data after it, in output sections of their
    # own, so that only code is in an executable section: mips1fix rewrites
    # anything there that reads as a trap, data included.
    free = [[a, a + n, [], []] for a, n in gaps]
    def end(origin, text, data):
        # Each output section starts at the largest alignment among its
        # sections, and they follow in order.
        at = origin
        for items in (text, data):
            if not items:
                continue
            at = (at + max(x[2] for x in items) - 1) & -max(x[2] for x in items)
            for _, size, align in items:
                at = (at + align - 1) & -align
                at += size
        return at
    def place(items, slot):
        unplaced = []
        for item in sorted(items, key=lambda t: -t[1]):
            for g, (a, n) in zip(free, gaps):
                trial = g[slot] + [item]
                text, data = (trial, g[3]) if slot == 2 else (g[2], trial)
                if end(a, text, data) <= a + n:
                    g[slot].append(item)
                    g[0] = end(a, g[2], g[3])
                    break
            else:
                unplaced.append((item[0], item[1]))
        return unplaced
    code = [x for x in insec if x[0].startswith(".text")]
    data = [x for x in insec if not x[0].startswith(".text")]
    unplaced = place(code, 2) + place(data, 3)
    if unplaced:
        print("UNPLACED", sum(s for _, s in unplaced), unplaced[:5])
    def glob(n):
        return "".join("?" if c in "()*[]? \"" else c for c in n)
    used = sum(g[0] - a for g, (a, n) in zip(free, gaps))
    print(f"packed {len(insec)} sections, {used} bytes with padding, into {sum(1 for g in free if g[2] or g[3])} gaps")
    last = len(gaps) - 1
    for j, ((a, n), g) in enumerate(zip(gaps, free)):
        if not g[2] and not g[3] and j != last:
            continue
        mem.append(f"  g{j} (rwx) : ORIGIN = 0x{a:08X}, LENGTH = {n}")
        text = " ".join(f"*({glob(x[0])})" for x in g[2])
        rest = " ".join(f"*({glob(x[0])})" for x in g[3])
        if j == last:
            text += " *(.text .text.*)"
            rest += " *(.rodata .rodata.* .rdata .rdata.*) *(.data .data.* .sdata .sdata.*) *(.sbss .sbss.* .bss .bss.* COMMON .scommon)"
        if text.strip():
            secs.append(f"  .gap{j} : {{ {text} }} > g{j}")
        if rest.strip():
            secs.append(f"  .gap{j}d : {{ {rest} }} > g{j}")
    secs.append(f"  .got : {{ *(.got) }} > g{last}")

ld = ["/* Generated by gen.py: the clean library laid out as the resident image. */",
      "ENTRY(libps_image_entry)", "MEMORY", "{", *mem, "}", "SECTIONS", "{", *secs,
      "  _heap_start = libps_image_heap;", "  _heap_end = libps_image_heap + 0x8000;",
      "  _libps_heap_end = _heap_start;", "  _globals_start = 0x80010000;", "  _globals_end = 0x80090000;",
      "  _stack_top = 0x801fff00;",
      "  /DISCARD/ : { *(.reginfo) *(.MIPS.abiflags) *(.MIPS.options) *(.gnu.attributes) *(.pdr) *(.comment) *(.note) *(.note.*) *(.eh_frame) *(.eh_frame_hdr) *(.mdebug.*) }",
      "}"]
open(os.path.join(out, "playstation-yaroze-cleanimage.ld"), "w").write("\n".join(ld) + "\n")

S = ["# Generated by gen.py: a jump at each of the resident library's entry points.",
     "\t.set noreorder", "\t.set noat",
     "\t.section .libps_stub.init,\"ax\",@progbits", "\t.globl libps_image_entry", "libps_image_entry:",
     "\tj libps_image_init", "\tnop"]
variadic = []
for name, addr, size, kind in fixed:
    if kind != "code" or name == "init" or name in slot_size:
        continue
    if name in missing:
        S += [f"\t.section .libps_stub.{name},\"ax\",@progbits", "\tj libps_image_missing", f"\tli $t9, 0x{addr & 0xffff:04x}"]
    elif "libps_va_" + name in exports:
        S += [f"\t.section .libps_stub.{name},\"ax\",@progbits", f"\tj libps_va_entry_{name}", "\tnop"]
        variadic.append(name)
    elif "libps_c_" + name in exports:
        S += [f"\t.section .libps_stub.{name},\"ax\",@progbits", f"\tj libps_c_{name}", "\tnop"]
    elif name in head:
        S += [f"\t.section .libps_stub.{name},\"ax\",@progbits", f"\tj {name}+4", f"\t.word 0x{head[name]:08x}"]
    else:
        S += [f"\t.section .libps_stub.{name},\"ax\",@progbits", f"\tj {name}", "\tnop"]
# A jump is what keeps a routine through the link's LTO, so one without a jump is
# named here instead, in a section the link script discards.
if slot_size:
    S += ["\t.section .note.libps_keep,\"\",@progbits"] + [f"\t.word {name}" for name in sorted(slot_size)]
S += ["", "\t.text",
      "# The init routine: starts the Go runtime and the library, and returns to",
      "# the program's start-up.",
      "\t.globl libps_image_init", "libps_image_init:",
      "\taddiu $sp, $sp, -24", "\tsw $ra, 16($sp)",
      "\tjal libps_image_start", "\tnop",
      "\tjal libps_image_setup", "\tnop",
      "\tlw $ra, 16($sp)", "\tnop", "\tjr $ra", "\taddiu $sp, $sp, 24",
      "\t.globl libps_image_missing", "libps_image_missing:",
      "\tlui $t8, 0xdead", "1:\tb 1b", "\tnop"]
for name in variadic:
    S += [f"libps_va_entry_{name}:", "\tsw $a0, 0($sp)", "\tsw $a1, 4($sp)", "\tsw $a2, 8($sp)", "\tsw $a3, 12($sp)",
          f"\tj libps_va_{name}", "\tmove $a0, $sp"]
open(os.path.join(out, "playstation-yaroze-cleanimage.S"), "w").write("\n".join(S) + "\n")

open(os.path.join(out, "playstation-yaroze-cleanimage.c"), "w").write(
    "/* Generated by gen.py: the Go runtime's heap. */\n"
    "unsigned char libps_image_heap[0x8000] __attribute__((aligned(8)));\n")

json.dump({
    "comment": "exp/cleanlib laid out as the resident library's image: a jump at each of its entry points and its variables at their addresses. Generated by exp/cleanlib/image/gen.py.",
    "inherits": ["r3000"],
    "build-tags": ["yaroze", "cleanlib", "fidelity", "cleanimage"],
    "cflags": ["-DLIBPS_GO", "-DLIBPS_CLEAN", "-DLIBPS_FIDELITY"],
    "linkerscript": "targets/playstation-yaroze-cleanimage.ld",
    "ldflags": ["-Map=/sysroot/map.txt"],
    "extra-files": [
        "targets/playstation-yaroze-cleanimage.S",
        "targets/playstation-yaroze-cleanimage.c",
        "targets/playstation-yaroze-cleanimage-clib.S",
        "targets/playstation-yaroze-bios.S",
        "targets/playstation-yaroze-gte.S",
        "targets/r3000-builtins.c",
        "targets/playstation-libc.c",
        "targets/playstation-yaroze-libps-data.c",
    ],
}, open(os.path.join(out, "playstation-yaroze-cleanimage.json"), "w"), indent="\t")
open(os.path.join(out, "missing.txt"), "w").write("\n".join(missing) + "\n")
print(f"{len(fixed)} fixed, {len(gaps)} gaps, missing {len(missing)}: {' '.join(missing)}")

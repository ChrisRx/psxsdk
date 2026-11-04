#!/usr/bin/env python3
# gen.py REPO OUTDIR: writes the prototype image target's link script,
# stub table and variable storage from the resident address table.
import re, sys, os
repo, out = sys.argv[1], sys.argv[2]
T = os.path.join(repo, "internal/toolchain/_tinygo/targets")
syms = {}
# symusr-orig.S has every resident entry point a program's link reaches,
# symusr.S's and the ones the link script and the linked members add,
# with _orig on their names.
for line in open(os.path.join(T, "playstation-yaroze-symusr-orig.S")):
    m = re.match(r"\s*\.set\s+([A-Za-z_][A-Za-z0-9_]*)_orig,\s*(0x[0-9A-Fa-f]+)", line)
    if m:
        syms[m.group(1)] = int(m.group(2), 16)
exports = set(open(sys.argv[3]).read().split())   # Go exports
cvars = {}
for m in re.finditer(r"VAR\((\w+),\s*(\d+)\)", open(os.path.join(T, "playstation-yaroze-libps-data.c")).read()):
    cvars[m.group(1)] = int(m.group(2))
# math_errno and the word after it, which _err_math writes.
extra_vars = {"math_errno": 8}
clib = {"memset", "memcpy", "memmove", "memcmp", "strlen", "bzero", "bcopy"}
# Entry points served by the image's C rather than the Go library's export.
override = {n: "libps_c_" + n for n in ["InitHeap", "malloc", "free", "calloc", "realloc", "FntPrint", "KanjiFntPrint"]}
clib |= set(override)
# What libc.c defines.
clib |= set("LoadTest Load Exec FlushCache setjmp longjmp putc getc getchar gets _dbl_shift _err_math _get_errno atof strtod printf sprintf printf2 sprintf2 strcpy strncpy strcat strncat strcmp strncmp strchr strrchr strstr strspn strcspn strpbrk strtok memchr bcmp toupper tolower strtoul strtol atoi atol qsort bsearch sqrt floor".split())
# The soft-float routines, each from compiler-rt's file of the same name
# (comparedf2 has the comparisons): the same names and convention as
# libgcc's, which the SDK's compiler called.
RT = {
    "adddf3": ["__adddf3"], "subdf3": ["__subdf3"], "muldf3": ["__muldf3"], "divdf3": ["__divdf3"],
    "fixdfsi": ["__fixdfsi"], "fixunsdfsi": ["__fixunsdfsi"], "floatsidf": ["__floatsidf"],
    "floatunsidf": ["__floatunsidf"], "extendsfdf2": ["__extendsfdf2"], "truncdfsf2": ["__truncdfsf2"],
    "addsf3": ["__addsf3"], "subsf3": ["__subsf3"], "mulsf3": ["__mulsf3"], "divsf3": ["__divsf3"],
    "negdf2": ["__negdf2"], "fp_mode": [], "floatundidf": [], "floatdidf": [],
    "comparedf2": ["__eqdf2", "__nedf2", "__ltdf2", "__ledf2", "__gtdf2", "__gedf2"],
}
for fns in RT.values():
    clib |= set(fns)
# The math library, each from musl's file of the same name, with what it
# defines that programs call: the rest are the helpers they share.
LIBM = {
    "sin": ["sin"], "cos": ["cos"], "tan": ["tan"], "asin": ["asin"], "acos": ["acos"],
    "atan": ["atan"], "atan2": ["atan2"], "exp": ["exp"], "log": ["log"], "log10": ["log10"],
    "pow": ["pow"], "sinh": ["sinh"], "cosh": ["cosh"], "tanh": ["tanh"], "hypot": ["hypot"],
    "ldexp": ["ldexp"], "frexp": ["frexp"], "modf": ["modf"], "fmod": ["fmod"], "ceil": ["ceil"],
    "__sin": [], "__cos": [], "__tan": [], "__rem_pio2": [], "__rem_pio2_large": [],
    "exp_data": [], "log_data": [], "pow_data": [], "__expo2": [], "expm1": [], "scalbn": [],
    "fabs": [], "__math_oflow": [], "__math_uflow": [], "__math_xflow": [], "__math_invalid": [],
    "__math_divzero": [], "__fpclassify": [],
}
for fns in LIBM.values():
    clib |= set(fns)
BASE, TOP, INIT = 0x80010000, 0x80090000, 0x80010030

fixed = [("init", INIT, 8, "code")]
missing = []
for name, addr in sorted(syms.items(), key=lambda kv: kv[1]):
    if name in cvars:
        fixed.append((name, addr, cvars[name], "var"))
    elif name in extra_vars:
        fixed.append((name, addr, extra_vars[name], "xvar"))
    else:
        fixed.append((name, addr, 8, "code"))
        if name not in exports and name not in clib:
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
    elif kind == "xvar":
        secs.append(f"  .fix.{name} : {{ KEEP(*(.libps_var.{name})) }} > f{i}")
    else:
        secs.append(f"  .fix.{name} : {{ KEEP(*(.libps_stub.{name})) }} > f{i}")
    cur = addr + size
gaps.append((cur, TOP - cur))
pat = "*(.text .text.*) *(.rodata .rodata.* .rdata .rdata.*) *(.data .data.* .sdata .sdata.*) *(.sbss .sbss.* .bss .bss.* COMMON .scommon)"
mapfile = sys.argv[4] if len(sys.argv) > 4 else None
if mapfile is None:
    # Pass 1: everything in one region above the program area, only to
    # learn the input sections' sizes.
    mem.append("  big (rwx) : ORIGIN = 0x80100000, LENGTH = 0x100000")
    secs.append(f"  .all : {{ {pat} }} > big")
    secs.append("  .got : { *(.got) } > big")
else:
    # Pass 2: each input section put into a gap, first fit, largest first.
    insec = []
    for line in open(mapfile):
        m = re.match(r"\s*([0-9a-f]+)\s+[0-9a-f]+\s+([0-9a-f]+)\s+(\d+)\s+\S+:\((\S.*)\)$", line)
        if not m:
            continue
        name, size, align = m.group(4), int(m.group(2), 16), int(m.group(3))
        if name.startswith((".libps_stub.", ".libps_var.")) or name[len(".bss."):] in cvars and name.startswith(".bss."):
            continue
        if not name.startswith((".text", ".rodata", ".rdata", ".data", ".sdata", ".sbss", ".bss", "COMMON", ".scommon")):
            continue
        if size == 0:
            continue
        insec.append((name, size, max(align, 1)))
    # Sections of the same name, a static function in several C files,
    # match the same pattern, so they are placed as one, end to end.
    grouped = {}
    for name, size, align in insec:
        if name in grouped:
            g = grouped[name]
            at = (g[1] + align - 1) & -align
            grouped[name] = (name, at + size, max(g[2], align))
        else:
            grouped[name] = (name, size, align)
    insec = list(grouped.values())
    # Code first in each gap and data after it, in output sections of
    # their own, so that only code is in an executable section: mips1fix
    # rewrites anything there that reads as a trap, data included.
    free = [[a, a + n, [], []] for a, n in gaps]
    def end(origin, text, data):
        # Where a gap's layout ends: each output section starts at the
        # largest alignment among its sections, and they follow in order.
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
ld = ["/* Generated by gen.py: the Go library laid out as the resident image. */",
      "ENTRY(libps_image_entry)", "MEMORY", "{", *mem, "}", "SECTIONS", "{", *secs,
      "  _heap_start = libps_image_heap;", "  _heap_end = libps_image_heap + 0x8000;",
      "  _libps_heap_end = _heap_start;", "  _globals_start = 0x80010000;", "  _globals_end = 0x80090000;",
      "  _stack_top = 0x801fff00;",
      "  /DISCARD/ : { *(.reginfo) *(.MIPS.abiflags) *(.MIPS.options) *(.gnu.attributes) *(.pdr) *(.comment) *(.note) *(.note.*) *(.eh_frame) *(.eh_frame_hdr) *(.mdebug.*) }",
      "}"]
open(os.path.join(out, "playstation-yaroze-image.ld"), "w").write("\n".join(ld) + "\n")

S = ["# Generated by gen.py: a jump at each of the resident library's entry points.",
     "\t.set noreorder", "\t.set noat"]
S += ["\t.section .libps_stub.init,\"ax\",@progbits", "\t.globl libps_image_entry", "libps_image_entry:",
      "\tj libps_image_init", "\tnop"]
for name, addr, size, kind in fixed:
    if kind != "code" or name == "init":
        continue
    target = override.get(name, name) if name not in missing else "libps_image_missing"
    S += [f"\t.section .libps_stub.{name},\"ax\",@progbits", f"\tj {target}",
          f"\tli $t9, 0x{addr & 0xffff:04x}" if name in missing else "\tnop"]
S += ["", "\t.text",
      "# libps_image_init is the resident library's init routine: it starts the",
      "# Go runtime and the library, and returns to the program's startup stub.",
      "\t.globl libps_image_init", "libps_image_init:",
      "\taddiu $sp, $sp, -24", "\tsw $ra, 16($sp)",
      "\tjal libps_image_start", "\tnop",
      "\tjal libps_image_setup", "\tnop",
      "\tlw $ra, 16($sp)", "\tnop", "\tjr $ra", "\taddiu $sp, $sp, 24",
      "# The kernel's executable calls, A-functions 41h to 44h, and its read",
      "# and write, B-functions 34h and 35h, as the originals reached them:",
      "# the arguments go through as they are.",
      "\t.globl LoadTest", "LoadTest:", "\tli $t2, 0xa0", "\tjr $t2", "\tli $t1, 0x41",
      "\t.globl Load", "Load:", "\tli $t2, 0xa0", "\tjr $t2", "\tli $t1, 0x42",
      "\t.globl Exec", "Exec:", "\tli $t2, 0xa0", "\tjr $t2", "\tli $t1, 0x43",
      "\t.globl FlushCache", "FlushCache:", "\tli $t2, 0xa0", "\tjr $t2", "\tli $t1, 0x44",
      "\t.globl libps_kread", "libps_kread:", "\tli $t2, 0xb0", "\tjr $t2", "\tli $t1, 0x34",
      "\t.globl libps_kwrite", "libps_kwrite:", "\tli $t2, 0xb0", "\tjr $t2", "\tli $t1, 0x35",
      "# setjmp keeps ra, sp, fp, s0 to s7 and gp, a word each in that order,",
      "# and returns 0; longjmp puts them back and returns its second argument",
      "# where setjmp returned, 0 included. The originals were probed for these.",
      "\t.globl setjmp", "setjmp:",
      "\tsw $ra, 0($a0)", "\tsw $sp, 4($a0)", "\tsw $fp, 8($a0)",
      "\tsw $s0, 12($a0)", "\tsw $s1, 16($a0)", "\tsw $s2, 20($a0)", "\tsw $s3, 24($a0)",
      "\tsw $s4, 28($a0)", "\tsw $s5, 32($a0)", "\tsw $s6, 36($a0)", "\tsw $s7, 40($a0)",
      "\tsw $gp, 44($a0)", "\tjr $ra", "\tmove $v0, $zero",
      "\t.globl longjmp", "longjmp:",
      "\tlw $ra, 0($a0)", "\tlw $sp, 4($a0)", "\tlw $fp, 8($a0)",
      "\tlw $s0, 12($a0)", "\tlw $s1, 16($a0)", "\tlw $s2, 20($a0)", "\tlw $s3, 24($a0)",
      "\tlw $s4, 28($a0)", "\tlw $s5, 32($a0)", "\tlw $s6, 36($a0)", "\tlw $s7, 40($a0)",
      "\tlw $gp, 44($a0)", "\tjr $ra", "\tmove $v0, $a1",
      "# _get_errno is the kernel's B-function 54h, as the original's was.",
      "\t.globl _get_errno", "_get_errno:", "\tli $t2, 0xb0", "\tjr $t2", "\tli $t1, 0x54",
      "# A function the Go library does not have: $t9 holds the low half of",
      "# its resident address.",
      "\t.globl libps_image_missing", "libps_image_missing:",
      "\tlui $t8, 0xdead", "1:\tb 1b", "\tnop"]
open(os.path.join(out, "playstation-yaroze-image.S"), "w").write("\n".join(S) + "\n")

C = ["/* Generated by gen.py: the Go library's heap and the C routines the image adds. */"]
for n, s in extra_vars.items():
    C.append(f"unsigned char {n}[{s}] __attribute__((used, aligned(4), section(\".libps_var.{n}\")));")
C.append("unsigned char libps_image_heap[0x8000] __attribute__((aligned(8)));")
C += ["",
"/* The C library routines programs call that the Go library does not have. */",
"typedef unsigned long size_t;",
"void *memset(void *, int, size_t);",
"void *memmove(void *, const void *, size_t);",
"__attribute__((used)) void bzero(void *p, size_t n) { memset(p, 0, n); }",
"__attribute__((used)) void bcopy(const void *src, void *dst, size_t n) { memmove(dst, src, n); }",
""]
C.append(open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "libc.c")).read())
C.append(open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "malloc.c")).read())
open(os.path.join(out, "playstation-yaroze-image.c"), "w").write("\n".join(C) + "\n")
print(f"{len(fixed)} fixed, {len(gaps)} gaps, missing {len(missing)}: {' '.join(missing)}")

# A file for each compiler-rt routine, which keeps what it defines through
# the link, and the target, which compiles them.
import json
extra = []
for f, fns in RT.items():
    name = f"playstation-yaroze-image-rt-{f}.c"
    with open(os.path.join(out, name), "w") as w:
        if f == "comparedf2":
            # r3000-builtins.c has __unorddf2 already.
            w.write("#define __unorddf2 libps_rt_unorddf2\n")
        w.write(f'#include "../lib/compiler-rt-builtins/{f}.c"\n')
        if fns:
            w.write("__attribute__((used)) static void *keep[] = {" + ", ".join(f"(void *){x}" for x in fns) + "};\n")
    extra.append("targets/" + name)
for f, fns in LIBM.items():
    name = f"playstation-yaroze-image-m-{f}.c"
    with open(os.path.join(out, name), "w") as w:
        w.write(f'#include "../lib/musl-math/{f}.c"\n')
        if fns:
            w.write("__attribute__((used)) static void *keep[] = {" + ", ".join(f"(void *){x}" for x in fns) + "};\n")
    extra.append("targets/" + name)
t = json.load(open(os.path.join(os.path.dirname(os.path.abspath(__file__)), "playstation-yaroze-image.json")))
t["extra-files"] = t["extra-files"] + extra
json.dump(t, open(os.path.join(out, "playstation-yaroze-image.json"), "w"), indent="\t")

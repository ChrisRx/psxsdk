#!/usr/bin/env python3
# archive.py SITE OUT: finds the Net Yaroze programs in the web archive that
# come with an AUTO batch file for siocons, and writes OUT/programs.tsv: a
# name, the batch's directory, the executable, and its --load arguments.
import hashlib, os, re, sys

site, out = sys.argv[1], sys.argv[2]


def find(root, lower, name):
    n = name.replace("\\", "/").lower()
    for cand in (n, n + ".exe"):
        if cand in lower:
            return os.path.join(root, lower[cand])
        for dirpath, _, files in os.walk(root):
            for x in files:
                if os.path.relpath(os.path.join(dirpath, x), root).lower() == cand:
                    return os.path.join(dirpath, x)
    return None


seen, rows = set(), []
for root, dirs, files in os.walk(site):
    lower = {f.lower(): f for f in files}
    batch = lower.get("auto") or lower.get("auto.bat")
    if not batch:
        continue
    exe, loads = None, []
    for line in open(os.path.join(root, batch), errors="replace"):
        f = line.strip().split()
        if len(f) >= 3 and f[0].lower() == "local" and f[1].lower() == "load":
            exe = f[2]
        elif len(f) >= 4 and f[0].lower() == "local" and f[1].lower() == "dload":
            loads.append((f[2], f[3]))
    if not exe:
        continue
    path = find(root, lower, exe)
    if not path or open(path, "rb").read(2) != b"\x62\x01":
        continue
    args = [(find(root, lower, f), a) for f, a in loads]
    if not all(p for p, _ in args):
        continue
    h = hashlib.sha1(open(path, "rb").read()).hexdigest()
    if h in seen:
        continue
    seen.add(h)
    name = re.sub(r"[^a-z0-9]+", "-", os.path.relpath(root, site).lower()).strip("-")
    for cut in ("yaroze-scei-forum-program-archive-", "-lzh-x", "-zip-x", "yaroze-scea-", "yaroze-ftp-"):
        name = name.replace(cut, "")
    rows.append((name, root, path, " ".join(f"{p}@{a}" for p, a in args)))
with open(os.path.join(out, "programs.tsv"), "w") as w:
    for r in rows:
        w.write("\t".join(r) + "\n")
print(len(rows), "programs")

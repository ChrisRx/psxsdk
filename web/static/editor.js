// The playground's code editor, for one Go file. The typing happens in a
// textarea whose own text is transparent, laid exactly over a highlighted
// copy of it, with the line numbers beside both; all three are sized to the
// text and scroll together inside one box, so nothing has to be kept in step
// but the copy's contents.
//
// It offers the yaroze package's names as they are typed, from yaroze.json,
// which web/site writes from the package's declarations: after the package's
// name and a dot, and after a variable declared with one of its struct types,
// that type's fields and methods.
"use strict";

const GO_KEYWORDS = new Set([
  "break", "case", "chan", "const", "continue", "default", "defer", "else", "fallthrough",
  "for", "func", "go", "goto", "if", "import", "interface", "map", "package", "range",
  "return", "select", "struct", "switch", "type", "var",
]);
const GO_TYPES = new Set([
  "any", "bool", "byte", "comparable", "complex64", "complex128", "error", "float32", "float64",
  "int", "int8", "int16", "int32", "int64", "rune", "string",
  "uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
]);
const GO_BUILTINS = new Set([
  "append", "cap", "clear", "close", "complex", "copy", "delete", "imag", "len", "make", "max",
  "min", "new", "panic", "print", "println", "real", "recover",
  "true", "false", "nil", "iota",
]);

// One token per match: a comment, a string or rune, a number, an
// identifier, or anything else, a run of which is left as it is.
const GO_TOKEN = /(\/\/[^\n]*|\/\*[\s\S]*?(?:\*\/|$))|(`[^`]*`?|"(?:[^"\\\n]|\\.)*"?|'(?:[^'\\\n]|\\.)*'?)|(\b(?:0[xX][\da-fA-F_]+|0[bB][01_]+|0[oO]?[0-7_]+|\d[\d_]*(?:\.[\d_]*)?(?:[eE][+-]?\d+)?)i?\b)|([A-Za-z_][\w]*)|([^\w"'`/]+|\/)/g;

const TAB = 4; // columns, as tab-size in style.css

function escapeHTML(s) {
  return s.replace(/[&<>]/g, (c) => (c === "&" ? "&amp;" : c === "<" ? "&lt;" : "&gt;"));
}

// goImports returns the packages a source imports, by the name it refers
// to each with: its alias, or the last element of its path.
function goImports(src) {
  const names = new Map();
  for (const m of src.matchAll(/^import\s*(?:\(([^)]*)\)|(.*))/gm)) {
    for (const line of (m[1] ?? m[2]).split("\n")) {
      const spec = line.match(/^\s*([\w.]+\s+)?"([^"]+)"/);
      if (!spec) continue;
      const name = spec[1]?.trim() ?? spec[2].split("/").pop();
      if (name !== "_" && name !== ".") names.set(name, spec[2]);
    }
  }
  return names;
}

class GoEditor {
  // root: the element to fill; options: {label, onRun, onChange}.
  constructor(root, options = {}) {
    this.root = root;
    this.options = options;
    this.api = null;     // {entries, byName}, once yaroze.json is fetched
    this.errors = new Map(); // line (from 1) to message
    this.lines = 0;
    this.popup = null;   // the completion list, while it is open
    this.inserting = false; // the editor's own change is under way

    root.classList.add("editor");
    root.innerHTML = `
      <div class="ed-scroll">
        <div class="ed-sizer">
          <pre class="ed-gutter" aria-hidden="true"></pre>
          <div class="ed-code">
            <pre class="ed-hl" aria-hidden="true"></pre>
            <textarea spellcheck="false" autocapitalize="off" autocomplete="off" autocorrect="off" wrap="off"></textarea>
          </div>
        </div>
      </div>
      <div class="ed-complete" hidden>
        <ul role="listbox" aria-label="Completions"></ul>
        <div class="ed-doc"><code></code><p></p></div>
      </div>`;
    this.scroller = root.querySelector(".ed-scroll");
    this.gutter = root.querySelector(".ed-gutter");
    this.hl = root.querySelector(".ed-hl");
    this.ta = root.querySelector("textarea");
    this.box = root.querySelector(".ed-complete");
    this.list = this.box.querySelector("ul");
    this.ta.setAttribute("aria-label", options.label ?? "Source");

    this.ta.addEventListener("input", (e) => this.onInput(e));
    this.ta.addEventListener("keydown", (e) => this.onKey(e));
    this.ta.addEventListener("keyup", (e) => {
      if (e.key.startsWith("Arrow") || e.key === "Home" || e.key === "End" || e.key.startsWith("Page")) this.reveal();
    });
    this.ta.addEventListener("blur", () => this.close());
    // The textarea is the size of its text, so it never scrolls itself;
    // should the browser scroll it to show the caret, the copy under it
    // would no longer line up.
    this.ta.addEventListener("scroll", () => {
      this.ta.scrollTop = 0;
      this.ta.scrollLeft = 0;
    });
    this.ta.addEventListener("pointerdown", () => this.close());
    this.scroller.addEventListener("scroll", () => this.close());
    // A press on the list must not take the focus from the textarea.
    this.box.addEventListener("pointerdown", (e) => e.preventDefault());
    this.list.addEventListener("click", (e) => {
      const li = e.target.closest("li");
      if (li) this.accept(Number(li.dataset.index));
    });
  }

  get value() { return this.ta.value; }
  set value(text) {
    this.ta.value = text;
    this.ta.setSelectionRange(0, 0);
    this.errors.clear();
    this.render();
  }

  focus() { this.ta.focus(); }

  // replaceText replaces the whole text as an edit, which undo takes back,
  // as formatting does, and keeps the caret where it was in the code: on
  // the same line, after as many characters other than spaces and tabs,
  // which formatting leaves alone.
  replaceText(text) {
    const old = this.ta.value;
    if (text === old) return;
    const at = this.ta.selectionStart;
    const before = old.slice(0, at);
    const line = before.split("\n").length - 1;
    const seen = before.slice(before.lastIndexOf("\n") + 1).replace(/[ \t]/g, "").length;
    const lines = text.split("\n");
    const target = lines[Math.min(line, lines.length - 1)];
    let col = 0;
    for (let n = 0; col < target.length && n < seen; col++) {
      if (target[col] !== " " && target[col] !== "\t") n++;
    }
    let caret = col;
    for (let i = 0; i < Math.min(line, lines.length - 1); i++) caret += lines[i].length + 1;
    const hadFocus = document.activeElement === this.ta;
    const top = this.scroller.scrollTop;
    // The browser's editing, which keeps undo, only edits what has the
    // focus.
    this.ta.focus({ preventScroll: true });
    this.insert(text, 0, old.length, caret);
    this.scroller.scrollTop = top;
    if (!hadFocus) this.ta.blur();
  }

  // loadAPI fetches the completion index; without it, the editor only
  // highlights.
  async loadAPI(url) {
    try {
      const res = await fetch(url);
      if (!res.ok) return;
      const entries = await res.json();
      this.api = { entries, byName: new Map(entries.map((e) => [e.name, e])) };
      this.render();
    } catch { /* no completion */ }
  }

  // render redraws the highlighted copy and the line numbers.
  render() {
    const src = this.ta.value;
    // A last empty line has no height in a pre unless something follows it.
    this.hl.innerHTML = this.highlight(src) + "\n";
    const lines = src.split("\n").length;
    if (lines !== this.lines || this.errors.size) {
      this.lines = lines;
      let html = "";
      for (let i = 1; i <= lines; i++) {
        const err = this.errors.get(i);
        html += err ? `<span class="err" title="${escapeHTML(err).replace(/"/g, "&quot;")}">${i}</span>\n` : `${i}\n`;
      }
      this.gutter.innerHTML = html;
    }
  }

  highlight(src) {
    const imports = goImports(src);
    const yaroze = this.yarozeName(imports);
    let out = "";
    let last = 0;
    for (const m of src.matchAll(GO_TOKEN)) {
      if (m.index > last) out += escapeHTML(src.slice(last, m.index));
      last = m.index + m[0].length;
      const text = escapeHTML(m[0]);
      let cls = "";
      if (m[1]) cls = "com";
      else if (m[2]) cls = "str";
      else if (m[3]) cls = "num";
      else if (m[4]) cls = this.identClass(src, m.index, m[0], imports, yaroze);
      out += cls ? `<span class="t-${cls}">${text}</span>` : text;
    }
    return out + escapeHTML(src.slice(last));
  }

  identClass(src, at, word, imports, yaroze) {
    if (GO_KEYWORDS.has(word)) return "kw";
    const prev = src.slice(Math.max(0, at - 1), at);
    const next = src.slice(at + word.length).match(/^\s*(\S?)/)[1];
    if (prev === ".") {
      // A name qualified by the package: what the package says it is.
      const qual = src.slice(0, at - 1).match(/(\w+)$/)?.[1];
      const entry = qual === yaroze && this.api?.byName.get(word);
      if (entry) return entry.kind === "func" ? "fn" : entry.kind === "type" ? "type" : "const";
      return next === "(" ? "fn" : "";
    }
    if (next === "." && imports.has(word)) return "pkg";
    if (GO_TYPES.has(word)) return "type";
    if (GO_BUILTINS.has(word)) return "builtin";
    if (next === "(") return "fn";
    return "";
  }

  // yarozeName is the name the source refers to the yaroze package by.
  yarozeName(imports) {
    for (const [name, path] of imports) if (path.endsWith("/yaroze")) return name;
    return null;
  }

  // markErrors shows the compiler's errors against their lines: a list of
  // {line, message}.
  markErrors(errors) {
    this.errors = new Map();
    for (const e of errors) if (!this.errors.has(e.line)) this.errors.set(e.line, e.message);
    this.lines = -1;
    this.render();
  }

  // goTo puts the caret at a line and column, both from 1.
  goTo(line, col = 1) {
    const lines = this.ta.value.split("\n");
    let at = 0;
    for (let i = 0; i < Math.min(line - 1, lines.length - 1); i++) at += lines[i].length + 1;
    at += Math.min(col - 1, lines[Math.min(line, lines.length) - 1]?.length ?? 0);
    this.ta.focus();
    this.ta.setSelectionRange(at, at);
    this.reveal(true);
  }

  // Measuring. The font is monospaced and every line the same height, so a
  // place in the text is a line and a column, which tabs widen.
  metrics() {
    if (!this.metric) {
      const style = getComputedStyle(this.hl);
      const ctx = document.createElement("canvas").getContext("2d");
      ctx.font = `${style.fontSize} ${style.fontFamily}`;
      this.metric = {
        char: ctx.measureText("0".repeat(100)).width / 100,
        line: parseFloat(style.lineHeight),
        top: parseFloat(style.paddingTop),
        left: parseFloat(style.paddingLeft),
      };
    }
    return this.metric;
  }

  // caretBox is where the caret is, in the scrolled content's coordinates.
  caretBox() {
    const before = this.ta.value.slice(0, this.ta.selectionEnd);
    const line = before.split("\n").length - 1;
    const text = before.slice(before.lastIndexOf("\n") + 1);
    let col = 0;
    for (const c of text) col = c === "\t" ? (Math.floor(col / TAB) + 1) * TAB : col + 1;
    const m = this.metrics();
    return {
      x: this.gutter.offsetWidth + m.left + col * m.char,
      y: m.top + line * m.line,
      h: m.line,
    };
  }

  // reveal scrolls the caret into view, to the middle of it if far off.
  reveal(center = false) {
    const c = this.caretBox();
    const s = this.scroller;
    const gutter = this.gutter.offsetWidth;
    if (c.y < s.scrollTop || c.y + c.h > s.scrollTop + s.clientHeight) {
      s.scrollTop = center ? c.y - s.clientHeight / 2 : c.y < s.scrollTop ? c.y - 8 : c.y + c.h - s.clientHeight + 8;
    }
    if (c.x < s.scrollLeft + gutter + 16) s.scrollLeft = Math.max(0, c.x - gutter - 32);
    else if (c.x > s.scrollLeft + s.clientWidth - 16) s.scrollLeft = c.x - s.clientWidth + 48;
  }

  // insert replaces from..to with text, through the browser's editing so
  // that undo still works, and leaves the caret at caret, or after text.
  insert(text, from = this.ta.selectionStart, to = this.ta.selectionEnd, caret = null) {
    this.ta.setSelectionRange(from, to);
    this.inserting = true;
    try {
      if (!document.execCommand("insertText", false, text)) {
        this.ta.setRangeText(text, from, to, "end");
        this.onInput({});
      }
    } finally {
      this.inserting = false;
    }
    if (caret !== null) this.ta.setSelectionRange(caret, caret);
    this.reveal();
  }

  onInput(e) {
    // Marks from the last build would point at the wrong lines once the
    // text changes.
    if (this.errors.size) {
      this.errors.clear();
      this.lines = -1;
    }
    this.render();
    this.options.onChange?.(this.ta.value);
    this.reveal();
    if (this.inserting) return;
    if (e.inputType === "insertText" && /[\w.]/.test(e.data ?? "")) this.complete(false);
    else if (this.popup) this.complete(false);
  }

  onKey(e) {
    const ta = this.ta;
    const mod = e.ctrlKey || e.metaKey;
    if (this.popup) {
      if (e.key === "ArrowDown" || e.key === "ArrowUp") {
        e.preventDefault();
        const n = this.popup.items.length;
        this.select((this.popup.selected + (e.key === "ArrowDown" ? 1 : n - 1)) % n);
        return;
      }
      if ((e.key === "Enter" && !mod) || e.key === "Tab") {
        e.preventDefault();
        this.accept(this.popup.selected);
        return;
      }
      if (e.key === "Escape") {
        e.preventDefault();
        this.close();
        return;
      }
    }
    if (mod && e.key === "Enter") {
      e.preventDefault();
      this.options.onRun?.();
      return;
    }
    if (mod && e.code === "Space") {
      e.preventDefault();
      this.complete(true);
      return;
    }
    if (mod && e.key === "/") {
      e.preventDefault();
      this.toggleComment();
      return;
    }
    const { selectionStart: start, selectionEnd: end, value } = ta;
    const lineStart = value.lastIndexOf("\n", start - 1) + 1;
    if (e.key === "Tab" && !mod && !e.altKey) {
      e.preventDefault();
      if (e.shiftKey || value.slice(start, end).includes("\n")) this.indentLines(e.shiftKey ? -1 : 1);
      else this.insert("\t");
      return;
    }
    if (e.key === "Enter" && !e.shiftKey && !mod && !e.altKey) {
      // Keep the line's indentation, one deeper after an opening bracket,
      // and with the caret between a pair, put the closer on its own line.
      e.preventDefault();
      const indent = value.slice(lineStart, start).match(/^[ \t]*/)[0];
      const before = value.slice(lineStart, start).trimEnd();
      const opens = /[{([]$/.test(before);
      const closes = opens && /^[ \t]*[})\]]/.test(value.slice(end).split("\n", 1)[0]);
      if (closes) {
        const text = `\n${indent}\t\n${indent}`;
        this.insert(text, start, end, start + indent.length + 2);
      } else {
        this.insert("\n" + indent + (opens ? "\t" : ""), start, end);
      }
      return;
    }
    if ((e.key === "}" || e.key === ")" || e.key === "]") && start === end && /^[ \t]+$/.test(value.slice(lineStart, start)) && value[start - 1] === "\t") {
      // A closer typed on a line of indentation alone outdents it.
      e.preventDefault();
      this.insert(e.key, start - 1, start);
    }
  }

  // indentLines indents the selected lines by one tab, or outdents them.
  indentLines(dir) {
    const ta = this.ta;
    const { selectionStart: start, selectionEnd: end, value } = ta;
    const from = value.lastIndexOf("\n", start - 1) + 1;
    const to = end > start && value[end - 1] === "\n" ? end - 1 : end;
    const lines = value.slice(from, to).split("\n");
    const changed = lines.map((l) => (dir > 0 ? "\t" + l : l.replace(/^(\t| {1,4})/, "")));
    const text = changed.join("\n");
    const shift = changed[0].length - lines[0].length;
    this.insert(text, from, to);
    ta.setSelectionRange(Math.max(from, start + shift), from + text.length + (to < end ? 1 : 0));
  }

  toggleComment() {
    const ta = this.ta;
    const { selectionStart: start, selectionEnd: end, value } = ta;
    const from = value.lastIndexOf("\n", start - 1) + 1;
    const to = end > start && value[end - 1] === "\n" ? end - 1 : end;
    const lines = value.slice(from, to).split("\n");
    const commented = lines.every((l) => /^\s*\/\//.test(l) || !l.trim());
    const text = lines.map((l) => {
      if (!l.trim()) return l;
      return commented ? l.replace(/^(\s*)\/\/ ?/, "$1") : l.replace(/^(\s*)/, "$1// ");
    }).join("\n");
    this.insert(text, from, to);
    ta.setSelectionRange(from, from + text.length);
  }

  // Completion.

  // context finds what is being completed at the caret: the members of the
  // package or of a variable's type, and the part of a name already typed.
  context() {
    if (!this.api) return null;
    const { value, selectionStart: at, selectionEnd } = this.ta;
    if (at !== selectionEnd) return null;
    const line = value.slice(value.lastIndexOf("\n", at - 1) + 1, at);
    // In a comment or a string, nothing is offered.
    const plain = line.replace(/"(?:[^"\\]|\\.)*"|`[^`]*`|'(?:[^'\\]|\\.)*'/g, '""');
    if (plain.includes("//") || /["`']/.test(plain.replace(/""/g, ""))) return null;
    const m = line.match(/(^|[^\w.\]])([A-Za-z_]\w*)((?:\[[^\[\]]*\])*)\.(\w*)$/);
    if (!m) return null;
    const [, , root, , prefix] = m;
    const yaroze = this.yarozeName(goImports(value));
    if (!yaroze) return null;
    if (root === yaroze) return { prefix, items: this.api.entries };
    const type = this.typeOf(root, value, yaroze);
    if (!type) return null;
    const items = [
      ...(type.fields ?? []).map((f) => ({ name: f.name, kind: "field", sig: `${f.name} ${f.type}`, doc: "" })),
      ...(type.methods ?? []),
    ];
    return items.length ? { prefix, items } : null;
  }

  // typeOf guesses a variable's yaroze type from how the source declares
  // it: with the type named, as a composite literal of it, or as what one
  // of the package's functions returns.
  typeOf(name, src, pkg) {
    const n = name.replace(/[$^*+?.()|[\]{}\\]/g, "\\$&");
    const list = `\\b${n}(?:\\s*,\\s*\\w+)*`;
    const named = src.match(new RegExp(`${list}\\s+(?:\\[[^\\]]*\\]|\\*)*${pkg}\\.(\\w+)`));
    const literal = src.match(new RegExp(`${list}\\s*:?=\\s*&?(?:\\[[^\\]]*\\])*${pkg}\\.(\\w+)\\s*\\{`));
    const call = src.match(new RegExp(`\\b${n}\\s*:?=\\s*${pkg}\\.(\\w+)\\s*\\(`));
    let type = (named ?? literal)?.[1];
    if (!type && call) {
      const sig = this.api.byName.get(call[1])?.sig ?? "";
      type = sig.match(/\)\s*\*?(\w+)$/)?.[1];
    }
    const entry = type && this.api.byName.get(type);
    return entry?.kind === "type" ? entry : null;
  }

  // complete opens or refreshes the list for the caret, or closes it when
  // nothing fits; forced, it opens even before anything is typed.
  complete(forced) {
    const ctx = this.context();
    if (!ctx || (!forced && !this.popup && !ctx.prefix && !this.ta.value.slice(0, this.ta.selectionStart).endsWith("."))) {
      this.close();
      return;
    }
    const p = ctx.prefix.toLowerCase();
    const starts = [];
    const contains = [];
    for (const item of ctx.items) {
      const name = item.name.toLowerCase();
      if (name.startsWith(p)) starts.push(item);
      else if (p && name.includes(p)) contains.push(item);
    }
    const byName = (a, b) => a.name.localeCompare(b.name);
    const items = [...starts.sort(byName), ...contains.sort(byName)].slice(0, 100);
    if (!items.length || (items.length === 1 && items[0].name === ctx.prefix)) {
      this.close();
      return;
    }
    this.popup = { items, prefix: ctx.prefix, selected: 0 };
    this.list.replaceChildren(...items.map((item, i) => {
      const li = document.createElement("li");
      li.role = "option";
      li.dataset.index = i;
      const kind = document.createElement("span");
      kind.className = `ed-kind k-${item.kind}`;
      kind.textContent = { func: "f", type: "T", const: "c", var: "v", field: "." }[item.kind] ?? "";
      const name = document.createElement("span");
      name.textContent = item.name;
      li.append(kind, name);
      return li;
    }));
    this.box.hidden = false;
    this.select(0);
    this.place();
  }

  // place puts the list under the caret, or over it if there is no room.
  place() {
    const c = this.caretBox();
    const s = this.scroller;
    const x = c.x - s.scrollLeft;
    const y = c.y - s.scrollTop;
    const box = this.box;
    const width = Math.min(460, this.root.clientWidth - 16);
    box.style.width = `${width}px`;
    box.style.left = `${Math.max(8, Math.min(x, this.root.clientWidth - width - 8))}px`;
    const below = this.root.clientHeight - (y + c.h);
    if (below >= box.offsetHeight + 4 || below > y) {
      box.style.top = `${y + c.h + 2}px`;
      box.style.bottom = "";
    } else {
      box.style.top = "";
      box.style.bottom = `${this.root.clientHeight - y + 2}px`;
    }
  }

  select(i) {
    const popup = this.popup;
    popup.selected = i;
    for (const li of this.list.children) li.setAttribute("aria-selected", String(Number(li.dataset.index) === i));
    this.list.children[i]?.scrollIntoView({ block: "nearest" });
    const item = popup.items[i];
    this.box.querySelector(".ed-doc code").textContent = item.sig;
    const doc = this.box.querySelector(".ed-doc p");
    doc.textContent = (item.doc ?? "").trim();
    doc.hidden = !doc.textContent;
  }

  accept(i) {
    const item = this.popup?.items[i];
    if (!item) return;
    const at = this.ta.selectionStart;
    const from = at - this.popup.prefix.length;
    this.close();
    this.insert(item.name, from, at);
  }

  close() {
    if (!this.popup) return;
    this.popup = null;
    this.box.hidden = true;
  }
}

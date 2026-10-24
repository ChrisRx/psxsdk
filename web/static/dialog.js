// Dialogs drawn in the page, in place of the browser's prompt, confirm and
// alert, which block the page, look like nothing else on it and cannot say
// what went wrong without closing: a name asked for, checked as it is
// typed; a step confirmed, the button saying what it does, with a way out
// beside it when there is one, such as downloading first; and a message.
// Each returns a promise of the answer. Escape and Cancel answer null for
// a name and false for a step.
"use strict";

const dialogs = (() => {
  let el = null;      // the dialog, made the first time one is asked for
  let settle = null;  // resolves the answer of the one showing

  function make() {
    el = document.createElement("dialog");
    el.className = "pg-dialog";
    el.innerHTML = `
      <form method="dialog">
        <h2 class="pg-dialog-title"></h2>
        <p class="pg-dialog-text"></p>
        <label class="pg-dialog-field"><span></span><input type="text" autocomplete="off" spellcheck="false"></label>
        <p class="pg-dialog-error" aria-live="polite"></p>
        <div class="pg-dialog-actions">
          <button type="button" class="pg-dialog-extra"></button>
          <span class="pg-dialog-gap"></span>
          <button type="button" class="pg-dialog-cancel">Cancel</button>
          <button type="submit" class="pg-dialog-ok primary">OK</button>
        </div>
      </form>`;
    document.body.append(el);
    el.addEventListener("cancel", (e) => {
      e.preventDefault();
      finish(null);
    });
    el.querySelector(".pg-dialog-cancel").addEventListener("click", () => finish(null));
    return el;
  }

  // open shows the dialog laid out for one question, answering a question
  // still showing as cancelled.
  function open({ title, text = "", label = null, value = "", ok = "OK", cancel = "Cancel", danger = false, extra = null }) {
    if (!el) make();
    if (settle) finish(null);
    el.querySelector(".pg-dialog-title").textContent = title;
    const p = el.querySelector(".pg-dialog-text");
    p.textContent = text;
    p.hidden = !text;
    const field = el.querySelector(".pg-dialog-field");
    field.hidden = label === null;
    field.querySelector("span").textContent = label ?? "";
    field.querySelector("input").value = value;
    el.querySelector(".pg-dialog-error").textContent = "";
    const okButton = el.querySelector(".pg-dialog-ok");
    okButton.textContent = ok;
    okButton.className = `pg-dialog-ok ${danger ? "danger-button" : "primary"}`;
    const cancelButton = el.querySelector(".pg-dialog-cancel");
    cancelButton.textContent = cancel;
    cancelButton.hidden = cancel === null;
    const extraButton = el.querySelector(".pg-dialog-extra");
    extraButton.hidden = !extra;
    extraButton.textContent = extra?.[0] ?? "";
    extraButton.onclick = extra ? () => extra[1]() : null;
    el.showModal();
    if (label !== null) {
      const input = field.querySelector("input");
      input.focus();
      // The name without its extension is what is likely to be changed.
      const dot = value.lastIndexOf(".");
      input.setSelectionRange(0, dot > 0 ? dot : value.length);
    } else {
      (danger ? cancelButton : okButton).focus();
    }
    return el.querySelector("form");
  }

  function finish(answer) {
    const s = settle;
    settle = null;
    if (el?.open) el.close();
    if (el) el.querySelector("form").onsubmit = null;
    s?.(answer);
  }

  return {
    // ask asks for a name, offering value. check says what is wrong with
    // what was typed, or nothing, and keeps the dialog open until it is
    // put right. It resolves to the name, trimmed, or null.
    ask({ title, text = "", label = "Name", value = "", ok = "OK", check = null }) {
      return new Promise((resolve) => {
        const form = open({ title, text, label, value, ok });
        const input = form.querySelector("input");
        const error = form.querySelector(".pg-dialog-error");
        input.oninput = () => { error.textContent = ""; };
        form.onsubmit = (e) => {
          e.preventDefault();
          const v = input.value.trim();
          const why = v ? check?.(v) : "A name is needed.";
          if (why) {
            error.textContent = why;
            input.focus();
            return;
          }
          finish(v);
        };
        settle = resolve;
      });
    },
    // confirm asks whether to go on, ok naming what happens, in red when
    // it cannot be taken back. extra, [label, fn], is a step to take on
    // the way, such as downloading first, which leaves the dialog open.
    // It resolves to true or false.
    confirm({ title, text = "", ok = "OK", danger = false, extra = null }) {
      return new Promise((resolve) => {
        const form = open({ title, text, ok, danger, extra });
        form.onsubmit = (e) => {
          e.preventDefault();
          finish(true);
        };
        settle = (v) => resolve(v === true);
      });
    },
    // tell says something, and resolves once it has been read.
    tell({ title, text = "" }) {
      return new Promise((resolve) => {
        const form = open({ title, text, ok: "OK", cancel: null });
        form.onsubmit = (e) => {
          e.preventDefault();
          finish(true);
        };
        settle = () => resolve();
      });
    },
  };
})();

// sentence makes a message, an error's say, a sentence of its own.
function sentence(text) {
  const t = String(text ?? "").trim();
  if (!t) return "";
  return t[0].toUpperCase() + t.slice(1) + (/[.!?]$/.test(t) ? "" : ".");
}

// confirmReplace asks before files the project has already are replaced.
function confirmReplace(paths) {
  return dialogs.confirm({
    title: paths.length === 1 ? `Replace ${paths[0]}?` : `Replace ${paths.length} files?`,
    text: paths.length === 1 ? "The project has a file of that name already." : `The project has ${paths.join(", ")} already.`,
    ok: "Replace",
  });
}

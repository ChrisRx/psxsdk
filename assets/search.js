// The search box: the index search.json, which the generator writes with
// every page's title and text, is fetched the first time the box is used
// and matched in the page. Each word typed has to appear somewhere in a page
// for it to be listed, titles first.
"use strict";

(() => {
  const root = document.documentElement.dataset.root || "";
  const input = document.getElementById("search");
  const list = document.getElementById("results");
  if (!input || !list) return;

  let pages = null;
  async function load() {
    if (pages) return pages;
    const res = await fetch(root + "search.json");
    pages = await res.json();
    for (const p of pages) {
      p.lt = p.t.toLowerCase();
      p.lx = p.x.toLowerCase();
    }
    return pages;
  }

  function snippet(text, lower, term) {
    const i = lower.indexOf(term);
    if (i < 0) return text.slice(0, 120);
    const start = Math.max(0, i - 50);
    return (start > 0 ? "…" : "") + text.slice(start, i + 90) + "…";
  }

  async function search(q) {
    const terms = q.toLowerCase().split(/\s+/).filter(Boolean);
    if (!terms.length) return hide();
    const all = await load();
    const hits = [];
    for (const p of all) {
      let score = 0;
      for (const t of terms) {
        if (p.lt.includes(t)) score += 10;
        else if (p.lx.includes(t)) score += 1;
        else { score = 0; break; }
      }
      if (score) hits.push({ p, score });
    }
    hits.sort((a, b) => b.score - a.score || a.p.t.localeCompare(b.p.t));
    show(hits.slice(0, 8), terms[0]);
  }

  function show(hits, term) {
    list.replaceChildren(...hits.map(({ p }) => {
      const li = document.createElement("li");
      const a = document.createElement("a");
      a.href = root + p.u;
      const title = document.createElement("b");
      title.textContent = p.t;
      const where = document.createElement("small");
      where.textContent = p.s;
      const text = document.createElement("span");
      text.textContent = snippet(p.x, p.lx, term);
      a.append(title, where, text);
      li.append(a);
      return li;
    }));
    if (!hits.length) {
      const li = document.createElement("li");
      li.className = "none";
      li.textContent = "Nothing matches.";
      list.append(li);
    }
    list.hidden = false;
  }

  function hide() {
    list.hidden = true;
  }

  let timer;
  input.addEventListener("input", () => {
    clearTimeout(timer);
    timer = setTimeout(() => search(input.value), 80);
  });
  input.addEventListener("focus", () => { load(); if (input.value) search(input.value); });
  input.addEventListener("keydown", (e) => {
    if (e.key === "Escape") { input.value = ""; hide(); input.blur(); }
    if (e.key === "Enter") {
      const first = list.querySelector("a");
      if (first) location.href = first.href;
    }
  });
  document.addEventListener("click", (e) => {
    if (!e.target.closest(".search")) hide();
  });
})();

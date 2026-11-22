// A copy button on each code block. A terminal transcript, a console block,
// copies only its commands: the lines after a $ prompt, without the prompt,
// and none of the output, so that what is pasted runs. Chroma marks a
// prompt gp, but takes Delve's "> main.tick()" and GDB's "#0" lines for
// prompts too, and GDB's "$1 = ..." values, so only a $ and a space count.
// A block with no such prompt is copied whole.
"use strict";

(() => {
  const copyIcon = '<svg viewBox="0 0 16 16" aria-hidden="true"><rect x="5.5" y="5.5" width="8" height="8" rx="1.5"/><path d="M10.5 5.5v-2a1.5 1.5 0 0 0-1.5-1.5H4A1.5 1.5 0 0 0 2.5 3.5V9A1.5 1.5 0 0 0 4 10.5h1.5"/></svg>';
  const doneIcon = '<svg viewBox="0 0 16 16" aria-hidden="true"><path d="M3 8.5l3.2 3L13 4.5"/></svg>';

  // prompts are a block's $ prompts.
  const prompts = (pre) => [...pre.querySelectorAll(".gp")].filter(
    (p) => p.textContent === "$" && p.nextSibling && p.nextSibling.textContent.startsWith(" "));

  // text is what copying pre gives. Chroma writes a line as a span of its
  // own, with the prompt and the command in it.
  function text(pre) {
    const commands = prompts(pre).map((prompt) => {
      let command = "";
      for (let node = prompt.nextSibling; node; node = node.nextSibling) {
        command += node.textContent;
      }
      return command.replace(/^ /, "").replace(/\n$/, "");
    });
    if (commands.length === 0) return pre.textContent.replace(/\n$/, "");
    return commands.join("\n");
  }

  async function write(s) {
    try {
      await navigator.clipboard.writeText(s);
      return true;
    } catch {
      // No clipboard API, as on a page opened from a file: the old way.
      const area = document.createElement("textarea");
      area.value = s;
      area.setAttribute("readonly", "");
      area.style.position = "fixed";
      area.style.opacity = "0";
      document.body.append(area);
      area.select();
      const ok = document.execCommand("copy");
      area.remove();
      return ok;
    }
  }

  for (const pre of document.querySelectorAll("article pre")) {
    const wrap = document.createElement("div");
    wrap.className = "code";
    pre.before(wrap);
    wrap.append(pre);
    const button = document.createElement("button");
    button.type = "button";
    button.className = "copy";
    button.setAttribute("aria-label", "Copy");
    button.title = prompts(pre).length > 0 ? "Copy the commands" : "Copy";
    button.innerHTML = copyIcon;
    let timer;
    button.addEventListener("click", async () => {
      const ok = await write(text(pre));
      button.innerHTML = ok ? doneIcon : copyIcon;
      button.setAttribute("aria-label", ok ? "Copied" : "Copy failed");
      button.classList.toggle("done", ok);
      clearTimeout(timer);
      timer = setTimeout(() => {
        button.innerHTML = copyIcon;
        button.setAttribute("aria-label", "Copy");
        button.classList.remove("done");
      }, 1500);
    });
    wrap.append(button);
  }
})();

---
title: The playground
weight: 5
description: Writing, building and running a PlayStation program in the browser, with nothing installed, starting from one of three programs.
---

[The playground](/play/) writes, builds and runs a program in the browser, with nothing installed. It runs the same compiler `yaroze` carries, TinyGo compiled to WebAssembly, the same converters and the same emulator ([how](../architecture/compiler.md)). **Run**, or Ctrl+Enter, formats the program, builds its disc and boots it beside the editor.

## Projects

A project is the files of one program, laid out as the examples are:

| Path | What is there |
| --- | --- |
| `main.go` | The program, and any other Go files beside it |
| `assets/` | The files the program registers with package `disc`, and the pictures, models and sounds its `go:generate` lines make them from |
| `scenes/` | The scene editor's files, cooked into `assets/` |
| `pack/NAME/` | The files packed into `assets/NAME.ARC` |

Projects are kept in the browser, and what is typed is saved a moment after it changes. Clearing the site's data removes them, so download anything worth keeping. The project menu, under the project's name, renames, duplicates and deletes a project. **Download zip** saves one in this layout, which `yaroze` builds once the directory has a module, made as [Getting started](index.md) shows. **Start over** puts back the files of the program it was started from.

## A new project

**New project** takes a name and one of three programs to start from, each an example in the repository, so it builds as that example does:

- [blank](../examples/blank.md): the screen, cleared every frame, and a line of text. For a program written from nothing.
- [jumpcube](../examples/jumpcube.md): a 2D platform level of tile layers made in the scene editor, with a cube that runs, jumps and takes coins.
- [quest](../examples/quest.md): rooms seen from above at a slant, joined by doorways and walked around by a hero. The start of a role playing game.

**Open folder** and **Import zip**, in the same dialog and in the project menu, bring in a project from this computer instead: a folder or a zip in the layout above, such as one of the repository's examples.

## Layouts

The switch in the top bar arranges the page for the work:

- **Code** shows the Go files in the editor, with the game, its controls and the compiler's output beside them.
- **Scenes** is the scene editor: models, sprites and lights placed, tile layers and rooms painted. Package [`scene`](scene) draws what it makes, and a change shows in the running program at once.
- **Assets** shows the files on the disc: pictures and sprites, models, sounds, archives and video memory.

The **?** button lists every key and mouse action.

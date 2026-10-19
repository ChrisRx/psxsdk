---
title: The frame and the controller
weight: 2
description: The frame loop of the square example, a filled box drawn into it, and the box moved with the directional buttons.
---

From here on the tutorial writes the [square](../examples/square.md) example, one part at a time. Start a new module as [setting up](setting-up.md) did, or reuse that one. This page covers the parts of it that draw the box and move it; the cube and the music are left for the pages after.

## The screen and the box

The program's state is package-level variables, as the first program's screen was. The screen is set up a little differently:

```go src=../../examples/square/main.go#screen
```

`Depth` is the ordering table's size, as a power of two: 16 entries, one for each depth a frame's drawing can be sorted to. The first program sorted nothing, so it did not matter; here the cube's faces will be sorted by how far away they are, and the box drawn over all of them. The rest of the screen's settings are left at their defaults, 320 by 240 at the console's own frame rate, sixty frames a second on an American or Japanese console and fifty on a European one.

The box is a `GsBOXF`, the library's filled rectangle: a position, a size and a color. A second one, thin and white, will be the bar that shows how far loading has got:

```go src=../../examples/square/main.go#box,bar
```

The controller is a `Pad`, which is read once a frame:

```go src=../../examples/square/main.go#pad
```

## The frame

This is the whole of `main`:

```go src=../../examples/square/main.go#main
```

### Where the screen is

`main` starts by opening the screen, as before, and then calls `GsInit3D`, which sets up the library's 3D and, with it, moves the origin of the screen's coordinates from the top left corner to the middle, which is where a 3D picture wants it. That applies to the box as well, so the program asks the screen where its corners are now. `Bounds` is the screen as the frame's coordinates see it, here from -160, -120 to 160, 120, so the text, the box and the bar are each placed from its corner or its middle and land where they would without `GsInit3D`. The box starts near the bottom left; the bar is a line across the middle that grows from no width as the files arrive.

### Each frame

Each time round, `pad.Update` reads the controller. `Dir` turns the directional buttons into steps of -1, 0 or 1 across and down, with opposite buttons held together cancelling, and `speed` makes those two pixels a frame:

```go src=../../examples/square/main.go#speed
```

`Clamp` moves the box back inside the screen's bounds if a step would take any of it outside. `Pressed` is true on the one frame a button goes down, not on every frame it is held, which is what a button that toggles something wants: Start pauses the music, and holding it should not pause and resume it sixty times a second. `Held` is the other kind, true for as long as the button is down.

`box.Sort(ot, 0)` sorts the box into the frame's ordering table at depth 0. The GPU draws the table from its deepest entry to its shallowest, so what is at 0 is drawn last, over everything else: the box always passes in front of the cube, whatever depth the cube's faces are sorted to. `Print` adds two lines of text to the font's window, which the frame draws after the table.

The `switch` that follows is the rest of the program: loading the files, saying so if they could not be read, and drawing the cube once they have been. The [next page](disc.md) is about the first two.

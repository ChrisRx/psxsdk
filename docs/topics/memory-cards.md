---
title: Memory cards
weight: 2
description: Saving a game to a memory card and loading it back, with the header and icon the console's card screen shows, and the card's files listed, dumped and deleted.
---

A memory card holds 128 KB, as 15 blocks of 8 KB and a directory. The BIOS keeps a small file system on it: a file has a name of up to 20 characters and a whole number of blocks, and a program reads and writes it through the kernel's file functions, `Open`, `Read`, `Write`, `Lseek` and `Close`, with `bu00:` in front of the name for the card in the first slot and `bu10:` for the second. A file is read and written in sectors of 128 bytes.

## A save file

A save that other games, and the console's own card screen, can show follows a convention Sony set out ([the save file format](../manuals/savefile.md)). Its name starts with the region and the game's product code, `BA` and a code for an American game, then a name of the game's own. Its first sector is a header: `SC`, how many frames its icon has, how many blocks the file takes, a title in full-width Shift-JIS, and the icon's 16 colours. The icon's frames come next, 16 by 16 pixels of 4 bits, a sector each, and then the game's data. `yaroze.SaveHeader` makes the header:

```go src=../../yaroze/card.go#SaveHeader
```

A Net Yaroze program had no product code, so its saves used `BA-NETYAROZE` and a name of its own, as the member FAQ suggested.

The [card](../examples/card.md) example saves a number with a title and an icon drawn by its `gen` program and turned into a 4-bit TIM by `png2tim`, whose pixels and palette are already in the form the header wants:

```go src=../../examples/card/main.go#writeSave
```

## Writing and reading

`yaroze.CardWrite` deletes the file if it is there, creates it with as many blocks as the data needs, writes it, and reads it back a sector at a time to see that it is there, and `CardRead` reads one:

```go src=../../yaroze/card.go#CardWrite
```

The BIOS does the card's work, a sector at a time, during the vertical blanks, and the call returns when it is done, so a save of a few sectors takes a few frames. A program checks what it loads anyway: a card can be pulled out half way through a save, and a mark at the start of the data, as the example's `PSXSDK CARD`, says whether what was read is the program's.

## Testing for a card

`TestCard` says whether a card is in a slot, whether it is new since the last test, and whether it is formatted. It talks to the card over the port the controller shares, and while it does the controller reads as nothing pressed for a frame, so a program tests when it starts and when the player asks to save or load, not every frame. `Format` writes an empty directory, and a program asks the player before it does.

Sony's checklist ([requirements](../manuals/requirements.md)) told magazine-disc programs not to touch memory cards at all, since a save's name should carry a product code. The example is for learning how cards work.

<!-- player card -->

The emulator keeps each slot's card in memory for the run, and the example works with Sony's BIOS and with OpenBIOS alike.

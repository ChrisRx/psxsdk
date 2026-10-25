---
title: Sony's manuals
weight: 8
group: Docs
section: Manuals
description: The Net Yaroze manuals, Sony's supplements to them, the programming FAQs and the SCEE tutorial, as Sony published them.
---

These are the documents Sony gave Net Yaroze members, from the printed manuals that came in the box to the supplements, FAQs and tutorials the members' websites carried until the program closed. They are reproduced as Sony published them. The pages here are the websites' HTML and Word files with their layout turned into the site's, and the words left alone: links into the members' home pages, which are long gone, point at the Wayback Machine, and the button symbols Word drew with its Symbol and Wingdings fonts are drawn as the symbols they were.

The [`yaroze`](yaroze) package keeps the library's names exactly as Sony wrote them, so what these documents say about `GsSortSprite` or `SsVabTransfer` is about the function of the same name in Go. They were written for C, for a Net Yaroze connected to a PC by its serial cable, and for the hardware of 1998: where they talk about uploading files with SIOCONS or about discs that the console will not boot, the toolchain here puts the files on a disc that boots in the emulator instead.

## The manuals

| Manual | |
| --- | --- |
| [Start Up Guide](010-1998-startupguide.pdf) | Setting up the Net Yaroze and its PC software, the August 1998 edition, also in [French](011-1997-startupguide-fr.pdf) and [German](012-1997-startupguide-de.pdf) |
| [User's Guide](020-1997-userguide.pdf) | The system, the library's services and the tools, the yellow book |
| [Library Reference](030-1997-libraryref.pdf) | Every function and structure in the library, the green book |
| [File Formats](040-1997-fileformat.pdf) | RSD, TMD, TIM, SEQ, VAG and VAB |

## Supplements and FAQs

| Page | |
| --- | --- |
| [Net Yaroze Programming FAQ](faq.md) | SCEE's FAQ, gathered from the newsgroups in 1998: ordering tables, packet areas, double buffering, the GPU's queue and cache, sprites that do not show, models that do not show, sound, fixed point |
| [Yaroze Member FAQ](member-faq.md) | SCEA's shorter FAQ |
| [Technical Requirements](requirements.md) | The rules a Net Yaroze program had to keep to for a magazine's cover disc, a subset of the checklist every published game passed |
| [Peripherals](peripherals.md) | The manual's additions on controllers and memory cards |
| [Save Files](savefile.md) | Naming a memory card file and the header that goes at its start |
| [Sound Basics](sound.md) | Sampling, CD audio, SPU-ADPCM and practical tips on samples, looping, MIDI and reverb |
| [TOD](tod.md) | The animation format libgs plays, missing from File Formats |
| [The GNU Debugger](gdb.md) | Debugging a program on the console with gdb over the serial cable |

## The SCEE tutorial

SCEE's Net Yaroze Programming Tutorial, by George Bain, is ten chapters of sample programs, each a directory of C with its data, for the GNU compiler and for CodeWarrior. Three of its chapters came with a document as well:

| Chapter | Programs |
| --- | --- |
| 1. [Peripherals](tutorial-peripherals.md) | A peripherals tester, the mouse, the neGcon, the controller, analog calibration, the GunCon |
| 2. Graphics | A generic game, double buffering, interlaced mode, 4, 8, 16 and 24-bit TIMs, semitransparent sprites, moving, scaling and rotating sprites |
| 3. [CD-ROM](tutorial-cd.md) | Loading files from the CD, playing CD-DA tracks |
| 4. Collision detection | Bounding boxes and spheres |
| 5. Timers | Root counters, a clock, the horizontal sync |
| 6. [Sound](tutorial-sound.md) | Music and sound effects |
| 7. Scrolling | Horizontal, vertical and parallax scrolling, tiles with a BG, star fields |
| 8. Artificial intelligence | Attack and evade, bouncing, random directions, set patterns, buzzing flies, random states, waves |
| 9. A 2D game | A whole game with sound, control and AI |
| 10. 3D graphics | The world, a model, projection, rotation and translation, fixed, moving and following cameras, lights, fog, texture mapping, gouraud shading, model attributes, ordering tables, split screens, coordinate systems |

The chapters' programs are in the `tutor` directory of [gwald/netyaroze_demo](https://github.com/gwald/netyaroze_demo), beside the demos the [examples](../examples/index.md) are ported from.

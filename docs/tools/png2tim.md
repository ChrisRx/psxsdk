---
title: png2tim
weight: 15
---

`png2tim` converts pictures into TIM files, the format a program loads into video memory for its sprites and textures, which Sony's TIMUTIL made on Windows. It reads PNG, BMP, GIF and JPEG, laid out as [`format/tim`](format/tim) describes, and `tim2png` turns a TIM back into a PNG, to look at a game's images or to edit them.

```bash
$ png2tim --at 640,0 --clut 0,480 cube.png
created "cube.TIM": 64x64 4-bit at (640,0), tpage 0x000a, u,v 0,0, CLUT at (0,480), cba 0x7800
$ tim2png CUBE.TIM
created "CUBE.png": 64x64 4-bit at (640,0), CLUT of 1 row(s) at (0,480)
```

A picture with a palette, an indexed PNG or GIF or a BMP of 8 bits or fewer, keeps its palette in order, since a program may name colors by their index, and becomes 4-bit when it uses no more than 16 colors and 8-bit otherwise. Any other picture becomes 4- or 8-bit when it has that few colors and 16-bit when it has more. `--bits` chooses, reducing the colors by median cut for 4 and 8 bits; 24 bits is for pictures that are only displayed, never drawn with.

Video memory is 1024x512 16-bit units, and `--at` and `--clut` say where in it the pixels and the palette go, which the TIM records and `Load` uses. A texture is drawn from a texture page, 64 units wide and 256 lines tall, and must not cross one; a 4-bit image is four pixels to a unit, so a 256-pixel-wide one fills a page. The tpage, u, v and cba printed are what a sprite or a model's primitives name the image by. The CLUT's x must be a multiple of 16.

The GPU does not draw a texel of 0x0000, which is how sprites have holes in them. A pixel with alpha under half becomes that. Opaque black becomes 0x8000, black that is drawn, unless `--black transparent` asks for the Net Yaroze convention of black as the transparent color, which most of the art in Sony's samples uses. `--semi` sets the STP bit on every other color, so that the whole image blends where a sprite or polygon turns semitransparency on; without it only black blends.

Colors are narrowed to 5 bits a component by dropping the low 3, as TIMUTIL did. The tests convert the BMPs in the Net Yaroze websites' archive that still sit beside their TIMs and check that the colors come out as Sony's tool made them.

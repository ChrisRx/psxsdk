---
title: Net Yaroze Programming FAQ
weight: 1
---

Welcome to the Yaroze Programming FAQ. This FAQ answers some of the questions most frequently asked when programming the Yaroze. It is currently maintained by [James Russell](mailto:James_Russell@scee.sony.co.uk) of Sony Computer Entertainment Europe. Please email him any clarifications, corrections or suggestions for questions.

The answers in this FAQ were compiled from questions and answers on the newsgroups. Many thanks to those who took the time to reply.

Get the latest version from:  [http://www.netyaroze-europe.com/yaroze/newmembers/yarfaq.htm](https://web.archive.org/web/1999/http://www.netyaroze-europe.com/yaroze/newmembers/yarfaq.htm)

This FAQ (C) 1998 Sony Computer Entertainment Europe. This FAQ is not to be distributed in whole or in part to any non Yaroze member.

**Last updated**: *10th December, 1998*

1.  [Version History](#version-history)
2.  [Documentation and Tool Links](#documentation-and-tool-links)
    1.  [Documentation and Tutorials](#documentation-and-tutorials)
    2.  [Tools & Utilities](#tools-and-utilities)
    3.  [Libraries](#libraries)
    4.  [Recommended Reading](#recommended-reading)
3.  [Controllers & Other Peripherals](#controllers--other-peripherals)
    - How can I make my Dual Shock vibrate?
    - How do I use the Light Gun or MultiTap?
    - Are there any provisos on how I should name a memory card file?
    - Why does the output of printf() sometimes corrupt?
    - Why do I sometime get garbage like "-\[0;36;1m" from SIOCONS?
    - Why do I get "Memory card checking - not found" errors when a memory card is inserted?
    - How can I format a memory card?
    - How do I permantly set a higher baud rate for the Yaroze?
    - Can I read from CDs other than the Yaroze boot CD?
    - Can I create my own CDs and use them in the Yaroze or another Playstation?
    - Why does TestCard() hang when used with CodeWarrior?
4.  [General Graphics & 2D Graphics](#general-graphics--2d-graphics)
    - Is there a hardware mechanism for collision detection?
    - How can I read/write individual pixels?
    - How do I detect whether I'm in NTSC or PAL?
    - Why can't I see my sprite on screen?
    - What are the minimum/maximum sizes for sprites?
    - How do I display a large (\> 256x256) image on the screen?
    - Why do I get weird extra vertical lines in some of my sprites?
    - What is an Ordering Table? What is an Ordering Table length?
    - How does the GPU work? What is the GPU queue?
    - What is the GPU cache?
    - What is FntPrint(), and how does it work?
    - How can I convert an ASCII string into a Kanji string?
    - What is double buffering?
    - Why do I need two packet areas and ordering tables?
    - How do I load TIMs into VRAM?
    - How big should I make my GsSetWorkBase area?
    - Why is there a solid bar at the bottom of the screen in PAL mode?
    - When do I call GsSortClear?
    - Why is there a single line that doesn't clear in my 256/512 high display buffer?
    - How can I use transparency in non-textured polygons?
    - How can I get more CPU/GPU time?
5.  [3D Graphics](#3d-graphics)
    - What do all the 3D related functions do?
    - Are there any tutorials which explain Yaroze 3D and matrices?
    - Why am I getting strange results with RotMatrixX, RotMatrixY and/or RotMatrixZ?
    - My object rotation is really strange - it works OK sometimes, but rotates at really weird angles at other times. What's happening?
    - How do I place a 2D sprite in a 3D world?
    - Why do I get strange UVs when I use RSDLink?
    - Why can't I see my 3D model?
    - Why am I'm getting errors like "GPU CODE xxH not supported" or "GPU CODE xxH not assigned"?
6.  [Sound](#sound)
    - What's the deal with MIDI on the PSX?
    - How do I pause a sound?
    - Can I play MOD files?
    - My sample seems off-key.
7.  [C and Compiling](#c--compiling)
    - What are some good books I can read?
    - What is the DCache?
    - How can I set the stack to the DCache?
    - The GNU compiler crashes with a stack error! How do I fix it?
    - I am running out of environment space! How do I set more?
    - How can I include and reference data in my C program directly, instead of uploading it separately with SIOCONS?
    - Is there anything to help me manage the addresses I place my data?
    - My data comes after my program, but I'm not sure exactly where to start placing it.
    - What is fixed point arithmetic?
    - I want to perform a maths function like `sin` or `cos`, but CodeWarrior complains that something is undefined.
    - Why are sin/cos/tan so slow? How can I speed them up?
    - What is a good way of conditionally compiling debugging strings?
    - What are these 'text', 'data' and 'BSS' sections I keep seeing?
    - What does the 'static' keyword in C mean?
    - I want to send some examples of my work to games companies - what do I have to do?

## Version History

- 10th December 1998
  - Added "How do I place a 2D sprite in a 3D world?" to the 3D section.
  - Added "How can I read/write individual pixels?" to the 2D section
- 16th November 1998
  - Added "Why can't I see my sprite?" to the 2D section.
  - Minor adjustments to "How big should I make my GsSetWorkBase area?"
- 19th October 1998
  - Added "Why am I getting strange results with RotMatrixX/Y/Z?" in the 3D section.
  - Added "How can I get more CPU/GPU time?" to the General/2D section.
  - Updated "When should I call GsSortClear" to include info on interlaced mode.
  - Added "What is a good way of conditionally compiling debugging strings?" to the C and Compiling section.
  - Added ClutAdd tool.
- 15th September 1998
  - Added Documentation and Tool links section.
  - Mild aesthetic cleanup.
  - Added the exact sizes of the different polygon types to "How big should I set my workbase?".
- 26th August 1998
  - Added more R3000 references.

  - Added "Are there any provisos on how I should name a memory card file?" and removed "How can I create a memory card file." (It is possible to create memory card files).
  - Added additional information on the D-Cache and GPU Cache.
  - Added "How can I use transparency in non-textured polygons?" to the 2D graphics section.
- 11th August 1998
  - Added "Why do I get "Memory card checking - not found" errors when a memory card is inserted?" and "How do I permantly set a higher baud rate for the Yaroze?" to Controllers & Other Peripherals
  - Added "What do all the 3D related functions do?" to the 3D Graphics area.
  - Removed "Why do I sometimes get strange results with ClearImage, LoadImage and StoreImage?" from 2D Graphics.
- 10th August 1998
  - Added "Why am I'm getting errors like "GPU CODE 00H not supported"?" to the 3D area.
  - Added "I want to send some examples to games companies - what do I have to do?" to the C/Compiling area
- 4th August 1998
  - Added "What's the deal with MIDI on the PSX?" to the Sound area
  - Updated MOD capabilities in Sound area.
  - Added "Why can't I see my 3D model?" to the 3D graphics area.
  - Added "Why is there a single line that doesn't clear in my 256/512 high display buffer?" to the 2D graphics area.
  - Added "What is fixed point arithmetic?" and "Why are sin/cos/tan so slow? How can I speed them up?" to the C/Compiling area.
- 28 July 1998 - First version.

## Documentation and Tool Links

This area contains links to reference material, tutorials, utilities and libraries created by other Net Yaroze users. Please e-mail the maintainer of this FAQ (address at the top) if you find any broken links.

### Documentation and Tutorials

- [Vectors and Matrices Tutorial](https://web.archive.org/web/1999/http://www.netyaroze-europe.com/~jruss1/matrix.html) - Contains information on how 3D and the GTE work inside the Playstation.
- [George's Tutorials](https://web.archive.org/web/1999/http://www.netyaroze-europe.com/yaroze/docs/main.htm) - Examples on sound, all peripherals (including light guns) and file formats.
- [CodeWarrior C++ HowTo Guide](https://web.archive.org/web/1999/http://www.netyaroze-europe.com/~c_graham/cplus.html) - How to code in C++ using CodeWarrior.
- [Yaroze Programming Tutorial](https://web.archive.org/web/1999/http://www.netyaroze-europe.com/~middx_uni/ftp/index.html) - From Middlesex University.

### Tools and Utilities

- [Dataman](https://web.archive.org/web/1999/http://www.netyaroze-europe.com/~jruss1/index.html) - automatically organises the positioning of data files within main memory, and generated appropriate 'auto' files and header files.
- [Raw2Src](https://web.archive.org/web/1999/http://www.netyaroze-europe.com/~jruss1/index.html) - converts a raw data file into a C source file, allowing raw data to be easily included and referenced in C programs.
- [MemEdit](https://web.archive.org/web/1999/http://www.netyaroze-europe.com/~jruss1/index.html) - Playstation program allowing the viewing and editing of memory card files.
- [Megatim](https://web.archive.org/web/1999/http://www.netyaroze-europe.com/~jruss1/index.html) - Creates one giant 16-bit TIM from many small TIMs.
- [Grabauto](https://web.archive.org/web/1999/http://www.netyaroze-europe.com/~jruss1/index.html) - Puts all data files into virtual Playstation RAM, then grabs a section of it. Means you only have to upload one data file instead of many.
- [Convert](https://web.archive.org/web/1999/http://www.netyaroze-europe.com/~nslaven/ftp/utils/convrt14.zip) - This package will convert between most audio formats.
- [RSDAnim](https://web.archive.org/web/1999/http://www.netyaroze-europe.com/~c_graham/rsdanim.html) - Allows you to create animated TMDs.
- [BgEdit](https://web.archive.org/web/1999/http://www.netyaroze-europe.com/~shaughnj/index.html) -  allows you to create any size BgMaps actually on the Yaroze. ("BgView" is a simple example of how to incorporate them in your games)
- [MemView](https://web.archive.org/web/1999/http://www.netyaroze-europe.com/~shaughnj/index.html) - Gives a graphical view of the whole of the Yaroze memory.
- [Analog](https://web.archive.org/web/1999/http://www.netyaroze-europe.com/~shaughnj/index.html) - Shows analogue pad values for calibration diagnostics.
- [ScBuild](https://web.archive.org/web/1999/http://www.netyaroze-europe.com/~shaughnj/index.html)  - DOS program that creates a SIN/COS lookup table header file.
- [ClutAdd](https://web.archive.org/web/1999/http://www.netyaroze-europe.com/~d_face/index.html)  - Allows you to add CLUTs to a TIM file, for displaying the same sprite with different CLUTs.

### Libraries

- [Trig](https://web.archive.org/web/1999/http://www.netyaroze-europe.com/~nslaven/ftp/utils/index.html) -  A simple fixed point trig library. Contains fixed point versions of cos, tan, sin, atan,asin, acos and a square root function.
- [FLibraries](https://web.archive.org/web/1999/http://www.netyaroze-europe.com/~mrfrosty/index.html) - Incorporates a controller library, screen patcher, versatile Yaroze development template and makefile.

### Recommended Reading

- **Graphics:** "Computer Graphics" by Foley, Van Dam, Feiner & Hughes (Addison Wesley, ISBN 0-201-12110-7) is considered the graphics bible, but isn't for the faint-hearted.
- **Graphics:** A much more reader friendly graphics book is "Computer Graphics" by F.S. Hill Jr. (Macmillan, ISBN 0-02-354860-6).
- **R3000 assembler:** "The MIPS Programmer's Handbook", by Farquhar & Bunce (Morgan Kauffman, ISBN 1-55860-297-6). It does NOT teach assembly language nor assembly language principles.
- **R3000 assembler:** "MIPS risc architecture" by Kane and Heinrich (ISBN 0-13-590472-2) It is even less of a gentle introduction then the book listed above. It also covers other MIPS processors like the R2000, R4000 and R6000.
- **R3000 assembler:** There are some MIPS simulators called SPIM that are available for Windows and Unix. <http://www.emulation.net/mips/index.html>
- **C:** "The C Programming Language" by Kernighan & Ritchie (?, ISBN ?) is considered the C bible, and it is by the people who invented C! It isn't brilliant though, and it certainly helps if you already know another language. If you know very little C or don't have much experience programming, then try one of the "C for Dummies" books.
- **C:** "C for Dummies" is part of the very popular "xxx for Dummies" series. We've never seen them, but apparently they're quite good.
- **Graphics** - "OpenGL Programming Guide - the official guide to learning OpenGL" by Mason Woo, Jackie Neider, Tom Davis: ; 1996 ISBN 0-201-46138-2. "Although the NY is not programmed in OpenGL, this book provides a very practical introduction to the three dimensional graphics pipeline. It not only introduces to the OpenGL commands but also explains the concepts behind it. Effectively we use OpenGL for the practical assignments for some of the courses we give here at the Computer Graphics Lab. Once you have understood the concepts behind OpenGL, you probably have very little difficulties in understanding the NY stuff. I think understanding OpenGL is easier to start with, as it is more transparent than the NY library. Another advantage of this book is, that is gives some hints how to implement more sophisticated technqiues like shadows, DOF and so on. (Actually we use shadows as an introduction to stencil buffer)." -- *recommended by Christopher Luerig*
- **Physics** - "Taschenbuch der Physik" - by Horst Kuchling; ISBN 3-446-19692-1; "This is a german book. The translation would be: Pocket Book of Physics. Probably there is an english translation of this book (author is a german) as it is quite popular here. It was used in the physics undergraduate course for computer science students at our university. The book contains a quite thick chapter about mechanics. You can use it as a recipe book, if you want to model somehow physically realistic phenomena. The book is divided into several small stand alone chapters, that provide an intuitive description of a phenomenon, some images and the relevant formulas. The book covers everything from basics like mass, acceleration, torque up to deformation phenomena of materials under shearing forces. All formulas are kept simple for the reduced case, so you will not find the partial differential equations for linear elasticity in there. You can compare this book somehow the Bronstein/Semandjajew for mathematics, except for the fact, that it does not go that deep into the material." -- *recommended by Christopher Luerig*

## Controllers & Other Peripherals

#### How can I make my Dual Shock vibrate?

You can't. The libraries do not have the ability to send information back to the controller.

#### How do I use the Light Gun or MultiTap?

It is possible to use the Namco Guncon light gun, but not the Konami light gun. It is not possible to use the Multitap. Examples of how to use the Namco light gun are available on the Yaroze site.

#### Are there any provisos on how I should name a memory card file?

If your game is submitted for a cover disk and uses memory card files, the name of the memory card file should adhere to a standard set by Sony, which involves the product code of the CD. Sony will let you know the exact details of this standard when you require it. However, due to Quality Assurance complications, it is recommended that you only use memory card code if **absolutely necessary**. For compliance, use the following standard:

| Bytes | Contents |
|----|----|
| 0 | Magic number. Always 'B' |
| 1 | Region. Japan 'I', North America 'A', Europe 'E'. |
| 2-11 | Title SCE product code number. Sony will tell you this if your game is going to be published. Otherwise, use anything. |
| 12-20 | User/Public. Use only non 0x00, non-0x2a('\*'), non-0x3d('?'), ASCII. End with 0x00. |

#### Why does the output of printf() sometimes corrupt?

Codewarrior sometimes drops bytes. The solution is to use SIOCONS. Dropping the baud rate helps, but does not eliminate the problem.

#### Why do I sometime get garbage like "-\[0;36;1m" from SIOCONS?

Make sure DEVICE=ANSI.SYS is in your `config.sys` file.

#### Why do I get "Memory card checking - not found" errors when a memory card is inserted?

The Yaroze will return this error if the memory card is not formatted, *or* if there is no Baud Rate saved (even on a formatted card). See below for instructions on formatting it and creating the Baud Rate file.

#### How can I format a memory card?

Insert it in slot 2 and type "`format bu10:`" from SIOCONS.

#### How do I permantly set a higher baud rate for the Yaroze?

To make a Baud Rate file (which saves a baud rate higher than 9600):

1.  Insert a memory card into card slot 2 (right hand side). The memory card should be already formatted and empty.
2.  In the SIOCONS prompt (`>>`), type: `BAUD 115200` This will cause a message to be displayed on screen.
3.  Quit SIOCONS.
4.  Invoke siocons with siocons -B115200
5.  Press return several times; wait for several seconds until the following message appears "Save the new baud rate into the memory card ? ( y/n )" type 'y' in response to this.
6.  SIOCONS will print 'Completed'.

When you reboot the Yaroze, it will use this baud rate if that memory card is inserted. Whenever you start SIOCONS again, you must use the `-Bxxxx` option to set SIOCONS's baud rate.

#### Can I read from CDs other than the Yaroze boot CD?

No. The hardware copy protection prevents this. You can play audio tracks from other CDs though.

#### Can I create my own CDs and use them in the Yaroze or another Playstation?

No. Once again, the hardware copy protection prevents any CDR discs from being used.

#### Why does TestCard() hang when used with CodeWarrior?

For reasons that are still unclear, CodeWarrior does not appear to link the TestCard function properly, which causes a hang when you try to run your program. Compiling and running with the GCC compiler will work. The problem appears to be the callbacks and controller initialisation. This is not performed by the PSComUtil utility (which CodeWarrior uses). The solution is to initialise a graphics screen, and try after a few VSyncs.

## General Graphics & 2D Graphics

#### Is there a hardware mechanism for collision detection?

No. Collision detection must be performed in software. It is possible to read and write individual pixels (see below), but you are likely to run into accuracy problems. The results of a StoreImage will be from frame X, when the CPU is currently building frame X+2. If you cleverly arrange your StoreImage calls, you can arrange it so that the result of the StoreImage will be from the last frame rather than the second to last frame.

The best way to perform collision detection is to use combinations of the following techniques:

- Bounding boxes/spheres - Each object has a specific bounding box or sphere (this is invisible, it is not part of the object model). This box/sphere is the smallest possible box/sphere that completely contains the object. To test if two objects that must be tested for collision have indeed collided, test the boxes/spheres for collision. If they have not intersected, then no further testing needs to be done. If they have intersected, then you can either assume that the two objects have hit, or engage in some more accurate testing.
- Outline detection - This is more accurate, but much slower than bounding box/sphere detection. For 2D sprites, create an outline of your object using lines. Then you can test the outlines of the two objects for intersection. If a line from one object interesects a line from the other object, or any point from one object is 'inside' the outline of the other object, then there is a definite collision. This is very slow and only works well on objects with simple outlines.
- BSP trees - I won't describe in detail how what these are or how they are used. Basically, you can create a set of bounding lines or planes around your 2D or 3D object respectively. This allows you to quickly check whether a point lies within the area defined by the lines/planes.
- Pixel detection - because of the 'frame latency' problems where the screen display is 2 frames behind the screen that the CPU is creating, it is awkward to use StoreImage/LoadImage collision detection. However, if you have the memory available and if it suits your game, then do some of your game drawing in main memory and use bitwise 'and' to test collisions. The image in main memory doesn't have to be identical to the image on the screen - it can just contain masks used for collision detection. With a little thought, such a buffer can take up around 10Kb.

#### How can I read/write individual pixels?

With the PC and many other computer architectures, Video RAM is memory mapped, which means you can access it just like it was memory. This makes it very simple to read and write pixels. However, on the PlayStation, main memory and Video RAM are separate, and only the DMA/GPU can talk to VRAM, not the CPU. It is possible to read and write individual pixels on the PlayStation, but you have to get the DMA/GPU to do it for you.

To load an individual pixel to the screen, sort in a line which has indentical start and end points. If you want to replace a small area, then you can use LoadImage, which can upload an arbitrarily sized rectangle into VRAM (Remember that 1 pixel in VRAM is equal to 2 byte in main memory).

To access the pixels in VRAM, the only method is StoreImage, which transfers an arbitrarily sized rectangle of VRAM back to main memory.

It is important to realise that performing a LoadImage/StoreImage command in your program doesn't instantly execute it. Instead, it puts that command into the GPU queue (see below), and the GPU will execute it when it has executed all previously inserted commands in the queue. This means if you want to use the result of a StoreImage command, you'll have to call DrawSync(0) after the StoreImage and before using the value.

    void loadPixel(int x, int y, unsigned short colour) {
        static RECT loadRect = { 0, 0, 1,1 };
        static unsigned long col;
        loadRect.x = x;
        loadRect.y = y;
        col = colour;
        LoadImage(&loadRect, &col);
    }

Note: A LoadImage call only puts the command into the GPU queue, so the value of col that is used is the value in 'col' when the GPU comes to execute the command. Hence successive calls to this function without a DrawSync(0) in between will probably result in the wrong colour being loaded!

    void storePixel(int x, int y, unsigned long *address) {
        static RECT storeRect = { 0, 0, 1, 1 };
        storeRect.x = x;
        storeRect.y = y;
        StoreImage(&storeRect, address);
    }

#### How do I detect whether I'm in NTSC or PAL?

When the PlayStation first boots up the video mode defaults to NTSC. If you do a GetVideoMode() at the startup it will be NTSC. Check the product code (which is different in all 3 territories) on the Net Yaroze Boot Disc. If the file is not found we now know that the user has a NTSC machine. Note: You have to use a scart cable to see color in NTSC mode.

    if( CdSearchFile(&file, "\\DTL_S30.35;1") == 0) {
        printf("Boot file not found... MODE_NTSC enabled \n");
        SetVideoMode( MODE_NTSC );
        video_mode = GetVideoMode();
    } else {
        printf("Boot file found... MODE_PAL enabled \n");
        SetVideoMode( MODE_PAL );
        video_mode = GetVideoMode();
    }

If your game is going to be submitted for a coverdisk, it is allowed to run in one video mode only. The ability to switch modes constitutes a Sony Quality Assurance failure.

#### Why can't I see my sprite on screen?

There are a number of reasons you may not be able to see your sprite.

- You haven't loaded the TIM image into VRAM. To check if this is a problem, make sure you can see at least something on the screen (by using FntPrint), then use MoveImage to copy the area of VRAM where you believe your sprite to be to the screen area. If you don't see anything, then you probably haven't loaded your TIM, your TIM is getting overwritten by another TIM, or you're accidentally clearing that area.
- You haven't properly up all the appropriate structures in your GsSPRITE structure. You can use the TIM loading code below to set up a basic sprite.
- Remember to set the RGB values to something greater than 0, or you won't see anything at all.
- When you call GsInit3D(), it internally sets (0,0) to be the middle of the screen. If you don't call it, it sets (0,0) to be the top left of the screen. Make sure your X and Y coordinates are appropriately set, because if they are too large your sprite will be drawn off screen.
- Make sure your texture page and texture U/Vs are set correctly.

#### What are the minimum/maximum sizes for sprites?

A sprite can be anywhere from 1x1 to 256x256 rendered pixels. (Note that if you are using `GsSortFastSprite` or backgrounds, the U coordinate must be even and the width must be even).

A 4/8 bit image takes up less space in VRAM, but the maximum UV widths are still (256,256). Because a computer starts counting at 0, not 1, the maximum UV coords you can specify are (255,255).

The UV coordinates are specified relative to the top left of the TPAGE, not the sprite image data.

#### How do I display a large (\> 256x256) image on the screen?

Either:

1.  Set the display area to the image in VRAM.
2.  Use a MoveImage command to copy the image into the display area.
3.  Use multiple sprites.

#### Why do I get weird extra vertical lines in some of my sprites?

For sprites that will be sorted with GsSortFastSprite, or for backgrounds, the U coordinate must be even and the width must be even.

#### What is an Ordering Table? What is an Ordering Table length?

To draw things on the screen, the Yaroze sends the GPU a list of commands, and it's the Graphics Processor that does the actual drawing. You can't affect the screen directly like on other machines. These commands are operations like "Draw Line" and "Clear Screen", or "Draw This Triangle with This Texture Map", and each command has a number or arguments (for instance, Draw Line's arguments specify the start and end points, and the colour of the line to be drawn).

With traditional games, especially 3D ones, the way you render a scene is to draw the objects furthest away from you first, then work your way to the objects nearest to you. This is called the Painters Algorithm. The reason we do this is because if you draw something near, then draw something further back that overlap in an area on the screen, the object further back will cover or overlap the object that is supposed to be nearer, and your scene will look all wrong. With this appraoch, you have to sort out which objects will be drawn first, and which will be drawn last.

This is where Ordering Tables come in. With ordering tables, you just work out what you want you object to look like (position, size and so on) and insert it (using `GsSortSprite` or `GsSortObject`) into the Ordering Table. The graphics library will automatically figure out where your object is supposed to be in amongst the other objects, so that when the GPU come to draw the scene, everything looks right. You don't have to worry about sorting objects, because the Ordering Tables do that for you. When you've put every object you want on the screen into the Ordering Table, you set the GPU off and it goes and draws everything for you.

The `GsSortSprite` and `GsSortObject` functions also create the Graphics Processor commands decribed above to draw your object. They put those commands in a working area, which you specify with `GsSetWorkbase` every frame. Ordering Tables are used by both 2D and 3D graphics, so you can mix sprites (2D graphics) with 3D objects. It may seem strange to think of 2D graphics as having an order, but if you can imagine a games with a scrolling background, well, you'd want that to be 'behind' your main character, so it would be further back in the order.

When you insert and object into an Ordering Table, you've got to specify how far back the object is. This 'distance' is called the objects *priority*. The higher the priority, the further back it is (if this seems strange, think of higher priority as meaning it gets drawn sooner). This is where the 'length' comes in. A priority of 0 means it is drawn last (is closest to you). But what is the highest priority? Well, the highest priority will depend on your situation. If you've only got two or three objects to draw, then you only need two priorities maximum, so a maximum priority of 1 will do. If you've got a zillion 3D objects to draw, then you'll probably need more! If you have complicated 3D objects, having a greater maximum priority can avoid some drawing errors.

When you make an Ordering Table, you choose the maximum priority. Priorities start at 2 and go up in powers of 2 (up to 2^14). This is for internal graphics processing reasons. So you could have a maximum priority of 2-1 (1), 4-1 (3), 8-1 (7), .... 16384-1 (16383).The higher the maxmimum priority, the more memory is taken up though. This *exponent* is called the Ordering Table 'length'. The minimum length is 1 and the maximum length is 14. Instead of having to remember these big numbers, you just use the exponent of 2. Thus, if you have an Ordering Table with length 10, that means you've got a maximum priority of 2^10-1 = 1024-1 = 1023. 0 is always the lowest priority. The reason why there are two Ordering Tables are because there must be one for each frame of the double buffering (ie the Graphics Processor is busy drawing off one while you're calculating the other).

An Ordering Table works via linked lists of GPU commands. One important thing to remember is that if you insert an object using `GsSortObject` or `GsSortSprite` into an Ordering Table at, say, priority 6, then insert another object at priority 6, the object drawn *first* will be the *last* one to be inserted. Thus if you have the following objects and insert them in the following order at the following priorities (My 'length' is 4, so my maximum priority is 2^4-1 = 15):

Aeroplane - priority 4
Wizard - priority 10
Moose wearing Wax Lips - priority 7
Postman Pat - priority 3
Thermonuclear Toaster - priority 7
Large Weasel with Contract - priority 10

They will be drawn by the GPU in the following order (remember, last to be drawn appears 'closest'):

Large Weasel - 10
Wizard - 10
Toaster - 7
Moose - 7
Aeroplane - 4
Postman Pat - 3
So Postman Pat will appear in front of all the others. If you use `GsSortObject4`() to send a 3D object to an Ordering Table, the libraries will break up the object into polygons of various priorities, in such an order that the furthest polygons are drawn first. The next question usually is - "Do I have to use Ordering Tables?" and the answer is yes, even though your application may not need them. OT's are the only way of getting the GPU to draw anything for you (apart from MoveImage).

#### How does the GPU work? What is the GPU queue?

The GPU (Graphics Processing Unit) processes many different commands (like 'Draw textured 3-sided poly at this point', 'Clear screen', etc). These commands can be linked to form a linked list of commands. This is done internally and automatically by the libraries. The GPU has a queue of up to 64 linked lists (each of which can be any length). The commands `ClearImage`, `LoadImage`, `StoreImage`, and `GsDrawOt` each put *one* linked list on the queue. The length of the linked list is 1 for the `Load`/`Store`/`Clear`/`MoveImage` functions, and is variable for `GsDrawOT`, depending on how many sprites or polygons you've put into the ordering table. The GPU constantly polls the queue and processes a linked list if one is available. This means that you can have up to 64 linked lists in the queue at any one time. The function `DrawSync(0)` returns only when the GPU has finished processing all the commands in the queue. If the queue is full (i.e. there are 64 linked lists already in the queue) and you try in add another linked list through one of the functions listed above, the call will block (wait) until there is a space in the queue.

#### What is the GPU cache?

The GPU has a cache on board, just like the CPU. It uses this to cache textures, and is 2k in size. The effective size of the texture cache changes depending on what type of texture you are rendering. A typical speed increase can be 30% for rendering from the texture cache.

| Texture resolution | Cache size (rendered pixels) |
|--------------------|------------------------------|
| 4-bit              | 64x64                        |
| 8-bit              | 64x32                        |
| 16-bit             | 32x32                        |

#### What is FntPrint(), and how does it work?

`FntPrint`() is like `printf`(), except it prints to the screen. Firstly initialise it (required once at the start) with `FntLoad(x,y)` where x,y is the position in VRAM of the font. The CLUT for the font will be placed at `y+128`, this is fixed. Followed this command with

`FntOpen(0,0,<screen_width>,<screen_height>,1,1024)`.

To print to the screen, use `FntPrint()` like you would use `printf`(). When you are ready to see all your text on the screen, use `FntFlush`(-1). This will put the text on the screen and reset the cursor to the top left.

To change the colour of FntPrint's output, insert the sequence "`~crgb`" in your string, where r, g and b are colour values from '0' to '9'.

#### How can I convert an ASCII string into a Kanji string?

    // ASCII code to Shift-JIS code transfer table
    static unsigned short ascii_table[3][2] = {{0x824f, 0x30}, /* 0-9 */ {0x8260, 0x41}, /* A-Z */ {0x8281, 0x61}, /* a-z */ };
    // ASCII code to Shift-JIS code transfer table
    static unsigned short ascii_k_table[] = {
    0x8140, /* */ 0x8149, /* ! */ 0x8168, /* " */ 0x8194, /* # */ 0x8190, /* $ */ 0x8193, /* % */ 0x8195, /* & */
    0x8166, /* ' */ 0x8169, /* ( */ 0x816a, /* ) */ 0x8196, /* * */ 0x817b, /* + */ 0x8143, /* , */ 0x817c, /* - */
    0x8144, /* . */ 0x815e, /* / */ 0x8146, /* : */ 0x8147, /* ; */ 0x8171, /* < */ 0x8181, /* = */ 0x8172, /* > */
    0x8148, /* ? */ 0x8197, /* @ */ 0x816d, /* [ */ 0x818f, /* \ */ 0x816e, /* ] */ 0x814f, /* ^ */ 0x8151, /* _ */
    0x8165, /* ` */ 0x816f, /* { */ 0x8162, /* | */ 0x8170, /* } */ 0x8150, /* ~ */ };
    // ASCII code to Shift-JIS code transfer function
    int ascii2sjis(unsigned char ascii_code) {
        int sjis_code = 0;
        unsigned char stmp;
        unsigned char stmp2 = 0;
        if ((ascii_code >= 0x20) && (ascii_code <= 0x2f)) stmp2 = 1;
        else if ((ascii_code >= 0x30) && (ascii_code <= 0x39)) stmp = 0;
        else if ((ascii_code >= 0x3a) && (ascii_code <= 0x40)) stmp2 = 11;
        else if ((ascii_code >= 0x41) && (ascii_code <= 0x5a)) stmp = 1;
        else if ((ascii_code >= 0x5b) && (ascii_code <= 0x60)) stmp2 = 37;
        else if ((ascii_code >= 0x61) && (ascii_code <= 0x7a)) stmp = 2;
        else if ((ascii_code >= 0x7b) && (ascii_code <= 0x7e)) stmp2 = 63;
        else { printf("bad ASCII code 0x%x\n", ascii_code); return(0); }
        if (stmp2) sjis_code = ascii_k_table[ascii_code - 0x20 - (stmp2 - 1)];
        else sjis_code = ascii_table[stmp][0] + ascii_code - ascii_table[stmp][1];
        return(sjis_code);
    }

#### What is double buffering?

Double buffering is the process of displaying one screen to the user, while the CPU/GPU is drawing the next. When the next screen is ready to be displayed, the CPU 'flips' buffers, so that the new screen is displayed, and the old screen is now the area to be drawn on. If the CPU was to draw on the screen at the same time as it was being displayed, some parts would be from the old screen and some from the new - this causes flicker and looks terrible. This is why double buffering is employed.

#### Why do I need two packet areas and ordering tables?

Strictly speaking you don't, but you'll get more speed out of your program if you do. This is related to double buffering. Examine the timeline diagram below.

    Vsync start----
        CPU busy        GPU idle
            |
            |
            |
        CPU finished    GPU begins drawing
        CPU idle                |
                                |
                                |
                        GPU finished drawing
    Wait for VSync
        |
        |
        |
    VSync start - Flip screen, go back to start.

This is very inefficient, because the GPU is idle while the CPU is busy, and the CPU is idle while the GPU is busy. But if the GPU/CPU does not complete its tasks before the VSync, then you'll get frames dropped. It would be more efficient to have both GPU/CPU going at the same time:

    VSync start----
    CPU busy on Frame 0    GPU idle                    Display Buffer 0 (currently empty)
            |
            |
            |
    CPU finished Frame 0
    Wait for VSync
        |
        |
    VSync start----
    Flip Screen             Set GPU drawing Frame 0     Display Buffer 1 (currently empty)
    CPU busy on Frame 1             | in Buffer 0
            |                       |
            |                       |
            |                       |
    Cpu finished Frame 1            |
    Wait for Vsync          GPU finishes Frame 0
        |
        |
    VSync start----
    Flip Screen             Set GPU drawing Frame 1     Display Buffer 0 (contains Frame 0)
    CPU busy on Frame 2             | in Buffer 1
            |                       |
            |                       |
            |                       |
    Cpu finished Frame 2            |
    Wait for Vsync          GPU finishes Frame 1
        |
        |
    VSync start----
    Flip Screen             Set GPU drawing Frame 2     Display Buffer 1 (contains Frame 1)
    CPU busy on Frame 3             | in Buffer 2
            |                       |
            |                       |
            |                       |
    Cpu finished Frame 3            |
    Wait for Vsync          GPU finishes Frame 2
        |
        |

     ......

This is much more efficient, because both GPU and CPU are processing in parallel. However, it also means that what is displayed on screen is *2* frames behind what the CPU is calculating, not 1, which can be confusing. The reason why two GsOTs and Packet areas are needed is that while the CPU is busy creating data in one GsOT and packet area, the GPU will be busy reading the other GsOT and packet area.

#### How do I load TIMs into VRAM?

A TIM file contains image data, like a sprite or a texture. It has to be loaded into VRAM before it can be used.  Here is a generic piece of code that will load in any TIM.

    // LoadTIM will load a TIM into VRAM.
    // Parameters: tim - Points to the very top of the TIM (i.e. including the header)
    //             pixelPos - If not NULL, this rect will be used to place the image data.
    //             clutPos - If not NULL, this rect will be used to place the CLUT data (if there is any).
    //             retImage - If not NULL, copies the image structure to this address.
    //             retSprite - If not NULL, initialises a basic sprite to this address.

    void loadTIM(u_long *tim, RECT *pixelPos, RECT *clutPos, GsIMAGE *retImage, GsSPRITE retSprite) {
        GsIMAGE image;
        RECT p;
        int i;

        GsGetTimInfo(tim + 1,&image);
        if(pixelPos) {
            image.px = pixelPos->x;
            image.py = pixelPos->y;
            image.pw = pixelPos->w;
            image.ph = pixelPos->h;
        }
        setRECT(&p,image.px,image.py,image.pw,image.ph);
        LoadImage(&p,image.pixel);

        if(image.pmode & 0x8) {
            if(clutPos) {
                image.cx = clutPos->x;
                image.cy = clutPos->y;
                image.cw = clutPos->w;
                image.ch = clutPos->h;
            }
            setRECT(&p,image.cx,image.cy,image.cw,image.ch);
            LoadImage(&p,image.clut);
        }

        if(retImage)
            memcpy(retImage,&image,sizeof(image));
        if(retSprite) {
            retSprite->attribute = (image.pmode & 3) << 24;
            retSprite->x = 0;
            retSprite->y = 0;
            switch(image.pmode & 3) {
                case 0: retSprite->w = image.pw << 2; break;
                case 1: retSprite->w = image.pw << 1; break;
                default: retSprite->w = image.pw;
            }
            retSprite->h = image.ph;
            retSprite->tpage = GetTPage(image.pmode&3,0,image.px,image.py);
            retSprite->u = image.px & 0x3f;
            retSprite->v = image.py & 0xff;
            retSprite->cx = image.cx;
            retSprite->cy = image.cy;
            retSprite->r = retSprite->g = retSprite->b = 128;
            retSprite->mx = 0;
            retSprite->my = 0;
            retSprite->scalex = retSprite->scaley = ONE;
            retSprite->rotate = 0;
        }
    }

#### How big should I make my GsSetWorkBase area?

This area contains GPU commands to be executed.When you sort sprites or objects, the libraries create the appropriate GPU commands in this area, before they get sent off to the GPU (with GsDrawOt). These GPU commands vary in size. Speaking roughly, the average size for a sprite command is 24 bytes. 3D polygons can vary from 20 bytes to 52 bytes. Most people set the length (in bytes) to be 24 \* 1000 or some arbitrary number. Try larger numbers if you are getting unexplained errors. You can make a better estimate of the size required if you know the number and type of polygons you will be drawing. Regardless of whether your polygons are lit, they will be converted into packets of the following types:

| **Type of packet**        | **Size (in bytes)** |
|---------------------------|---------------------|
| Flat Triangle             | 20                  |
| Flat Textured Triangle    | 32                  |
| Flat Quad                 | 24                  |
| Flat Textured Quad        | 40                  |
| Gouraud Triangle          | 28                  |
| Gouraud Textured Triangle | 40                  |
| Gouraud Quad              | 36                  |
| Gouraud Textured Quad     | 52                  |

**Packet sizes**

#### Why is there a solid bar at the bottom of the screen in PAL mode?

This is because the height of the screen has been set to 240, not 256. In your initialisation code, insert the following lines:

`extern DISPENV GsDISPENV;`
`GsDISPENV.screen.h=256;` (In your graphics initialisation code)

The internal variable `GsDISPENV` contains a rectangle called `screen`. You can experiment with different display sizes by tweaking this rectangle.

Setting the height to 256 will increase the size of the visible screen, but will not completely eliminate the solid bar. To completely remove it, set the height of the `screen` rectangle higher, and set the 'y' value of the `screen` rectangle lower.

#### When do I call GsSortClear?

`GsSortClear` inserts a 'Clear Screen' command into the very end of your Ordering Table. Since the GPU starts at the end and works its way to the front, your 'Clear Screen' command will be executed first. So call `GsSortClear` just before you draw the Ordering Table with `GsDrawOt`. However, it must be called *after* you have called `GsSwapDispBuff`. This is due to the way the function works with double buffering.

If you are working in Hi-Res mode (interlaced display), then don't use `GsSortClear`. Instead, set the '`isbg`' field to one when initialising your display and drawing environments.

#### Why is there a single line that doesn't clear in my 256/512 high display buffer?

There is a hardware bug in the GPU which clips drawing operations at 511 lines high. You can never draw to the last line of a 512 high screen, or the last line of the second of two 256 high screens.

When doing hi-res clearing using GsSortClear() it will only work if the screen res is 640 x 511. As a workaround, set your screen height to 255 or 511 instead of 256 or 512 when doing GsInitGraph(). That just cheats and only displays 511 lines, so you don't see the line that the clear fails on.

#### How can I use transparency in non-textured polygons?

The packets for non-textured polys don't allow you to set the transparency rate. The easy way around this is to make all your polys textured, but referencing a pure white square (can be any size, say, 4x4 pixels) as a texture. This increases drawing time slightly though, since textured polys take slightly longer than non-textured.

If the ABE bit in a non-textured poly is set, that poly will use the current transparency rate. The current transparency rate is set when you change drawing environments, and also by the last non-textured polygon to be processed. With ordering tables, you're never too sure which is the last polygon to be drawn, so you might get unexpected semi-trans rate values.

#### How can I get more CPU/GPU time?

Most programs call DrawSync(0), then VSync(0), and then swap the buffers and draw their ordering tables. VSync(0) call will wait for the TV scanline to be at the top of the frame, not when the VBlank event actually occurs (which is when the scanline is at the bottom of the frame). This is due to a hardware GPU bug which occurs in interlaced mode, and VSync(0) was written to get around this. However, this waiting can take up to 10% of potential CPU time. If you are not using interlaced mode, you can gain about 8-10% more CPU/GPU time, by having the VSyncCallback reset a flag which allows your program to continue. Basically, you're rewriting your own VSync(0) function using VSyncCallback. This isn't overly complex, but don't do this if you're a beginner.

Put this as a global variable:

`volatile int waitForVSync = 0;`

In your normal code where you'd normally call DrawSync(0); VSync(0), put this:

```c
DrawSync(0);
waitForVSync = 1;
while(waitForVSync == 1);
waitForVSync = 2;
GsSwapDispBuf()...etc
```

In your VSyncCallback, put this:

```c
if(waitForVSync == 1)
    waitForVSync = 0;
```

## 3D Graphics

#### What do all the 3D related functions do?

- `GsDefDispBuff()`, `GsMapModellingData()`, `GsInitGraph()`, `GsSetAmbient()`, `GsSetFlatLight()`, `GsInitCoordinate2()` and `GsInit3D()` are all functions that initialise data so it can be used later on. They only have to be called once at the start of your program.
- `GsSetWorkBase()`, `GsClearOT(),` `GsDrawOT()`, `GsSwapDispBuff()` and `GsSortClear()` are all functions concerned with the ordering tables and packet buffers (see elsewhere in this FAQ for more information on these concepts). All of these will need to be called (usually once) every time you create a new frame to display.
- `GsSortObject4()` will put a 3D object into the current Ordering Table using the current translation/rotation settings in the GTE. You call this once for every 3D object in your scene. If you have the same object in two different places, you would call this twice (changing the current translation/rotation settings in-between calls, of course).
- `GsGetLs()`, `GsGetLws()`, `GsSetLs()`, `GsSetLightMatrix()` are all functions which calculate or set the current translation/rotation settings in the GTE that are used by `GsSortObject4()`. These functions use a `GsCOORDINATE2` structure to store values.
- `RotMatrix()` is a function which creates a rotation matrix, which can be used in the GTE to rotate objects. Most people store 6 numbers which designate the X, Y and Z rotation values for their object and the X, Y and Z position (translation) of their object, then use RotMatrix to create the necessary matrix which is held inside a `GsCOORDINATE2` structure. The 4x3 matrix in `GsCOORDINATE2` will consist of a 3x3 result from RotMatrix (the rotation) and the 1x3 vector which is identical to the translation vector.

For a more detailed explanation of some of these functions, see the Yaroze Vectors and Matrices tutorial at: [http://www.netyaroze-europe.com/~jruss1/](https://web.archive.org/web/1999/http://www.netyaroze-europe.com/~jruss1/index.html)

#### Are there any tutorials which explain Yaroze 3D and matrices?

Try [http://www.netyaroze-europe.com/~jruss1/](https://web.archive.org/web/1999/http://www.netyaroze-europe.com/~jruss1/index.html) for a tutorial on matrices and vectors. In addition, it tells you how the GTE works, and how the Playstation libraries manage hierarchal coordinate systems.

#### Why am I getting strange results with RotMatrixX, RotMatrixY and/or RotMatrixZ?

The RotMatrix function takes a vector of angles and creates a rotation matrix from scratch, with the rotation order Z-\>Y-\>X. However, the RotMatrixX/Y/Z functions (which generate just one type of rotation) do not build the matrix from scratch, but instead multiplies the passed matrix by an appropriate rotation matrix. So say you'd built a rotation matrix using RotMatrix, but you then wanted to rotate about the Z axis again, your code would look like:

    RotMatrix(&myRotationVector, &myRotationMatrix);
    RotMatrixZ(additionalZRotateAngle, &myRotationMatrix);

If, on the other hand, you didn't want to use RotMatrix first, then you'd have to make "myRotationMatrix" the identity matrix (a matrix of no transformations at all):

    MATRIX myRotationMatrix = { { 1, 0, 0 },
                                { 0, 1, 0 },
                                { 0, 0, 1 } };
    RotMatrixZ(additionalZRotateAngle, &myRotationMatrix);

#### My object rotation is really strange - it works OK sometimes, but rotates at really weird angles at other times. What's happening?

This is the "gimbal lock" problem manifesting itself. If you're using RotMatrix to create your rotation matrix then the rotation matrix it creates is a transformation about the Z axis first, then the Y axis and finally the X axis. This can produce some strange results. For instance, take a pen and hold it straight up. Rotate it 90 degrees about the X axis, then 90 degrees about the Y. It should be horizontal from left to right in front of you. Now put it back upright and rotate it 90 degrees about the Y, then 90 degrees about the X. It should be facing towards/away from you! See how the order of rotation matters? If you want a different order, then you can use the functions RotMatrixX, RotMatrixY and RotMatrixZ to create a rotation matrix with your desired order. Another way to solve this is to use quaternions, which is a lot more complicated. Look for references to it on the Internet, there are some online tutorials about.

#### How do I place a 2D sprite in a 3D world?

In a 3D game, sometime you want to insert sprites into the world as if they were true 3D objects. For instance, in a racing game, you might want to make all the trees at the side of the road sprites instead of true 3D objects.

If you choose this method, you must first have calculated the X/Y/Z position of your sprite in the 3D world. The following code fragment will calculate the required screen coordinates and scale, and insert it into the Ordering Table for you. This code fragment requires 2 constants - the Projection Distance (the value that you use as the parameter to GsSetProjection()) and the Ordering Table length (from 1 to 14).

*(This code sample by Harvey C and James Russell)*

    void draw_3dsprite (GsSPRITE *sprite,VECTOR spritePosition) {
        // The spritePosition is the point in 3D space (world coords) that your sprite lies at.

        VECTOR transformedPosition;
        extern MATRIX GsWSMATRIX;

        ApplyMatrixLV(&GsWSMATRIX,&spritePosition,&transformedPosition);
        transformedPosition.vx += GsWSMATRIX.t[0];
        transformedPosition.vy += GsWSMATRIX.t[1];
        transformedPosition.vz += GsWSMATRIX.t[2];

        if (transformedPosition.vz>0) {    // Only draw sprite if it's in front of the camera.
                // Scales the sprite according to the distance.
            sprite->scalex = sprite->scaley = (ONE << 12) / transformedPosition.vz;
            sprite->x = transformedPosition.vx * PROJECTION_DISTANCE / transformedPosition.vz;
            sprite->y = transformedPosition.vy * PROJECTION_DISTANCE / transformedPosition.vz;
            GsSortSprite(&sprite,&OTable_Header[GsGetActiveBuffer()],transformedPosition.vz >> (14 - ORDERING_TABLE_LENGTH));
        }
    }

#### Why do I get strange UVs when I use RSDLink?

The UVs specified in the RSD's `.MAT` file are relative to the top left of the texture's image data, not the texture page (unlike normal GsSprites).

#### Why can't I see my 3D model?

There are a number of possible reasons.

1.  If your model is textured, you must load the textures into VRAM (using a function such as loadTIM listed above) first. You only have to do this once, on initialisation. The textures are stored in the TIM files, not in the TMD file.
2.  You might be setting all your lights to black. Try creating an unlit model first.
3.  The model might be so small or so big that all the points translate to either the center of the screen or so far outside it that all the polygons are clipped.
4.  You camera might be facing in the wrong direction, or could be in the middle of the model. If your polygons are not double sided, you won't be able to see anything if the camera is in the middle.
5.  The object is getting clipped. The clipping mechanism will clip any polygon that has a negative screen Z value.
6.  You haven't set up your coordinate systems and/or the GTE properly.
7.  You haven't set the projection distance in the GTE.

#### Why am I'm getting errors like "GPU CODE xxH not supported" or "GPU CODE xxH not assigned"?

There are 3 possible problems:

1.  Your TMD is getting corrupted in memory (being overwritten accidentally by the main program).
2.  Your packet creation area (the buffer you set every frame with GsSetWorkBase() ) is not large enough. Roughly speaking, a single flat shaded textured model will take up a maximum of 32\*NUM_POLYGONS bytes in the packet area (see "How big should I make my WorkBase?" for more detailed information). If this area isn't big enough, you can get some strange GPU code errors as one buffer overflows into another.
3.  Secondly, due to a bug in the libraries, a few TMD primitives don't work. These are:
    - 3D sprites (sprites within a TMD)
    - Lines within a TMD
    - Double sided polygons
    - Non-textured Gouraud (gradient) quads

    If you want lines within the TMD, the easiest solution is to replace them with a thin triangle or quad.

## Sound

#### What's the deal with MIDI on the PSX?

The standard VAB (found on the boot up CD) follows the GM (General Midi) standard. GM was introduced by Roland (who make audio-production stuff, synths etc) so that compositions recorded on one synth can play with the correct sounds on another synth. (As long as they are both synths are GM.)

All GM synths have the same first 128 instruments (programs).  Not all GM synths sound the same, but at least the sounds will be similar.  GM is like the synth equivalent to ASCII.

GM synths tend to have much more memory for their wave tables than the PSX, which means that the standard vab has a lot of duplication of sounds.  E.g. many of the guitar sounds in the standard vab sound identical, whereas on a synth there would be subtle differences between them.

Fortunately, there are a lot of people with GM synths and soundcards, and there are thousands of tunes to be found on the net in GM format.  So, you can download a midi file, convert it to a .SEQ file, and play it without modification on the PSX using the standard VAB.

But, because of the shortage of wave table memory on the PSX, the tunes hardly ever sound as good as they should.  Having said that though, it's unlikely that a tune will use anything like the 128 GM programs available.  This means that you can build banks of sounds yourself which only contain the sounds used in the midi file.

```text
 TABLE 1 - General MIDI Instrument Patch Map (groups sounds into
        sixteen families, w/8 instruments in each family)
┌───────────────────────────────┬────────────────────────────────┐
│(1-8        PIANO)             │(9-16      CHROM PERCUSSION)    │
│1         Acoustic Grand       │9        Celesta                │
│2         Bright Acoustic      │10        Glockenspiel          │
│3         Electric Grand       │11        Music Box             │
│4         Honky-Tonk           │12        Vibraphone            │
│5         Electric Piano 1     │13        Marimba               │
│6         Electric Piano 2     │14        Xylophone             │
│7         Harpsichord          │15        Tubular Bells         │
│8         Clav                 │16        Dulcimer              │
├───────────────────────────────┼────────────────────────────────┤
│(17-24      ORGAN)             │(25-32      GUITAR)             │
│17        Drawbar Organ        │25        Acoustic Guitar(nylon)│
│18        Percussive Organ     │26        Acoustic Guitar(steel)│
│19        Rock Organ           │27        Electric Guitar(jazz) │
│20        Church Organ         │28        Electric Guitar(clean)│
│21        Reed Organ           │29        Electric Guitar(muted)│
│22        Accoridan            │30        Overdriven Guitar     │
│23        Harmonica            │31        Distortion Guitar     │
│24        Tango Accordian      │32        Guitar Harmonics      │
├───────────────────────────────┼────────────────────────────────┤
│(33-40      BASS)              │(41-48     STRINGS)             │
│33        Acoustic Bass        │41        Violin                │
│34        Electric Bass(finger)│42        Viola                 │
│35        Electric Bass(pick)  │43        Cello                 │
│36        Fretless Bass        │44        Contrabass            │
│37        Slap Bass 1          │45        Tremolo Strings       │
│38        Slap Bass 2          │46        Pizzicato Strings     │
│39        Synth Bass 1         │47        Orchestral Strings    │
│40        Synth Bass 2         │48        Timpani               │
├───────────────────────────────┼────────────────────────────────┤
│(49-56     ENSEMBLE)           │(57-64      BRASS)              │
│49        String Ensemble 1    │57        Trumpet               │
│50        String Ensemble 2    │58        Trombone              │
│51        SynthStrings 1       │59        Tuba                  │
│52        SynthStrings 2       │60        Muted Trumpet         │
│53        Choir Aahs           │61        French Horn           │
│54        Voice Oohs           │62        Brass Section         │
│55        Synth Voice          │63        SynthBrass 1          │
│56        Orchestra Hit        │64        SynthBrass 2          │
├───────────────────────────────┼────────────────────────────────┤
│(65-72      REED)              │(73-80      PIPE)               │
│65        Soprano Sax          │73        Piccolo               │
│66        Alto Sax             │74        Flute                 │
│67        Tenor Sax            │75        Recorder              │
│68        Baritone Sax         │76        Pan Flute             │
│69        Oboe                 │77        Blown Bottle          │
│70        English Horn         │78        Skakuhachi            │
│71        Bassoon              │79        Whistle               │
│72        Clarinet             │80        Ocarina               │
├───────────────────────────────┼────────────────────────────────┤
│(81-88      SYNTH LEAD)        │(89-96      SYNTH PAD)          │
│81        Lead 1 (square)      │89        Pad 1 (new age)       │
│82        Lead 2 (sawtooth)    │90        Pad 2 (warm)          │
│83        Lead 3 (calliope)    │91        Pad 3 (polysynth)     │
│84        Lead 4 (chiff)       │92        Pad 4 (choir)         │
│85        Lead 5 (charang)     │93        Pad 5 (bowed)         │
│86        Lead 6 (voice)       │94        Pad 6 (metallic)      │
│87        Lead 7 (fifths)      │95        Pad 7 (halo)          │
│88        Lead 8 (bass+lead)   │96        Pad 8 (sweep)         │
├───────────────────────────────┼────────────────────────────────┤
│(97-104     SYNTH EFFECTS)     │(105-112     ETHNIC)            │
│97        FX 1 (rain)          │105       Sitar                 │
│98        FX 2 (soundtrack)    │106       Banjo                 │
│99        FX 3 (crystal)       │107       Shamisen              │
│100        FX 4 (atmosphere)   │108       Koto                  │
│101        FX 5 (brightness)   │109       Kalimba               │
│102        FX 6 (goblins)      │110       Bagpipe               │
│103        FX 7 (echoes)       │111       Fiddle                │
│104        FX 8 (sci-fi)       │112       Shanai                │
├───────────────────────────────┼────────────────────────────────┤
│(113-120    PERCUSSIVE)        │(121-128     SOUND EFFECTS)     │
│113        Tinkle Bell         │121       Guitar Fret Noise     │
│114        Agogo               │122       Breath Noise          │
│115        Steel Drums         │123       Seashore              │
│116        Woodblock           │124       Bird Tweet            │
│117        Taiko Drum          │125       Telephone Ring        │
│118        Melodic Tom         │126       Helicopter            │
│119        Synth Drum          │127       Applause              │
│120        Reverse Cymbal      │128       Gunshot               │
└───────────────────────────────┴────────────────────────────────┘
```

TABLE 1 - General MIDI Instrument Patch Map (groups sounds into sixteen families, w/8 instruments in each family)

```text
TABLE 2 - General MIDI Percussion Key Map (assigns
drum sounds to note numbers. MIDI Channel 10 is for
                    percussion)
┌──────────────────────────┬──────────────────────┐
│MIDI Drum Sound Key       │MIDI Drum Sound Key   │
├──────────────────────────┼──────────────────────┤
│35     Acoustic Bass Drum │59      Ride Cymbal 2 │
│36     Bass Drum 1        │60      Hi Bongo      │
│37     Side Stick         │61      Low Bongo     │
│38     Acoustic Snare     │62      Mute Hi Conga │
│39     Hand Clap          │63      Open Hi Conga │
│40     Electric Snare     │64      Low Conga     │
│41     Low Floor Tom      │65      High Timbale  │
│42     Closed Hi-Hat      │66      Low Timbale   │
│43     High Floor Tom     │67      High Agogo    │
│44     Pedal Hi-Hat       │68      Low Agogo     │
│45     Low Tom            │69      Cabasa        │
│46     Open Hi-Hat        │70      Maracas       │
│47     Low-Mid Tom        │71      Short Whistle │
│48     Hi-Mid Tom         │72      Long Whistle  │
│49     Crash Cymbal 1     │73      Short Guiro   │
│50     High Tom           │74      Long Guiro    │
│51     Ride Cymbal 1      │75      Claves        │
│52     Chinese Cymbal     │76      Hi Wood Block │
│53     Ride Bell          │77      Low Wood Block│
│54     Tambourine         │78      Mute Cuica    │
│55     Splash Cymbal      │79      Open Cuica    │
│56     Cowbell            │80      Mute Triangle │
│57     Crash Cymbal 2     │81      Open Triangle │
│58     Vibraslap          │                      │
└──────────────────────────┴──────────────────────┘
```

TABLE 2 - General MIDI Percussion Key Map (assigns drum sounds to note numbers. MIDI Channel 10 is for percussion)

##### General MIDI minimum sound module specs

- **Voices**: A minimum of either 24 fully dynamically allocated voices available simultaneously for both melodic and percussive sounds or 16 dynamically allocated voices for melody plus eight for percussion.
- **Channels**: General MIDI mode supports all sixteen MIDI channels. Each channel can play a variable number of voices (polyphony). Each channel can play a different instrument (timbre). Keybased Percussion is always on Channel 10.
- **Instruments**: A minimum of sixteen different timbres playing various instrument sounds. A minimum of 128 preset for Intruments (MIDI program     numbers).
- **Note on/Note off**: Octabe Registration: Middle C(C3) = MIDI key 60. All Voices  including percussion respond to velocity.
- **Controllers**:

  `Controller #   Description`
  `1              Modulation`
  `7              Main Volume`
  `10             Pan`
  `11             Expression`
  `64             Sustain`
  `121            Reset All Controllers`
  `123            All Notes Off`
- `Registered      Description`

  `Parameter #`
  `0               Pitch Bend Sensitivity`
  `1               Fine Tuning`
  `2               Coarse Tuning`

#### How do I pause a sound?

Set the pitch to 0 to pause it, then reset it back to the original pitch to continue.

#### Can I play MOD files?

There are some MOD-\>MIDI converters available on the Internet, but the results tend to be less than impressive. There is a utility called DMS play (created by Yaroze member Elliot Lee) which allows you to play MODs.

#### My sample seems off-key.

If a sample is very short it will sound off-key when played. Try upsampling the sample so that it is longer. Programs such as FastTracker or GoldWave can do this.

## C & Compiling

#### What is the DCache?

The DCache is 1K (1024 bytes) of RAM starting at address `0x1f800000`. This address is returned by the macro `getScratchAddr(0)`. It is very fast RAM, in that a read/write to/from it takes 1 cycle instead of 6 or 8. Your programs are free to use it, but beware that some library calls use it as well (most don't).

#### How can I set the stack to the DCache?

Running with the stack on DCache will speed up some applications, but don't overflow the stack! It's only 1K long, remember.

To save the current stack pointer:

        __asm__ volatile ("sw $29,(savesp)");
        __asm__ volatile ("la $29,0x1f8003f0");

and to restore it:

        __asm__ volatile ("lw $29,(savesp)");

#### The GNU compiler crashes with a stack error! How do I fix it?

The GNU compiler needs a lot of stack space when the optimisation level is high. Type "`set GO32=dpmistack 2000000`" on the command line to give it more room. Put this line in your `autoexec.bat` file to make the change permanent. You can set this number higher if it still crashes.

#### I am running out of environment space! How do I set more?

In your `config.sys` file, insert the line: `SHELL=command.com /E:1024 /p` This will set your environment space to 1024 bytes.

#### How can I include and reference data in my C program directly, instead of uploading it separately with SIOCONS?

There are a number of utilities which can convert raw data into a form that you can \#include or compile on its own. They turn the data into C source (Eg: `char MyRawData[] { 0x12, 0x23, 0x34...` ) which you can reference from your program with the variable name `MyRawData`. Try Raw2Src.exe or Raw2C.exe. The downside to this is that if the data is destined for a once-only upload to VRAM or SPU RAM, it is not as simple to reuse the area it occupies. But it is useful for testing or small programs.

#### Is there anything to help me manage the addresses I place my data?

Try DATAMAN at [http://www.netyaroze-europe.com/~jruss1/](https://web.archive.org/web/1999/http://www.netyaroze-europe.com/~jruss1/index.html) This will create SIOCONS scripts for you and \#include-able C source which contains macros of the data addresses, which it calculates automatically.

#### My data comes after my program, but I'm not sure exactly where to start placing it.

If you get the compiler to generate a MAP file, it will contain the addresses and lengths of all the different parts of your executable. You can use this information to calculate the first address of your data.

#### What is fixed point arithmetic?

Computers represent most numbers as integers - there is no fractional part to a number. An integer is a whole number like 4, 3, 0, -10, 22 and so on. Sometimes, however, you need a fractional part. For example, if you were calculating the square root of 2, the answer in integers is 1, but you'd probably want more accuracy than that! The more fractional digits you have, the more accurate your calculations will be. Floating point numbers (`float` or `double` in C) store as many fractional digits as is possible in 32/64 bits, but they are rather slow to process. A much faster method is to use fixed point arithmetic.

To use fixed point arithmetic, you designate how many bits of accuracy you would like in your number. Then you divide up your standard integer (of 32-bits) into a integer part and a fractional part. As an example, here is a 32 bit integer with 12 bits of fractional accuracy.

| Bits 31-12   | Bits 11-0       |
|--------------|-----------------|
| Integer part | Fractional part |

There is an *implicit* decimal point between the integer and fractional part.

A standard unsigned integer represents each bit as follows:

2³¹, 2³⁰, 2²⁹...  ... 2³, 2², 2¹, 2⁰

We have shifted the standard exponents to:

2¹⁹, 2¹⁸, 2¹⁷...   ....2³, 2², 2¹, 2⁰, 2⁻¹, 2⁻², 2⁻³  ....    .....,2⁻⁸, 2⁻⁹, 2⁻¹⁰, 2⁻¹¹

So the largest unsigned number we can represent in this format is 2²⁰ -1, not 2³² -1

To represent a number in this format, some bit shifting is required. If you wish to represent the number 1.0, then that means you want the fractional part to be all 0, and the integer part to contain '1'. So storing 1.0 in this format would be the same as 1\<\<12 (1 shifted left by 12 bits). If you wanted to store 0.5 in this format, then the integer part part will be 0 and the fractional part will be (in binary) 100000000000. This is the same as (1 \<\< 12) / 2.

This explains why the Playstation considers angles to be 360 degrees and sprite scales to be normal (1.0) at the integer value of 4096 - it is just employing fixed point arithmetic on the angles and scales with a precision of 12 bits. (4096 = 1 \<\< 12);

##### Fixed point arithemetic operations

For the most part, arithemetic operations remain basically the same. Assuming that the level of accuracy is shared by all the numbers below, and that accuracy is PREC bits, then:

| **Operation** | **The C operations necessary** |
|----|----|
| Conversion of an integer/float/double to fixed point: `A` | `A * (1 << PREC)` |
| Addition: `A + B` | `A + B` |
| Subtraction: `A - B` | `A - B` |
| Multiplication: `A * B` | `(A * B) / (1 << PREC)` |
| Division: `A / B` | `(A / B) * (1 << PREC)` |
| Conversion to integer: `A` | `A / (1 << PREC)` |

While it looks inefficient to have all those shifts and divides and multiplies in the C operations, the compiler will translate them to arithmetic shifts, which are very fast.

#### I want to perform a maths function like `sin` or `cos`, but CodeWarrior complains that something is undefined.

You should link the "Math-ISA1-noFPU-LE.lib" to your executable. Goto: "Project" / "Add files..." and search for this LIB-file.

#### Why are sin/cos/tan so slow? How can I speed them up?

These functions use floating point emulation to create an accurate result. However, most games are not always after accuracy, they want speed instead! The best method is to create a lookup table which uses fixed point arithmetic (see above). You can take advantage of the fact that the sin(x) = cos(x-90), and also that the cosine function is symmetric about the Y axis to keep the table small. Another optimisations you can make is the fact that most functions require both the sine and cosine of the same angle, so interleaving the two tables can increase speed because you're only doing one lookup instead of two.

#### What is a good way of conditionally compiling debugging strings?

Often in your programs it is useful to print out certain values, but when you want to make you code run fast for the 'proper' version, you don't want to compile these printfs in your program. Traditionally, the way to do this was to define a dummy macro like `#define DEBUG`, then then have lots of:

    #ifdef DEBUG
        printf("The value of foo is %d\n",foo);
    #endif

A cleaner way is still define the dummy macro DEBUG, but redefine printf (as seen below). This method makes your code cleaner and easier to read, because it's not full of \#ifdefs and \#endifs.

`#define DEBUG`

```c
#ifdef DEBUG
#define dprintf(x) printf x
#else
#define dprintf(x)
#endif
```

You can then have lines like (note the parentheses surrounding the 'normal' function call parentheses:

`dprintf( ("The values of foo is %d\n",foo) );`

#### What are these 'text', 'data' and 'BSS' sections I keep seeing?

The compiler divides compiled code into 3 general areas: 'Text' is the machine code that performs all the actual operations. 'Data' is the area that contains initialised data like strings, numbers, floats, etc. 'BSS' contains uninitialised data. It stores all three sections in the final executable, but since the BSS section is all zeroes anyway it just store the \_length\_ of the BSS section, to save on file space. When the executable is loaded, it 'expands' the BSS section again. This is why your executable may be 100K in size, but appears to take up 150K of RAM. This expansion of the BSS section can overwrite data placed by unwary users. Use the MAP file generated by the compiler to accurately see where the different sections starts and ends.

#### What does the 'static' keyword in C mean?

If applied to a function name or global variable, then that function/variable is only visible to functions inside that C source file. Functions outside that C source file cannot access it, even if they '`extern`' it. If applied to a variable inside a function, then that variable becomes 'persistent'. That means that if you exit the function and call it again, the value of the variable is not lost. Only that function can access that variable. Eg:

    int A(void) {
        static int banana = 0;
        banana ++;
        return banana;
    }

    void main(void) {
        printf("Banana = %d\n",A());
        printf("Banana = %d\n",A());
        printf("Banana = %d\n",A());
    }

will print:

    1
    2
    3

If the variable was not static, it would print:

    1
    1
    1

Making a local variable 'static' also means that the compiler will not allocate space for it on the stack. Instead, it will place it in the text section. Since it is very easy to overflow the stack with a large variable, making that variable static will move it off the stack which can help in most cases.

#### I want to send some examples of my work to games companies - what do I have to do?

Most developers will *not* have a Yaroze, as they will have the full professional development kits. If you want to send them examples of your work, it may be better and more reliable to send a VHS video tape of your programs in action. Depending on how you've coded your demo, it may take them a while to convert it so it will work with the professional development kit, so the video tape option saves both you and them a lot of time (they may not want to spend a couple of  hours fiddling with your code!). Sending source code has ***many complications*** so it is ***highly recommended*** you use the video tape option.

If you want to send them a program anyway, or if they request the source as proof of authorship, then follow the guidelines below:

- Don't assume that professional developers will be using CodeWarrior. If possible, get it to compile with GCC. There are a few important differences between the file-serving capabilities of the professional development kit and CodeWarrior's method, so it is highly recommended that you do not use any file serving.
- Programs and data can start at `0x8001000` instead of `0x80090000`.
- Don't assume that professional developers know the difference between the Yaroze and the professional development kits. Most know about the Yaroze in general, but don't know the details. It *is* possible to run your programs on their machines, with some relatively minor adjustments to your code.
- Controllers are handled in slightly different way in the developers libraries. If you send source code, move all your controller code to one file, and let them know that this file will have to be adjusted and recompiled. The controllers are initialised automatically in the Yaroze, but not in the professional libraries. Allow for this by including a function InitialisePad() which does nothing, but to which they can add a call to the function "`PadInit(0)`". Let them know that this has to be done. `GetPadBuf()` does not exist in the professional libraries. Instead, let them know that they have to use the function `PadRead(`\<controller number\>`)` to get the button status. It is best to code your program in such a way that the controller functions are abstracted into small functions, because this will allow them to easily recompile your code.
- The single `#include <libps.h>` file is split up into many different `.h` files in the professional system. Let them know that these will have to be changed. It would be better if you replaced `#include <libps.h>` with `#include "libraries.h"`, and the file "`libraries.h`" had the single line "`#include <libps.h>`". This means they only have to change one file (`libraries.h`) to replace all the libraries.
- If you are wary about sending source code, you can compile it (but not link it) and send the object files instead. This is not possible with controller code though.
- The `make` utility is different, and some developers use their own. To avoid a conflict with makefile syntax, use a single batch file which compiles everything.
- The compiler that professional developers use is called `ccpsx`, but the command line options are basically the same as the GNU compiler.
- Instead of using `dload` or the `PSComUtil` to move data into the development kit, try to include all your code in one big program instead of a main program and lots of individual data files (see above for hints on how to do this). You can take advantage of the fact that the full developers kit has 8 megabytes of RAM instead of 2.

If you're unsure if your code will work, then (time permitting) Yaroze Support can help. If you ***follow the guidelines above***, we can confirm whether your code will run on a development kit.

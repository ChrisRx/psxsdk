---
title: Yaroze Member FAQ
weight: 2
---

1.  How do I determine where to load my TIM, TMD and Sound files?

    Data files can be loaded into memory anywhere within the available memory range. However, most of the time the data is loaded at the beginning of user memory with the program loaded after the last data block. Remember that the Yaroze library uses the first 0x90000 bytes of main memory, so the first available address is 0x80090000.

2.  How do I use the memory card to store the default baud rate?

    Information on this is available on the Yaroze web site in the file BAUDRATE.ZIP located in the MISC section of the file area.

3.  Are there any naming conventions established for the Net Yaroze when using the memory card to store game information?

    Please follow the conventions outlined below. While not mandatory, commercially released games expect save files to be in this format, and may behave unpredictably if they encounter a Memory Card with a file that is not.

    ```text
    File Names
    =========

    Use the following structure for file names.

    Bytes   Contents        Notes
       0    Magic Number    Always 'B'
       1    Region          Japan: 'I' (*1)
                            North America: 'A'
                            Europe: 'E'
     2-11   "NETYAROZE"
    12-20   User/Public     Use only non-0x00,
                            0x2a(*),0x3f(?) ASCII.
                            End with 0x00.

    *1: None are checked by the system
    ```

    Example: If the product code is SLPS-00001, the file name's first 12 characters are BISLPS-00001. Always add zeros to make the numerical portion 5 digits.

    ```text
    File Headers
    ==========

    Put the following headers at the start of each file.

    Bytes   Contents        Notes
      2     Magic number    Always 'SC'
      1     Type            See "Type Field" table below
      1     Number of slots
     64     Text name       Shift JIS, (*1)
     28     Pad
     32     CLUT
    128     Icon image (1)  16 x 16 x 4 bits
    128     Icon image (2)  Type: 0x12, 0x13 only
    128     Icon image (3)  Type: 0x13 only
    128 x N Data            Varies

    *1: Non-kanji and primary standard kanji only, full-size 32 characters.

    Type Field
    ========

    Type    Number of icon images (automatically replaced animation)
    0x11    1
    0x12    2
    0x13    3
    ```

4.  How do I read the controller?

    The controllers are read automatically during the vertical blank interval. All you need to do is provide a pair of variables to store the controller state and register those variables with the Yaroze libraries. Like this:

    ```c
    volatile u_char *Cont0;
    volatile u_char *Cont1;

    GetPadBuf(&Cont0, &Cont1);
    ```

    Once this has been done, the controllers can be referenced as Cont0 and Cont1.

    For more information see Section 12, "Peripheral Devices Management," in the Yaroze User Guide.

5.  In the sample programs I see ResetGraph(3). The manual does not document what the value of 3 is supposed to do.

    ResetGraph(3) is essentially the same as ResetGraph(0) except that the current screen mode is maintained and the screen is not cleared.

6.  Why can't I see all the pixels on my TV screen? Part of the image is cut off on all sides.

    Most TV’s do not display the entire image broadcast to them. The size of the border varies from set to set, but for 320x240 mode it can be as much as 27 pixels on all sides. For this reason, it is recommended that you not display information critical to the game in these areas. A good rule of thumb is to limit your game play to the center 266x200 pixels.

7.  What can I do if I get "Out of environment space" when I try to build a project?

    There are a couple of ways to increase your environment space, depending on your operating system. The easiest is to add "/e:4096" to the SHELL= line in your CONFIG.SYS file. If you do not have a "SHELL=" line in your config.sys file, you can add one. The format for the command is:

    `SHELL=<full path to command.com> /E:<memory size> /p`

    If your COMMAND.COM is at the root of your C: drive, the command would be:

    `SHELL=C:\command.com /E:4096 /p`

    You may also need to add the following line to your AUTOEXEC.BAT file:

    `SET GO32=DPMIStack 500000`

    After making these changes, you must reboot your computer before they will take effect.

8.  What is an Ordering Table?

    An Ordering Table is a list of items to be drawn on the screen. The Ordering Table can also implement a simple Z-buffer algorithm to ensure that ‘closer’ objects are not drawn under objects that are ‘deeper’ into the screen. You can think of an Ordering Table as a series of transparent layers laid on top of the other. Any number of objects can be drawn on each of the layers. The layers are drawn from the back toward the front.

    For more information see Section 6, "Integrated Graphics," in the Yaroze User Guide.

9.  What is a Packet?

    A packet is sometimes referred to as a primitive. It is the smallest unit that can be dealt with by the GPU.

    For more information see Section 6, "Integrated Graphics," in the Yaroze User Guide.

10. In the sample programs, the packet area is defined as:

    `PACKET GpuPacketArea[2][PACKET_CNT*(20+4)];`

    What does this mean?

    Four (4) bytes are used for the tag area of the packet. Twenty (20) bytes is the largest possible primitive packet, hence, (20+4) bytes per packet. You may use a smaller value than 20, like (n+4) where n is the size of the largest possible packet in your models. Information on the number and size of the primitives is reported when you run the RSD2TMD tool. Using these values, you can calculate the largest memory block taken by all the primitives in your world and use that area as the GS packet area.

11. Why does the screen flicker in interlace mode?

    In interlace mode, the entire screen must be drawn between Vsync’s. In NTSC mode this means that you must be able to draw the entire screen 60 times a second. If not, flicker will occur.

    For more information see Section 5, "Frame Buffer Access," in the Yaroze User Guide.

12. What is a texture page, and why does the Yaroze system use them?

    A tpage is a memory area in the frame buffer that stores images used for texturing a 3D model, or 2D sprite. They are used to make it easier to reference a portion of the frame buffer. A texture page ID is simply a form of shorthand for referring to a specific rectangular block of the frame buffer using a single byte rather than a complete set of coordinates.

    This also allows the texture UV coordinates to be specified as 8-bit values that reference an offset from the start of the texture page, instead of 16-bit values that reference an offset into the frame buffer as a whole.

13. How do I create a 3D object?

    3D objects can be created using a variety of modeling and/or CAD programs. Any program that can output a 3D DXF file can be used. Once you have created a DXF file, you run the program DXF2RSD.EXE to convert it into a format usable with the Yaroze.

    For more information see Section 14, "Graphic Tools," in the Yaroze User Guide.

14. I’ve set the ‘fce’ flag in the TMD structure to create double-sided polygons, but it doesn’t work. What’s wrong?

    The ‘fce’ flag and double-sided polygons are not supported on the Yaroze system. However, when you are converting the DXF file to an RSD file, you can use the –both option to simulate double-sided polygons. This option works by creating two polygons (one for each side) for each polygon in the original model.

15. How do I draw lines in 3D?

    There is no library function for drawing 3D lines. What you need to do is map the endpoints of the line to the screen and draw a 2D line (using the GsSortLine() function) connecting the points. If you are connecting the vertices of a 3D object with lines, you can use the function GsGetLs() to obtain a local-to-screen transformation matrix. This matrix could then be applied to the vertex coordinates to obtain the endpoints of the line.

16. How can I mix 3D objects with 2D sprites?

    The best way to do this is to create a single polygon for each 2D sprite and texture it with the sprite image. This method makes it easier to scale, rotate and otherwise fit the sprite into the 3D environment.

17. I tried to test my sound FX with VABPLAY but it doesn’t work. What’s wrong?

    VABPLAY uses a .SEQ file to key the sounds and is used primarily for testing your game music. If you want to test your sound FX, you’ll need to create a VAB containing the sound FX. Load the VAB in SPU using SsVabTransfer() and save the return VabId. Set the main volume using SsSetMVol() and use SsUtKeyOn() with proper parameters (including the VabId) to play the sound effect. (Please refer to LibSound.c and LibSound.h in the game Survival for a simple implementation of playing background music and sound FX).

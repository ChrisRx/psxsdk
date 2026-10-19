---
title: Manual Additions: Peripherals
weight: 4
---

There are two types of peripheral to consider in the Net Yaroze environment: Controllers and Memory cards.

[\[Names of Peripherals\]](#names-of-peripherals) [\[Controller Recommendations\]](#controller-recommendations) [\[Analog Joystick\]](#analog-joystick) [\[neGcon\]](#negcon) [\[Memory Cards\]](#memory-cards)

## Names of Peripherals

| **Right**          | **Wrong**           |
|--------------------|---------------------|
| Memory card        | RAM card            |
| Controller         | Pad                 |
| Link cable         | Combat cable        |
| Multi tap          | Multi-tap, Multitap |
| Analog Joystick    | Flight Stick        |
| Mouse              | -                   |
| Directional button | D-PAD, Cross button |
| Controller port    | Controller slot     |
| Memory card slot   | Memory card port    |
| neGcon             | Negcon              |

## Controller Recommendations

The following conditions should be met by the game for it to pass the tests placed upon it by the European Quality Assurance Group, and are recommended for Net Yaroze Members.

- The game must be playable with the standard Controller. Controller Port 1 always takes precedence over Port 2. So in the case of a single player game, the game only responds to Controllers connected to Port 1.
- If a Controller is plugged in that is not supported by the game it should be ignored. For example, if a neGcon is plugged into a game which requires a standard pad, the neGcon input should be ignored.
- The game should recognise specific Controllers from the Controller option screen and include calibration options, if they are required. For example the, analogue controls should be adjustable for sensitivity. It should be possible to reconfigure a game to any attached peripheral.
- If a controller is removed during play the game should go into a pause state, and maintain that state while the Controller is reconnected or a different Controller connected. If the newly connected Controller needs configuring, this should be an option.

Most of the sample code supplied only supports a single Controller and does not detect the type of Controller connected - in a commercial title this would fail QA tests.

## Analog Joystick

Physically, the Analog Joystick consist of two standard joysticks, linked by a control panel with standard switches. The joystick can act in either digital or analogue mode.
Make sure you include a Calibration menu (0 position calibration, idle movement, sensitivity etc.) in your application.

### Analog Joystickr Button Assignment

The 8 byte Controller packet (giving the status of the Controller) which can be returned at every vsynch call back consists of the following for the Analog Joystick.

```text
┏━━━━━┯━━━━━━━━━━━━━━━━━━━━━┯━━━━━━━━━━━┯━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃Byte │        Type         │ Bit No./  │                    Button Assignment                    ┃
┃ No  │                     │   Value   │                                                         ┃
┠─────┼─────────────────────┼───────────┼─────────────────────────────────────────────────────────┨
┃1    │Buffer Status        │8 bits     │0x00: success;                                           ┃
┃     │                     │           │0xff: failure                                            ┃
┠─────┼─────────────────────┼───────────┼─────────────────────────────────────────────────────────┨
┃     │                     │bits 0 - 3 │data length/2 (= 0x03)                                   ┃
┃2    │Data length          │bits 4 - 7 │Controller type (0x05)                                   ┃
┃     │Controller type      │(8 bits    │(Analog Joystick is 5)                                   ┃
┃     │                     │total)     │                                                         ┃
┠─────┼─────────────────────┼───────────┼─────────────────────────────────────────────────────────┨
┃     │                     │           │0x00: push ;                                             ┃
┃3, 4 │Status of 14 digital │16 bits    │0x01 : release                                           ┃
┃     │buttons              │           │(as per standard Controller - however L1/L2/R1/R2 are not┃
┃     │                     │           │present on stick)                                        ┃
┠─────┼─────────────────────┼───────────┼─────────────────────────────────────────────────────────┨
┃5 - 8│Analogue value (4    │4 x 8 bits │range 0x00 - 0xff.                                       ┃
┃     │values)              │           │(x, y) , (x, y)                                          ┃
┗━━━━━┷━━━━━━━━━━━━━━━━━━━━━┷━━━━━━━━━━━┷━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛
```

## neGcon

### NegCon Controller Button Assignment

The input data is transmitted as 6-byte serial data and the contents are as follows.
Make sure you include a Calibration menu (0 position calibration, idle movement, sensitivity etc.) in your title.
The 8 byte Controller packet (giving the status of the Controller) which can be returned at every vsynch call back consists of the following for the neGcon.
***Note*** The value varies by twisting NegCon not by pressing a button.

See the controller packet for an Analog Joystick, in the table, below.

```text
┏━━━━━━━━━┯━━━━━━━━━━━━━━━┯━━━━━━━━━━━━━━━━┯━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃ Byte No │     Type      │ Bit No./Value  │    Button Assignment    ┃
┠─────────┼───────────────┼────────────────┼─────────────────────────┨
┃ 1       │ Digital No. 1 │ bit 7          │ Left (on cross button)  ┃
┃         │               │ bit 6          │ Down (on cross button)  ┃
┃         │               │ bit 5          │ Right (on cross button) ┃
┃         │               │ bit 4          │ Up (on cross button)    ┃
┃         │               │ bit 3          │ S (Start)               ┃
┃         │               │ bit 2          │ none                    ┃
┃         │               │ bit 1          │ none                    ┃
┃         │               │ bit 0          │ none                    ┃
┃         │               │ (8 bits total) │                         ┃
┠─────────┼───────────────┼────────────────┼─────────────────────────┨
┃ 2       │ Digital No. 2 │ bit 7          │ none                    ┃
┃         │               │ bit 6          │ none                    ┃
┃         │               │ bit 5          │ A                       ┃
┃         │               │ bit 4          │ B                       ┃
┃         │               │ bit 3          │ R (side button)         ┃
┃         │               │ bit 2          │ none                    ┃
┃         │               │ bit 1          │ none                    ┃
┃         │               │ bit 0          │ none                    ┃
┃         │               │ (8 bits total) │                         ┃
┠─────────┼───────────────┼────────────────┼─────────────────────────┨
┃ 3       │ Analog        │ 0-255          │ Centre (1)              ┃
┠─────────┼───────────────┼────────────────┼─────────────────────────┨
┃ 4       │ Analog        │ 0-255          │ I                       ┃
┠─────────┼───────────────┼────────────────┼─────────────────────────┨
┃ 5       │ Analog        │ 0-255          │ II                      ┃
┠─────────┼───────────────┼────────────────┼─────────────────────────┨
┃ 6       │ Analog        │ 0-255          │ L (side button)         ┃
┗━━━━━━━━━┷━━━━━━━━━━━━━━━┷━━━━━━━━━━━━━━━━┷━━━━━━━━━━━━━━━━━━━━━━━━━┛
```

### Data Format

*1 - Switch Data*

0 is returned when the button is pressed otherwise 1 is returned. Unused bits return 1. The return values comply with the standard Controller specifications, however, they can be changed by BIOS if necessary.

*2 - Switch Data*

As above.

*3 - Twist Data*

The title needs to be designed so that neGcon's twist data value is around 128 when it is released. However, a variation of +/-8 should be allowed for to take account of variations that may occur between different neGcons and the different effect gravity may have on Controller buttons if the Controller is held in a different position. (The specified +/-8 allowance may be changed later if it proves unsuitable but this does not affect your current title.)

Remember that you are recommended to include a calibration option in any title which uses a neGcon Controller as individual neGcons may differ slightly.

*4 - I button Data*

The more this button is pressed, the bigger value is returned. The title should be designed so that a value of 16 or less is returned when it is released.. The software should consider 16 or less as the button not pressed. This error margin allows for variations between different neGcons, as described in *3 - Twist Data*, above - the specified value of 16 or less may be changed later if it proves unsuitable but this does not affect your current title.) The maximum value returned when pressed should be not less than 192(C0H).

*5 - II button Data*

Same as I button, above.

*6 - L button Data*

## Memory Cards

Memory cards are the removable storage medium used by the PlayStation to save and recover game sessions (game save file). A Memory card contains 120K of static non volatile RAM. This is divided conceptually into 15 slots, each of 8k.

Memory cards have the advantage of small size and ruggedness (there are no moving parts and no batteries) but they are relatively slow to access.

#### General Features of Memory Cards
*Capacity:* 120 Kilobytes

*Access Speed:* 10 Kilobytes per second

*Life span:* 100,000 reads guaranteed

As well a being accessed by the application that created the game save file, each file will also need to be compatible with the OSD memory card management program built into the ROM of each PlayStation.

Different versions of the OSD are available for each PlayStation territory (i.e. European, Japanese, American).

The OSD program displays each save game file as a number of icons plus a textual title. Each icon displayed relates to a slot used. This information is provided by a header that must be included at the start of the save game file.

### Memory Card Filenames

Filenames used for memory card save games are based upon the product code for professional games - you of course are free to choose one that suits your purposes.

The file name is in the following format:

**`B`**`E`**`SLES12435`**`plusthis`

- **`B`**: Always B
- `E`: E - Europe A - America I - Japan
- **`SLES12345`**: Product Code (different for each territory)
- `plusthis`: 8 Characters (used as required by the developer)

#### Some Rules on Title Names
Choose either Shift-JIS or ASCII code. Do not mix them.

Use a maximum 32 ASCII letters (32 Byte). If less than 32 characters, please put null letter (0x00) at the end or fulfil the vacant part with blank letter.

### Save Game File Header Format

The Save Game File Header format is as follows.

| Name              | Size (Byes)                                 |
|-------------------|---------------------------------------------|
| Magic Number      | 2 (always 'SC')                             |
| Type(see table 1) | 1 (always 0x11 for Europe)                  |
| No. of Slots      | 1 (1..15, each being 8k)                    |
| Text Name         | 64 (32 Shift JIS (Must be Kanji, not ASCII) |
| Pad               | 28 (padding)                                |
| CLUT              | 32                                          |
| Icon Image (1)    | 128 (16 x 16 x 4 bits)                      |
| Icon Image (2)    | 128 (Type:0x12, 0x13 only)                  |
| Icon Image (3)    | 128 (Type:0x13 only)                        |

In the header file there is provision for three icon images, as below, which allows animation.

```text
┏━━━━━━┯━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃      │        Number of Icon Images        ┃
┃ Type │                                     ┃
┃      │ (automatically replaced animation)  ┃
┠──────┼─────────────────────────────────────┨
┃ 0x11 │                  1                  ┃
┠──────┼─────────────────────────────────────┨
┃ 0x12 │                  2                  ┃
┠──────┼─────────────────────────────────────┨
┃ 0x13 │                  3                  ┃
┗━━━━━━┷━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┛
```

### Use of Kanji Fonts

The Memory card save game file header *Text Name* must be stored in Kanji (Japanese Shift-JIS) type character coding and not the normal ASCII. (If ASCII were used the game header file, the title would not be displayed correctly if it is longer than 16 characters - 32 is the total possible. )

### Animated Icons

As can be seen from the table above it is possible to have animated icons in the save game. There are three frames available and they are cycled automatically by the OSD.

(If two slots are used per save, a two frame animation is allowed; with three slots, a three frame animation).

The Icon is a 16 \* 16 image with 4 bit colour depth.

*Note:* do not use animated icons for European titles (i.e. only use type 0x11) if your save occupies a single slot. 3.2.3

### Hints on Using Memory Cards

Memory cards are slow to recognise and access. Memory card accesses, therefore, should be kept to a minimum and when they are made be as efficient as possible.

As a Memory card is divided into 8k slots, the minimum possible number should be used. Writes to the card system are always done as multiples of 8K, so if the intended game save is 8K + 1 bytes, it will use two slots, taking twice as long to save as one slot and take twice as much space as a save that is only 1 byte smaller.

In this example, there is only 1 byte over so some compression/economising of the data could probably be achieved. For over spill which cannot be squeezed into 8k, the alternative is to save more data in the in the file and use up the wasted space. It costs nothing in terms of resources or performance.

### Notes for Those Writing RPGs and Strategy Games

Many games in these genres benefit from allowing the user to save several sessions concurrently. This allows the user to backup before undertaking hazardous stages, or to approach a complex problem from several angles simultaneously.

This can be achieved in two ways, firstly by having several saves within the same file. For example if the data required by the save game is only 2K, you can save 4 games within the same slot.

Alternatively, the format of the save game filename allows for multiple saves from the same game, because there are nine characters in the memory card file name left available for user allocation. This could be used to store a user entered string that is a description of the game save at that point in the game.

Better still, add an internal character field that allows for a textual description of the current stage of the game and display this when displaying a list of all the save games on the memory card. BESLES-12345level2 In some games the memory card facilities play a very important role. A classic example of this is *XCOM: Enemy Unknown*, which would be very hard, if not impossible to finish without frequent memory card access. These unfortunately are slow and so disrupt the flow of play. One solution would be to save games into RAM as a quick save, with the option to commit the game to the card at any time. This is only feasible if the game never crashes out and there is room in memory for the save game(s).

It is not possible to lengthen a file saved on a Memory card once it is created, therefore when the file is created it should as large as it will ever need to be.

### Advice on Testing Memory Cards

1.  Is the Memory card operation display working? (Display of icon, or file name, etc.)
2.  Can Memory card be formatted? (On-screen instructions and statements are correct?)
3.  The "formatting menu" options should include a dialogue such as "The Memory card ID is not formatted yet. Do you want to format that? Yes? or No?". Then, when "Yes" is chosen, saving should start and when "No" is chosen, the menu should exit from save mode.
4.  Check what happens when there aren’t enough empty blocks in the Memory card for a game save.
5.  Check what happens when the Memory.

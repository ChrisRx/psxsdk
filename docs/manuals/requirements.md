---
title: Technical Requirements for Net Yaroze Software
weight: 3
---

Development rules for Net Yaroze software featuring on PlayStation® titles

Version 1.0

Effective from: 6th August 1998

## About These Requirements

The recommendations and requirements described in this document are a subset of those listed in the document ‘Technical Requirements Checklist for PlayStation® Software’, to which all published PlayStation® titles must conform.

Compliance with the *requirements* listed below is mandatory for Net Yaroze software which is to be considered for inclusion in published PlayStation® titles such as magazine coverdiscs. Compliance with the *recommendations* however is not obligatory, but the guidelines described will help you create software whose operation is intuitive and consistent with other PlayStation® titles.

The rules and recommendations contained in this document are not intended to stifle your creativity as a programmer, but to ensure that all published PlayStation® software meets users’ expectations of usability and quality.

PlayStation and PlayStation Logos are trademarks of Sony Computer Entertainment Inc. All other trademarks are the property of their respective owners and/or their licensors.

Guncon™, G-Con45™ are trademarks of NAMCO LTD.

neGcon® is a registered trademark of NAMCO LTD.

## Requirements

### Sony Corporate Logo, PlayStation Logo and other corporate trademarks

- The four-letter "SONY" logotype must not be used within this program except where this logo appears on a digitised image of the PlayStation console or on a peripheral licensed by SCEI/SCEA/SCEE.
- The PlayStation logo, or logotype, and the SCE logo must not be used within this program except where this logo appears on a digitised image of the PlayStation console or on a peripheral licensed by SCEI/SCEA/SCEE.
- The logos described in the exceptions to the above (seen in digitised images of the PlayStation console and peripherals) must not be defaced or blurred, and the design/inscription on the face must be correct.

  If such problems exist with the PlayStation logo within the digitised image, please remove the logo from the image.
- In the program, the design of the PlayStation console and any PlayStation peripherals should not be used or shown unless necessary, for example, to explain software operation. Any deformation or characterisation of the console or peripheral designs is strictly prohibited.
- Other corporate logos or properties are not to be used in the program without permission of the property holders.
- Other corporate logos, properties, or trademarks cannot be displayed on any image of the PlayStation console or any licensed peripheral by SCEI/SCEA/SCEE.

### PlayStation Component Naming Conventions

- The following names must be used in the program when referring to the console or a licensed peripheral of SCEI/SCEA/SCEE.

  This table is reproduced from ‘Technical Requirements Checklist for PlayStation Software’ version 1.3, and hence contains some definitions which are not relevant for Net Yaroze programs.

| | English | French | German |
| --- | --- | --- | --- |
| PlayStation General | PlayStation | PlayStation | PlayStation |
| | console | console | Konsole |
| PlayStation component names | connector port section | section des connecteurs | Anschlüß |
| | controller port | port de manette | Controller-Anschlüß |
| | controller port 1 | port de manette N°1 | Controller-Anschlüß 1 |
| | controller port 2 | port de manette N°2 | Controller-Anschlüß 2 |
| | Memory Card slot | fente pour carte mémoire | Memory Card Steckplatz |
| | Memory Card slot 1 | fente pour carte mémoire N°1 | Memory Card Steckplatz 1 |
| | Memory Card slot 2 | fente pour carte mémoire N°2 | Memory Card Steckplatz 2 |
| | right audio output | sortie audio droite | Rechter Audioausgang |
| | left audio output | sortie audio gauche | Linker Audioausgnag |
| | video output | sortie vidéo | Videoausgang |
| | PARALLEL I/O port | port PARALLEL I/O | PARALLEL I/O Anschluß |
| | SERIAL I/O port | port SERIAL I/O | SERIAL I/O Anschluß |
| | AV MULTI OUT connector | connecteur AV MULTIOUT | AV MULTI OUT Anschluß |
| | S video output connector | connecteur de sortie S-VIDEO | S-VIDEO-Anschluß |
| | RGB outoput connector | connecteur de sortie RVB | RGB-Anschluß |
| | video output jack | prise de sortie vidéo | Videoeingangsstecker |
| | POWER button | bouton POWER | POWER-Taste |
| | POWER indicator | indicateur de mise sous tension | Kontrolleuchte |
| | RESET button | bouton RESET | RESET-Taste |
| | OPEN button | bouton OPEN | OPEN-Taste |
| | disc cover | couvercle du compartiment à disque | CD-Fach |
| Controller component names | directional button | touche directionnelle | Richtungstaste |
| | SELECT button | touche SELECT | SELECT-Taste |
| | START button | touche START | START-Taste |
| | △ button | touche △ | △-Taste |
| | □ button | touche □ | □-Taste |
| | O button | touche O | O-Taste |
| | X button | touche X | X-Taste |
| | L1 button | touche L1 | L1-Taste |
| | L2 button | touche L2 | L2-Taste |
| | L3 button | touche L3 | L3-Taste |
| | R1 button | touche R1 | R1-Taste |
| | R2 button | touche R2 | R2-Taste |
| | R3 button | touche R3 | R3-Taste |
| | right button | touche droite | Rechter-Taste |
| | left button | touche gauche | Linker-Taste |
| | right stick | joystick droite | Rechter Stick |
| | left stick | joystick gauche | Linker Stick |
| | analog mode switch | Interrupteur mode analogique | Analog-Modus Schalter |
| | LED | voyant | LED |
| | controller port\*-A | port de manette\*-A | Controller-Anschlüß\*-A |
| | controller port\*-B | port de manette\*-B | Controller-Anschlüß\*-B |
| | controller port\*-C | port de manette\*-C | Controller-Anschlüß\*-C |
| | controller port\*-D | port de manette\*-D | Controller-Anschlüß\*-D |
| | Memory Card slot\*-A | fente pour carte mémoire\*-A | Memory Card Steckplatz\*-A |
| | Memory Card slot\*-B | fente pour carte mémoire\*-B | Memory Card Steckplatz\*-B |
| | Memory Card slot\*-C | fente pour carte mémoire\*-C | Memory Card Steckplatz\*-C |
| | Memory Card slot\*-D | fente pour carte mémoire\*-D | Memory Card Steckplatz\*-D |
| Peripheral names | Controller | Manette | Controller |
| | Memory Card | Carte mémoire | Memory Card |
| | Mouse | Souris | Maus |
| | Link Cable | Câble de liaison | Link Kabel |
| | Multi Tap | Connecteur multi-manettes | Multi Tap |
| | Analog Joystick | Joystick Analogique | Analog Joystick |
| | Analog Controller (DUAL SHOCK) | Manette Analogique (DUAL SHOCK) | Analog Controller (DUAL SHOCK) |
| | Analog Controller | Manette Analogique | Analog Controller |
| | Euro-AV Cable | Câble Euro-AV | Euro-AV-Kabel |
| | S Video Cable | Câble S VIDEO | S-VIDEO-Kabel |
| | AV Cable | Câble AV | AV-Kabel |
| | AV Adaptor | Adaptateur AV | AV-Adapter |
| | RFU Adaptor | Adaptateur RFU | RF Adapter |
| | AC Power Cord | Cordon d’alimentation | Netzkabel |
| Names of discs in multi-disc applications | DISC 1 | DISQUE 1 | CD 1 |
| | DISC 2 | DISQUE 2 | CD 2 |
| | DISC 3 | DISQUE 3 | CD 3 |
| | DISC 4 | DISQUE 4 | CD 4 |
| Instruction manual | instruction manual | mode d’emploi | Bedienungsanleitung |
| Users Manual | software manual | manuel du logiciel | Software-Handbuch |

\* When referring to a Multi Tap connected to controller port 1, the names of each controller port on the Multi Tap are 1-A, 1-B, 1-C and 1-D. When referring to a Multi Tap connected to controller port 2, the names of each controller port on the Multi Tap are 2-A, 2-B, 2-C and 2-D.

For the part name of each peripheral, please refer to the explanation in the instruction manual for that peripheral.

### Text and Graphics Display

- The program must not feature any means of switching between PAL and NTSC video modes.

### Peripheral Handling

- The program must function correctly when any licensed peripherals are added or removed at any time during the execution of the program.
- When supporting multiple licensed peripherals (such as both Controller and Mouse, etc.) the program must not be affected by peripherals being added or removed at any time during the program execution.
- The title must function correctly when three or more of the directional buttons on any licensed Controller are pressed simultaneously.

  (After using the control device for an excessively long time, the props to support the buttons can become worn away. In such cases the Controller may behave as if three or more directional buttons have been pressed simultaneously).
- Even if unused buttons are pressed, the program must continue to function correctly.
- The program must recognise any licensed Controller even when it is inserted at any time during the program execution.
- The program must function correctly with any licensed Controllers it claims to fully support.
- The program must function correctly (ie. does not hang or otherwise malfunction) even after mode change (ie. digital to analog or analog to digital) with multiple-mode Controllers, such as the Analog Joystick.

#### Analog Joystick

If the program supports the Analog Joystick, the following points must be observed:

- Calibration mode for centre position correction of left/right sticks must be present.

  Obtain the value of each stick with the sticks released from the user’s hands (in neutral position). This value should be used as the centre position value throughout the program.

  Recommended: set up of idle movement, idle sensitivity, etc.

  When using the Analog Joystick in analog input mode imitating digital, please provide centre position calibration or set the value of 128 ± 32 as off and other values as on.
- Calibration mode for movable range of left/right sticks must be present.

  Movable range detection and correction of left/right sticks.

  1. Obtain the value of each stick where the sticks are moved as far as possible to each direction.
  2. With these values and the values of the centre position obtained as described previously, obtain the absolute values of difference between the centre position values and the minimum, maximum values in the movable range of each stick.
  3. Use the smaller values of the absolute values as the movable range, and the larger value should be corrected.

#### neGcon®

If the program supports the neGcon®, the following points must be observed:

- Calibration mode for centre position correction of twist must be present.

  Centre position detection and correction of twist: obtain the value of twist with the neGcon® released from the user’s hand (in the neutral position). This value should be used as the centre position value throughout the program.

  Recommended: detection of maximum value of twist, set up of idle position range, set up of sensitivity, I, II, L button detection, etc.
- If the title requires a maximum twist value of larger than +/- 45 degrees, detection of the maximum value of twist must be present.

#### Synchronised Lightgun (eg. Guncon™, G-Con45™)

If the program supports a synchronised lightgun, centre position calibration mode must be present.

#### Analog Controller (DUAL SHOCK)

If the program supports the Analog Controller (DUAL SHOCK), the following points must be observed:

- There must be no calibration screen (function) for centre position and detection of maximum travel value.

  Since the Analog Controller (DUAL SHOCK) has a built-in auto-calibration function for centre position calibration and maximum travel detection, an application level calibration screen (function) for these two must not be implemented. The centre position calibration mentioned above performs calibration using the value obtained when the Analog Controller (DUAL SHOCK) is inserted. Thus the setup of the centre idle range can be done by an application.

  Whenever referring to the Analog Controller (DUAL SHOCK), the appropriate naming conventions should be used.

### Memory Card

- The program must not access Memory Card procedures.

  This is due to a requirement that data written to Memory Cards must be accompanied by the product code of the program performing the write. Since Net Yaroze programs do not have registered product codes, they should not access Memory Cards without prior consultation with SCEE.

### Handling CD-ROM Read Errors

- Error flags are to be checked for every CD access call that is made.

### Aging Tests

- The program should continue to function correctly after an aging test of eight hours has been performed with the title in Demo Mode (title screen/opening demonstration screen).
- The program should continue to function correctly after an aging test of eight hours has been performed with the title in Pause Mode (only for titles which implement Pause Mode).
- The program should continue to function correctly after aging tests of eight hours have been performed in all areas of the program where a user could reasonably be expected to leave the program unattended for extended periods (e.g. menu screens, save game screens, etc.)

## Recommendations

### Text and Graphics Display

- The display should be full-screen PAL (512 pixels tall in interlaced mode, or at least 256 pixels tall otherwise).
- Text and essential display items should not be placed close to display edges.

  (Due to variations in television displays, critical application text and graphics might be cut-off on some displays).
- A screen-adjust mode should be present.

### Controls

- Include a ‘quick start’ functionality that will allow a user to bypass the introductory credits or animation sequences to start the program with a default set of options.

  For example, if the user depresses the L1 + R1 buttons during the start of the program, the program will bypass non-essential title screens to allow the user to quickly start the program.
- For screens where the selection of a single item takes the user to another screen/menu, pressing the X button should make the choice and advance. All of the same menu/list rules that applied to the original screen should also apply to the new screen.
- Selection (activation) of any option or menu item should be accomplished by pressing the X button. Once the user’s selections are completed, the user should either (1) press the START button to proceed, or (2) highlight an ‘exit’ option and then press the X or START button to proceed.
- The △ button should be used to take the user back to the previous menu. When the user presses the △ button, the program must return the user to the previous screen without accepting any changes that may have been made on the screen from which they are quitting.
- The X button should be the primary button throughout the program.

### Loading

- During loading, the user should not be presented with a blank screen lasting more than five seconds.
- Whenever loading occurs which may interrupt play, some sort of display should be used.

  Any load that lasts longer than five seconds while on screen should notify the user that a load is occurring.

### Connecting/disconnecting peripherals

- The program should enter pause mode if a controller or peripheral becomes connected or disconnected at any time during program execution.

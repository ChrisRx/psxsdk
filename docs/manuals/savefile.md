---
title: Save Files
weight: 5
---

*9/22/97*

## Dear Yaroze Members

When creating save game files that will co-exist with commercial save game files please use the following format. The reason for this is that most commercial games rely on these conventions in order to display other games save file. In many cases when a save file exists that doesn't follow these conventions, the commercial game will behave unpredictably. For information on creating a Shift-JIS Save Title Name, please refer to the website.

## File Names

#### Use the following structure for file names:

| **Bytes** | **Contents** | **Notes** |
|----|----|----|
| 0 | Magic Number | Always 'B' |
| 1 | Region | Japan: 'I' North America: 'A',Europe: 'E'(\*1) |
| 2-10 | -NETYAROZE |   |
| 12-20 | User/Public | Use only non-0x00, 0x2a(\*), 0x3f(?) ASCII. End with 0x00 |

\*1: None are checked by the system\
\*2: The first disk for multi-disk titles

Example: If the region is North America, the file name's first 12 characters are BA-NETYAROZE.\
Always add zeros to make the numerical portion 5 digits.

## File Headers

**Put the following headers at the start of each file.\
Table 5-7: Memory Card File Header**

| **Item**               | **Size (bytes)**           |
|------------------------|----------------------------|
| Header                 | 128                        |
| Magic number           | 2 (always "SC")            |
| Type (see table below) | 1                          |
| No. of slots           | 1                          |
| Text name              | 64 (Shift JIS, \*1)        |
| Pad                    | 28                         |
| CLUT                   | 32                         |
| Icon image (1)         | 128 (16 x 16 x 4 bits)     |
| Icon image (2)         | 128 (Type:0x12, 0x13 only) |
| Icon image (3)         | 128 (Type:0x13 only)       |
| Data Varies            | (128Byte x N)              |

\*1: Non-kanji and primary standard kanji only, full-size 32 characters.

**(automatically replaced animation)\
Table 5-8: Type Field**

| **Type** | **Number of icon images** |
|----------|---------------------------|
| 0x11     | 1                         |
| 0x12     | 2                         |
| 0x13     | 3                         |

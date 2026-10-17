---
title: sioload
weight: 13
---

The linux version of siocons, found on the [psxdev.net forums](http://www.psxdev.net/forum/viewtopic.php?f=67&t=1078) (and floating around other places), did not initially work when loading a Net Yaroze executable. Even after figuring out a way to make it work, it didn't work for all baud rates, didn't work consistently, and had several huge bugs in how it loaded binaries (it loaded the sections incorrectly in a way that still worked), so `sioload` was written as a replacement.

`sioload` isn't a full-feature siocons replacement, but a serial executable loader in Go. Unless specified it finds the first available serial port and defaults to a baud rate of 115200:

```bash
$ sioload --exec main-ecoff
```

It has been reliable at every baud rate tested, using a Net Yaroze DTL-H3050 serial cable through a [TRENDnet USB to Serial converter](https://www.amazon.com/dp/B0007T27H8/ref=cm_sw_em_r_mt_dp_U_FHmgEbZAAPNX5).

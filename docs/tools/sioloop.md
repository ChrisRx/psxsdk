---
title: sioloop
weight: 23
---

`sioloop` checks a USB serial adapter with no console attached, before it is used on the serial cable to a Net Yaroze. It runs from the repository, not as an installed tool.

Join pins 2 and 3 of the adapter's DB9 connector, and pins 7 and 8, keeping the two pairs apart. Then plug it in and run:

```bash
$ go run ./tools/sioloop -d /dev/ttyUSB0
```

It reports whether the adapter returns every byte sent, whether its CTS follows its RTS, how long each of those takes, and from that an estimate of the speed of an upload. It exits with 1 if the adapter fails a check.

`-d` names the device, the first one found if not given, and `-b` the baud rate, 115200 by default.

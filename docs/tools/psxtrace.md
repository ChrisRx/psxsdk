---
title: psxtrace
weight: 8
---

`psxtrace` reads the traces `psxemu` and psxe write. `dump` prints one as text, `diff` reports the first record two traces disagree on, and `vram` compares two VRAM dumps or converts one to an image:

```bash
PSXE_TRACE=psxe.trace PSXE_TRACE_COUNT=1000000 psxe -a -b bios.bin
psxemu --bios bios.bin --trace emu.trace --count 1000000
psxtrace diff psxe.trace emu.trace
```

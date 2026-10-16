---
title: mdecenc
weight: 5
---

`mdecenc` encodes PNG, JPEG or GIF pictures, in the order given, into a movie of run-length codes the MDEC decodes as they are, laid out as [`format/mdec`](format/mdec) describes. Every picture must be the same size, and each is extended to a multiple of 16 pixels each way. `-q` sets the quantization scale, from 1 to 63, where higher is smaller and blurrier.

```bash
$ mdecenc -o MOVIE.MDC frame*.png
```

The [mdec example](../examples/mdec.md) plays one, rendered by its own ray tracer in `examples/mdec/gen`.

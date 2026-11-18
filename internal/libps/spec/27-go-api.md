# The Go API on the clean library

`exp/ps` is the ordinary Go API: Screen, Frame, Sprite, Image, Text, Pad and so on. It sits on the library through one file, `exp/ps/lib_yaroze.go`,
which still imports the decompiled library, and that is leaving. `exp/ps` and `exp/examples` are not off limits, and this spec moves them onto `exp/cleanlib`.

Static linking is now yaroze's default (`--libps static`, target `playstation-yaroze-clean`), so a program that imports `exp/ps` gets the clean library linked in.

## What to do

In `exp/ps/lib_yaroze.go`, import `go.chrisrx.dev/psx/exp/cleanlib` in place of what it imports now, and use it. Of the 116 names it uses, these 23 aren't in `exp/cleanlib`
under those names:

    AverageZ3 AverageZ4 DIRENTRY DispEnv FileClose FileDelete FileFirst FileFormat FileNext FileOpen FileRead FileWrite
    FntWrite FntWriteBytes GsOT_TAG GteMIMefunc NormalClip Origin PACKET Rand RotTransPers RotTransPers3 Srand WSMatrix

They are Go-facing names: some for entries `exp/cleanlib` already has under their C name (`rand`, `srand`, `open`, `read`, `firstfile`), some GTE sequences the C library leaves to macros in
its headers, some types. What each is for, you can read from how `exp/ps` calls it. The behavior is the existing C entry's where there is one, and otherwise the manuals'.

Add what's missing to `exp/cleanlib`, in a Go file of their own, as thin as they can be, and these are the API's own: they need no cycle matching against Sony's, since a program through `exp/ps` has no
Sony's to match. Where a name is only an alias for the API's convenience, rather change `exp/ps` to fit. Keep `exp/ps`'s public API as it is. It should not change for this.

`exp/ps/scene`, `exp/ps/stage` and `exp/examples` may need nothing, but build them all.

## Acceptance

- `go run ./cmd/yaroze disc -o .build/NAME-ps/NAME-ps.bin exp/examples/NAME` builds for every directory under `exp/examples`, and `go vet ./exp/ps/...` passes on the host.
- `go test ./exp/ps/...`.
- The programs run the same as the originals. `exp/examples` are ports of `examples`, and `tests` compares them, in tests named `...PS`. Those compare with golden pictures taken with Sony's font,
  so a difference only in text is expected for now and the spec side deals with it. Build the original with the same static default onto `.build/NAME`, and compare the two pictures with each other.
- `exp/cleanlib/regress.sh` without `PSX_BIOS`, FAIL stays 0, since you added to the library: 33 PASS, 8 TIME before.
- `exp/cleanlib/image/make.sh` still builds the image, and adding Go API to the library doesn't grow the image: anything only `exp/ps` calls must be dropped from it, and say what size it comes out.

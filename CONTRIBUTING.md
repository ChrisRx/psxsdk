# Contributing

Thanks for wanting to help. Bug reports, fixes, new examples, ports of Net Yaroze games, documentation and tests are all welcome.

## Before you start

For anything bigger than a small fix, open an issue first so we can agree on the approach before you spend time on it.

## Setting up

[Building from source](README.md#building-from-source) covers it. In short, `go build` and `go test ./...` work with Go alone, and `nix develop` gives you everything else: TinyGo with the PlayStation targets, the cross binutils, go-task and the reference emulators.

## Making a change

- Keep each pull request to one change, and keep it small enough to review.
- Run `task test` (`go vet` and `go test`) before you push. If you touched the emulator, the `yaroze` package or an example, run `task test:emu` too.
- Format Go code with `gofmt`.
- Add or update tests for what you changed. Tests use [`go.chrisrx.dev/x/assert`](https://pkg.go.dev/go.chrisrx.dev/x/assert).
- Update the docs under `docs/` if the change is something a user would notice.

## Commit messages

Commits follow a short conventional style: a type, then a sentence in lower case saying what is now true.

```
feat: breakpoints in the emulator
fix: the playground's sidebar fits a phone
docs: the tutorial covers sound
```

The types in use are `feat`, `fix`, `perf`, `refactor`, `docs`, `test` and `chore`.

## AI tools

Using them is fine, but no `Co-Authored-By` lines naming an AI. See [AI usage](README.md#ai-usage) in the README.

## License

By contributing you agree that your work is released under the [MIT license](LICENSE).

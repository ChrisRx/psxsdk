package yaroze

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"go.chrisrx.dev/psx/internal/keepcheck"
)

func vetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "vet [package]...",
		Short: "find variables the library would go on using after they are gone",
		Long: `vet reports the variables a program passes to the Net Yaroze library that
the library goes on using after they are gone. LoadImage reads its pixels
until DrawSync, so a variable of a function other than main passed to it
has to be followed by DrawSync before that function returns. GsSetWorkBase
keeps its packet area for good, so a variable made in a loop, or in the
body of the frame loop, would be made anew every time round and never
freed. Either way the answer is most often a package-level variable. The
build refuses the same; vet says it sooner, by the variable's name.

Each package is a directory, the current one if none is given, and one
ending in /... is that directory and those under it.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				args = []string{"."}
			}
			var dirs []string
			for _, a := range args {
				d, err := vetDirs(a)
				if err != nil {
					return err
				}
				dirs = append(dirs, d...)
			}
			n := 0
			for _, dir := range dirs {
				problems, err := keepcheck.CheckDir(dir)
				if err != nil {
					return err
				}
				for _, p := range problems {
					fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", p.Pos, p.Msg)
					n++
				}
			}
			if n > 0 {
				return fmt.Errorf("%d variable(s) the library would lose", n)
			}
			return nil
		},
	}
}

// vetPackage checks the package in pkg as vet does, before a build, and
// refuses it with what vet says: the compiler refuses the same, but cannot
// name the variable. A package that is not a directory, or does not parse,
// is left to the compiler to say what is wrong with it.
func vetPackage(pkg string, w io.Writer) error {
	if info, err := os.Stat(pkg); err != nil || !info.IsDir() {
		return nil
	}
	problems, err := keepcheck.CheckDir(pkg)
	if err != nil {
		return nil
	}
	for _, p := range problems {
		fmt.Fprintf(w, "%s: %s\n", p.Pos, p.Msg)
	}
	if len(problems) > 0 {
		return fmt.Errorf("%d variable(s) the library would lose", len(problems))
	}
	return nil
}

// vetDirs returns the directories a package argument names: the one, or
// with /... every one under it that holds Go files.
func vetDirs(arg string) ([]string, error) {
	root, all := strings.CutSuffix(arg, "/...")
	if !all {
		return []string{arg}, nil
	}
	if root == "" {
		root = "."
	}
	var dirs []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		name := d.Name()
		if path != root && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_") || name == "testdata" || name == "vendor") {
			return filepath.SkipDir
		}
		matches, err := filepath.Glob(filepath.Join(path, "*.go"))
		if err != nil {
			return err
		}
		if len(matches) > 0 {
			dirs = append(dirs, path)
		}
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%s: no such directory", root)
	}
	return dirs, err
}

package toolchain

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go.chrisrx.dev/x/assert"
)

// readTarget is the target file at path.
func readTarget(t *testing.T, path string) targetFile {
	t.Helper()
	data, err := os.ReadFile(path)
	assert.NoError(t, err)
	var target targetFile
	assert.NoError(t, json.Unmarshal(data, &target))
	return target
}

// TestWriteTargets: every mode gets a target file that inherits the files
// that go first and then the base target, by the path the compiler sees
// them at, and each file a target names is one of the Net Yaroze's.
func TestWriteTargets(t *testing.T) {
	dir := t.TempDir()
	const guest = "/sysroot/tinygo/yaroze"
	assert.NoError(t, WriteTargets(dir, guest))

	first := readTarget(t, filepath.Join(dir, "first.json"))
	assert.Equal(t, "yaroze/start.S", first.ExtraFiles[0], "the start-up is first on the link line")
	assert.Equal(t, 0, len(first.Inherits))

	for _, m := range Modes {
		assert.Equal(t, guest+"/"+string(m)+".json", m.TargetPath(guest))
		target := readTarget(t, filepath.Join(dir, string(m)+".json"))
		assert.Equal(t, []string{guest + "/first.json", BaseTarget}, target.Inherits, "%s", m)
		assert.Equal(t, "yaroze/yaroze.ld", target.LinkerScript, "%s", m)
		assert.Equal(t, "yaroze", target.BuildTags[0], "%s", m)
		for _, f := range append(first.ExtraFiles, target.ExtraFiles...) {
			_, err := os.Stat(filepath.Join("yaroze", filepath.Base(f)))
			assert.NoError(t, err, "%s names %s", m, f)
		}
	}

	static := readTarget(t, filepath.Join(dir, "static.json"))
	assert.Equal(t, []string{"yaroze", "cleanlib"}, static.BuildTags)
	assert.Equal(t, []string{"-DLIBPS_GO", "-DLIBPS_CLEAN"}, static.CFlags)
	resident := readTarget(t, filepath.Join(dir, "resident.json"))
	assert.Equal(t, 0, len(resident.CFlags)+len(resident.LDFlags), "a resident build links where the link script says")
	fidelity := readTarget(t, filepath.Join(dir, "fidelity.json"))
	assert.Equal(t, append(static.CFlags, "-DLIBPS_FIDELITY"), fidelity.CFlags)
	assert.Equal(t, static.ExtraFiles, fidelity.ExtraFiles)

	_, err := Mode("go").target(guest)
	assert.Error(t, "no way to build", err)
}

// TestOverTarget: a program's own target file is built over a mode's, which
// takes the base target's place in what it inherits, with everything else
// it sets kept.
func TestOverTarget(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "target.json")
	dst := filepath.Join(dir, "over.json")
	const base = "/sysroot/tinygo/yaroze/resident.json"

	assert.NoError(t, os.WriteFile(src, []byte(`{"inherits": ["playstation"], "ldflags": ["--defsym=__load=0x80068000"], "default-stack-size": 4096}`), 0o666))
	assert.NoError(t, overTarget(src, dst, base))
	over := readTarget(t, dst)
	assert.Equal(t, []string{base}, over.Inherits)
	assert.Equal(t, []string{"--defsym=__load=0x80068000"}, over.LDFlags)
	data, err := os.ReadFile(dst)
	assert.NoError(t, err)
	var fields map[string]any
	assert.NoError(t, json.Unmarshal(data, &fields))
	assert.Equal(t, 4096.0, fields["default-stack-size"], "a field yaroze does not set is kept")

	assert.NoError(t, os.WriteFile(src, []byte(`{"inherits": ["playstation-yaroze"]}`), 0o666))
	assert.Error(t, "does not inherit playstation", overTarget(src, dst, base))
	assert.NoError(t, os.WriteFile(src, []byte(`{"ldflags": []}`), 0o666))
	assert.Error(t, "does not inherit playstation", overTarget(src, dst, base), "a target file that inherits nothing")
}

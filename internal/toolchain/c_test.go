package toolchain

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"go.chrisrx.dev/x/assert"
)

func TestHasHeader(t *testing.T) {
	assert.Equal(t, true, HasHeader("libps.h"))
	assert.Equal(t, true, HasHeader("sys/types.h"))
	assert.Equal(t, false, HasHeader("nowhere.h"))
	assert.Equal(t, false, HasHeader("sys"))
}

func TestLineFilter(t *testing.T) {
	var out bytes.Buffer
	l := &lineFilter{w: &out, drop: "noise"}
	l.Write([]byte("a\nno"))
	l.Write([]byte("ise\nb\nunfinished"))
	l.Flush()
	assert.Equal(t, "a\nb\nunfinished", out.String())
}

func serveFull(t *testing.T, module []byte) (*httptest.Server, fullManifest, *int) {
	t.Helper()
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	zw.Write(module)
	zw.Close()
	hits := new(int)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*hits++
		w.Write(gz.Bytes())
	}))
	t.Cleanup(srv.Close)
	sum := sha256.Sum256(gz.Bytes())
	return srv, fullManifest{URL: srv.URL, SHA256: hex.EncodeToString(sum[:]), Size: int64(gz.Len())}, hits
}

func setManifest(t *testing.T, m fullManifest) {
	t.Helper()
	data, err := json.Marshal(m)
	assert.NoError(t, err)
	old := embeddedFull
	EmbedFull(data)
	t.Cleanup(func() { embeddedFull = old })
}

func TestFullModuleDownloadsOnce(t *testing.T) {
	t.Setenv("YAROZE_CACHE", t.TempDir())
	t.Setenv("YAROZE_CC", "")
	_, m, hits := serveFull(t, []byte("the module"))
	setManifest(t, m)
	var stderr bytes.Buffer
	b := &CBuild{Stderr: &stderr}
	got, err := fullModule(context.Background(), b)
	assert.NoError(t, err)
	data, err := os.ReadFile(got)
	assert.NoError(t, err)
	assert.Equal(t, "the module", string(data))
	again, err := fullModule(context.Background(), b)
	assert.NoError(t, err)
	assert.Equal(t, got, again)
	assert.Equal(t, 1, *hits)
}

func TestFullModuleRefusesWrongHash(t *testing.T) {
	t.Setenv("YAROZE_CACHE", t.TempDir())
	t.Setenv("YAROZE_CC", "")
	_, m, _ := serveFull(t, []byte("the module"))
	m.SHA256 = "00000000000000000000000000000000"
	setManifest(t, m)
	var stderr bytes.Buffer
	_, err := fullModule(context.Background(), &CBuild{Stderr: &stderr})
	assert.Error(t, "has hash", err)
}

func TestFullModuleNamedByEnvironment(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tinygo-full.wasm")
	t.Setenv("YAROZE_CC", path)
	got, err := fullModule(context.Background(), &CBuild{})
	assert.NoError(t, err)
	assert.Equal(t, path, got)
}

func TestFullModuleWithNoWhereToGetIt(t *testing.T) {
	t.Setenv("YAROZE_CC", "")
	setManifest(t, fullManifest{})
	_, err := fullModule(context.Background(), &CBuild{})
	assert.Error(t, "set YAROZE_CC", err)
}

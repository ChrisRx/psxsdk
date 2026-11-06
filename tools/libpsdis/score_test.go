package main

import (
	"cmp"
	"os"
	"slices"
	"strings"
	"testing"

	"go.chrisrx.dev/x/assert"
)

func TestCommon(t *testing.T) {
	assert.Equal(t, common([]int{1, 2, 3}, []int{1, 2, 3}), 100.0)
	assert.Equal(t, common([]int{1, 2, 3, 4}, []int{1, 9, 3, 8}), 50.0)
	assert.Equal(t, common([]int{}, []int{}), 100.0)
	assert.Equal(t, common([]int{1}, []int{2}), 0.0)
}

func TestOverlap(t *testing.T) {
	assert.Equal(t, overlap([]string{"lw", "sw"}, []string{"sw", "lw"}), 100.0)
	assert.Equal(t, overlap([]string{"lw", "lw"}, []string{"lw", "sw"}), 100.0/3)
}

func TestExactKeyMasksWhereThingsAre(t *testing.T) {
	const jalA, jalB = 0x0c004000, 0x0c008000
	a, _ := exactKey(scored{Insn: Decode(0x80010000, jalA), Word: jalA})
	b, _ := exactKey(scored{Insn: Decode(0x80010000, jalB), Word: jalB})
	assert.Equal(t, a, b)
}

func TestScoreControls(t *testing.T) {
	if os.Getenv("PSX_SONY_LIBPS") == "" {
		t.Skip("PSX_SONY_LIBPS names the private copy of Sony's image")
	}
	img, err := LoadImage("", "../../internal/toolchain/_tinygo/targets/playstation-yaroze-symusr.S")
	assert.NoError(t, err)
	fns := img.Functions()
	sony := &sonyCode{img: img, fns: fns}
	var named []*Function
	for _, f := range fns {
		if !strings.HasPrefix(f.Name, "func_") {
			named = append(named, f)
		}
	}
	// A library against itself is the same.
	for _, f := range named {
		a := flatten(sony, f.Start, true)
		assert.Equal(t, common(keys(a, shapeKey), keys(a, shapeKey)), 100.0)
	}
	// Different functions of about the same size give the floor a score means anything above.
	slices.SortFunc(named, func(a, b *Function) int { return cmp.Compare(a.Size, b.Size) })
	floor := map[string][2]float64{}
	for i := 0; i+1 < len(named); i++ {
		a, b := flatten(sony, named[i].Start, false), flatten(sony, named[i+1].Start, false)
		size := "under 100"
		if len(a) >= 100 {
			size = "100 or more"
		}
		c := floor[size]
		c[0] += common(keys(a, shapeKey), keys(b, shapeKey))
		c[1]++
		floor[size] = c
		c = floor["exact "+size]
		c[0] += common(keys(a, exactKey), keys(b, exactKey))
		c[1]++
		floor["exact "+size] = c
		c = floor["access "+size]
		c[0] += common(keys(a, accessKey), keys(b, accessKey))
		c[1]++
		floor["access "+size] = c
	}
	for k, c := range floor {
		t.Logf("different functions, %s: mean %.1f over %.0f pairs", k, c[0]/c[1], c[1])
	}
}

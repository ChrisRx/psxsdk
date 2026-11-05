package main

import (
	"bufio"
	"cmp"
	"debug/elf"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// The score command measures how close the clean library's instructions are to
// Sony's, entry point by entry point. It is a measure for those outside the clean
// room, to see how far along a function is and which need work. It is off limits
// to the implementation side, as this whole tool is, and so is what it writes:
// a score to climb would steer an implementation toward Sony's code.

func scoreCommand() *cobra.Command {
	var image, symbols, clean, out, against string
	cmd := &cobra.Command{
		Use:   "score",
		Short: "compare the clean library's instructions with Sony's, by entry point",
		Long: `score compares each entry point's instructions on Sony's image and on the clean
library's, an ELF built by exp/cleanlib/image/make.sh. "body" is the function itself and
what it jumps to, "all" goes on through what it calls, each callee placed once at its
first call, so a helper Sony's keeps apart lines up with code Go inlined.

exact counts instructions the same but for jump targets and the high half of addresses.
shape counts mnemonics, in order, without nops. access counts the order of loads and
stores with their offsets, and GTE operations. mix is the overlap of the mnemonics in
any order. Each is 100 when the two are the same.`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			img, err := LoadImage(image, symbols)
			if err != nil {
				return err
			}
			fns := img.Functions()
			sony := &sonyCode{img: img, fns: fns}
			other, err := loadCleanCode(clean)
			if err != nil {
				return err
			}
			var rows []scoreRow
			for _, f := range fns {
				if strings.HasPrefix(f.Name, "func_") {
					continue
				}
				if row, ok := scoreEntry(f.Name, f.Start, sony, other); ok {
					rows = append(rows, row)
				}
			}
			slices.SortFunc(rows, func(a, b scoreRow) int { return cmp.Compare(a.name, b.name) })
			w := cmd.OutOrStdout()
			if out != "" {
				file, err := os.Create(out)
				if err != nil {
					return err
				}
				defer file.Close()
				w = file
			}
			before, err := readScores(against)
			if err != nil {
				return err
			}
			return writeScores(w, rows, before)
		},
	}
	f := cmd.Flags()
	f.StringVar(&image, "image", "", "a PSX-EXE of Sony's library, in place of the one built in")
	f.StringVar(&symbols, "symbols", "internal/toolchain/_tinygo/targets/playstation-yaroze-symusr.S", "the symbol file naming the entry points")
	f.StringVar(&clean, "clean", ".build/cleanimage/image.elf", "the clean library's image, as an ELF")
	f.StringVarP(&out, "out", "o", "", "the file to write, standard output if not given")
	f.StringVar(&against, "against", "", "an earlier output to show the change from")
	return cmd
}

// code is one library's instructions.
type code interface {
	word(addr uint32) (uint32, bool)
	// body lists the addresses of the function at or around addr, in order. It reports
	// false for what isn't library code to follow: the kernel, the Go runtime.
	body(addr uint32) (start uint32, addrs []uint32, ok bool)
	// code reports whether a function starts at addr, for following tables of them.
	code(addr uint32) bool
	// entry goes from an entry point to its code.
	entry(addr uint32) uint32
}

// sonyCode is Sony's image, with its functions found by Image.Functions.
type sonyCode struct {
	img *Image
	fns Functions
}

func (c *sonyCode) word(addr uint32) (uint32, bool) { return c.img.word(addr) }
func (c *sonyCode) entry(addr uint32) uint32        { return addr }

func (c *sonyCode) body(addr uint32) (uint32, []uint32, bool) {
	f := c.fns.containing(addr)
	if f == nil {
		return 0, nil, false
	}
	return f.Start, spanAddrs(f.Start, f.Start+f.Size), true
}

func (c *sonyCode) code(addr uint32) bool { return c.fns.At(addr) != nil }

func spanAddrs(start, end uint32) []uint32 {
	addrs := make([]uint32, 0, (end-start)/4)
	for a := start; a < end; a += 4 {
		addrs = append(addrs, a)
	}
	return addrs
}

type cleanSymbol struct {
	name       string
	addr, size uint32
}

// cleanCode is the clean library's ELF, whose functions its symbols give.
type cleanCode struct {
	base    uint32
	data    []byte
	symbols []cleanSymbol
}

func loadCleanCode(path string) (*cleanCode, error) {
	f, err := elf.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	const base, top = 0x80010000, 0x80090000
	c := &cleanCode{base: base, data: make([]byte, top-base)}
	for _, p := range f.Progs {
		if p.Type != elf.PT_LOAD || p.Filesz == 0 || p.Vaddr < base || p.Vaddr >= top {
			continue
		}
		if _, err := p.ReadAt(c.data[p.Vaddr-base:min(p.Vaddr-base+p.Filesz, top-base)], 0); err != nil && err != io.EOF {
			return nil, err
		}
	}
	syms, err := f.Symbols()
	if err != nil {
		return nil, err
	}
	for _, s := range syms {
		if s.Name != "" && s.Value >= base && s.Value < top && elf.ST_TYPE(s.Info) != elf.STT_OBJECT {
			c.symbols = append(c.symbols, cleanSymbol{name: s.Name, addr: uint32(s.Value), size: uint32(s.Size)})
		}
	}
	slices.SortStableFunc(c.symbols, func(a, b cleanSymbol) int { return cmp.Compare(a.addr, b.addr) })
	return c, nil
}

func (c *cleanCode) word(addr uint32) (uint32, bool) {
	off := int64(addr) - int64(c.base)
	if off < 0 || off+4 > int64(len(c.data)) {
		return 0, false
	}
	d := c.data[off:]
	return uint32(d[0]) | uint32(d[1])<<8 | uint32(d[2])<<16 | uint32(d[3])<<24, true
}

// entry follows the jumps of the table of entry points.
func (c *cleanCode) entry(addr uint32) uint32 {
	for range 4 {
		w, ok := c.word(addr)
		if !ok || w>>26 != 2 {
			break
		}
		addr = (addr+4)&0xf0000000 | (w&0x3ffffff)<<2
	}
	return addr
}

func (c *cleanCode) symbol(addr uint32) (cleanSymbol, bool) {
	i, _ := slices.BinarySearchFunc(c.symbols, addr+1, func(s cleanSymbol, a uint32) int { return cmp.Compare(s.addr, a) })
	if i == 0 {
		return cleanSymbol{}, false
	}
	return c.symbols[i-1], true
}

func (c *cleanCode) code(addr uint32) bool {
	s, ok := c.symbol(addr)
	return ok && s.addr == addr
}

func (c *cleanCode) body(addr uint32) (uint32, []uint32, bool) {
	s, ok := c.symbol(addr)
	if !ok {
		return 0, nil, false
	}
	for _, skip := range []string{"runtime.", "internal/", "libps_image_missing", "tinygo_"} {
		if strings.HasPrefix(s.name, skip) {
			return 0, nil, false
		}
	}
	if s.size != 0 {
		if addr >= s.addr+s.size {
			return 0, nil, false
		}
		return s.addr, spanAddrs(s.addr, s.addr+s.size), true
	}
	return s.addr, c.flow(s.addr), true
}

// flow follows the code from a label in assembly, which has no size: through its
// branches and jumps, ending at a return, a jump away, or another symbol.
func (c *cleanCode) flow(start uint32) []uint32 {
	seen := map[uint32]bool{}
	work := []uint32{start}
	for len(work) > 0 {
		a := work[len(work)-1]
		work = work[:len(work)-1]
		for !seen[a] && (a == start || !c.code(a)) {
			w, ok := c.word(a)
			if !ok {
				break
			}
			seen[a] = true
			in := Decode(a, w)
			if in.Kind == insnBranch {
				work = append(work, in.Target)
			}
			if in.Kind == insnJump || in.Kind == insnJumpReg && in.Mnemonic == "jr" {
				seen[a+4] = true
				if in.Kind == insnJump && !c.code(in.Target) {
					work = append(work, in.Target)
				}
				break
			}
			a += 4
		}
	}
	addrs := make([]uint32, 0, len(seen))
	for a := range seen {
		addrs = append(addrs, a)
	}
	slices.Sort(addrs)
	return addrs
}

// flatten lists the instructions from entry addr in order of address, in the place of
// each jump out of the function, and of each call if calls is set, the function it
// reaches, once.
func flatten(c code, addr uint32, calls bool) []scored {
	seen := map[uint32]bool{}
	var out []scored
	var walk func(addr uint32)
	walk = func(addr uint32) {
		addr = c.entry(addr)
		start, addrs, ok := c.body(addr)
		if !ok || seen[start] {
			return
		}
		seen[start] = true
		own := map[uint32]bool{}
		for _, a := range addrs {
			own[a] = true
		}
		// hi is the high half each register holds, for finding what code a table reaches.
		var hi [32]uint32
		for _, a := range addrs {
			w, ok := c.word(a)
			if !ok {
				return
			}
			in := Decode(a, w)
			out = append(out, scored{Insn: in, Word: w})
			switch {
			case in.Kind == insnCall && calls:
				walk(in.Target)
			case in.Kind == insnJump && !own[in.Target]:
				walk(in.Target)
			case in.Kind == insnLui:
				hi[in.Rd] = uint32(in.Imm) << 16
				continue
			}
			if !calls {
				continue
			}
			// lui then addiu, or lui, an addu of an index, and a load: the address of a
			// function or of a table of them, which is how polygon routines are chosen.
			if in.Rs >= 0 && hi[in.Rs] != 0 && (in.Mnemonic == "addiu" || in.Mnemonic == "lw") {
				for t := hi[in.Rs] + uint32(in.Imm); ; t += 4 {
					target := t
					if in.Mnemonic == "lw" || !c.code(t) {
						v, ok := c.word(t)
						if !ok || !c.code(v) {
							break
						}
						target = v
					}
					walk(target)
					if target == t {
						break
					}
				}
			}
			if in.Rd > 0 && in.Kind == insnPlain && in.Mnemonic != "addu" {
				hi[in.Rd] = 0
			}
		}
	}
	walk(addr)
	return out
}

// A scored is a decoded instruction and its word.
type scored struct {
	Insn
	Word uint32
}

// exactKey is the word but for what depends on where things are.
func exactKey(in scored) (uint32, bool) {
	switch in.Kind {
	case insnJump, insnCall:
		return in.Word >> 26 << 26, true
	case insnBranch, insnLui:
		return in.Word >> 16 << 16, true
	}
	return in.Word, true
}

func shapeKey(in scored) (string, bool) { return in.Mnemonic, in.Word != 0 }

// accessKey keeps what reaches outside the registers: memory, with its offset, and the GTE.
func accessKey(in scored) (string, bool) {
	switch {
	case in.Kind == insnLoadStore:
		return in.Mnemonic + strconv.Itoa(int(in.Imm)), true
	case in.Word>>26 == 0x12:
		return strconv.FormatUint(uint64(in.Word), 16), true
	}
	return "", false
}

func keys[K comparable](list []scored, key func(scored) (K, bool)) []K {
	out := make([]K, 0, len(list))
	for _, in := range list {
		if k, ok := key(in); ok {
			out = append(out, k)
		}
	}
	return out
}

// maxCells bounds a comparison's table, at the cost of leaving the longest out.
const maxCells = 1 << 29

// common is the share of two lists, from 0 to 100, in their longest common subsequence,
// or -1 if they are too long to compare.
func common[K comparable](a, b []K) float64 {
	if len(a)+len(b) == 0 {
		return 100
	}
	if len(a)*len(b) > maxCells {
		return -1
	}
	prev, cur := make([]int32, len(b)+1), make([]int32, len(b)+1)
	for _, x := range a {
		for j, y := range b {
			switch {
			case x == y:
				cur[j+1] = prev[j] + 1
			case prev[j+1] > cur[j]:
				cur[j+1] = prev[j+1]
			default:
				cur[j+1] = cur[j]
			}
		}
		prev, cur = cur, prev
	}
	return 200 * float64(prev[len(b)]) / float64(len(a)+len(b))
}

// overlap is the share of mnemonics the two lists have in common, in any order.
func overlap(a, b []string) float64 {
	count := map[string][2]int{}
	for _, k := range a {
		c := count[k]
		c[0]++
		count[k] = c
	}
	for _, k := range b {
		c := count[k]
		c[1]++
		count[k] = c
	}
	both, either := 0, 0
	for _, c := range count {
		both += min(c[0], c[1])
		either += max(c[0], c[1])
	}
	if either == 0 {
		return 100
	}
	return 100 * float64(both) / float64(either)
}

type scoreRow struct {
	name                            string
	sonyBody, cleanBody             int
	exactBody, shapeBody            float64
	sonyAll, cleanAll               int
	exactAll, shapeAll, access, mix float64
}

// scoreEntry compares one entry point, or reports false if the clean library has no code for it.
func scoreEntry(name string, addr uint32, sony, clean code) (scoreRow, bool) {
	if _, _, ok := clean.body(clean.entry(addr)); !ok {
		return scoreRow{}, false
	}
	a, b := flatten(sony, addr, false), flatten(clean, addr, false)
	x, y := flatten(sony, addr, true), flatten(clean, addr, true)
	return scoreRow{
		name:      name,
		sonyBody:  len(a),
		cleanBody: len(b),
		exactBody: common(keys(a, exactKey), keys(b, exactKey)),
		shapeBody: common(keys(a, shapeKey), keys(b, shapeKey)),
		sonyAll:   len(x),
		cleanAll:  len(y),
		exactAll:  common(keys(x, exactKey), keys(y, exactKey)),
		shapeAll:  common(keys(x, shapeKey), keys(y, shapeKey)),
		access:    common(keys(x, accessKey), keys(y, accessKey)),
		mix:       overlap(keys(x, shapeKey), keys(y, shapeKey)),
	}, true
}

// readScores reads an earlier output's shape over everything, by name.
func readScores(path string) (map[string]float64, error) {
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	before := map[string]float64{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		field := strings.Fields(sc.Text())
		if len(field) >= 9 && !strings.HasPrefix(field[0], "#") {
			if v, err := strconv.ParseFloat(field[8], 64); err == nil {
				before[field[0]] = v
			}
		}
	}
	return before, sc.Err()
}

func writeScores(w io.Writer, rows []scoreRow, before map[string]float64) error {
	bw := bufio.NewWriter(w)
	var sum, shape, exact float64
	for _, r := range rows {
		weight := float64(r.sonyBody)
		sum += weight
		shape += weight * r.shapeBody
		exact += weight * r.exactBody
	}
	fmt.Fprintf(bw, "# %d entries. Over Sony's own instructions, body: exact %.1f, shape %.1f\n", len(rows), exact/sum, shape/sum)
	for _, floor := range []float64{95, 80, 50} {
		n := 0
		for _, r := range rows {
			if r.shapeAll >= floor {
				n++
			}
		}
		fmt.Fprintf(bw, "# shape over everything at %.0f or more: %d\n", floor, n)
	}
	fmt.Fprintf(bw, "# %-22s %6s %6s %6s %6s %6s %6s %6s %6s %6s %6s\n", "entry", "sony", "clean", "exact", "shape", "Sony", "Clean", "exact", "shape", "access", "mix")
	for _, r := range rows {
		fmt.Fprintf(bw, "%-24s %6d %6d %6.1f %6.1f %6d %6d %6.1f %6.1f %6.1f %6.1f", r.name, r.sonyBody, r.cleanBody, r.exactBody, r.shapeBody, r.sonyAll, r.cleanAll, r.exactAll, r.shapeAll, r.access, r.mix)
		if v, ok := before[r.name]; ok {
			fmt.Fprintf(bw, " %+6.1f", r.shapeAll-v)
		}
		fmt.Fprintln(bw)
	}
	return bw.Flush()
}

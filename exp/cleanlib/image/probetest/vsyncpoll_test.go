package probetest

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	"go.chrisrx.dev/psx/emu/cpu"
	"go.chrisrx.dev/psx/emu/emutest"
	"go.chrisrx.dev/x/assert"
)

var vsyncpoll = flag.String("vsyncpoll", "", "the cue sheet of the vsyncpoll probe's disc, as probe.sh builds it, for TestVSyncPoll")

// pollBlank is a vertical blank interrupt taken during one of vsyncpoll's
// calls of VSync(-1): when, in cycles from the program's read of timer 2
// before the call, and the call's accesses to the devices before it.
type pollBlank struct {
	taken    int64
	accesses []string
}

// pollRun is what vsyncpoll printed and what was timed from outside.
type pollRun struct {
	tty    string
	blanks []pollBlank
	// calls holds, for an uninterrupted call, its accesses to the devices,
	// in cycles from the read of timer 2 before it, and the read after, by
	// display.
	calls map[string]string
}

// runPoll runs the vsyncpoll probe on disc to its END, with the CPU charging
// a console's costs or not, and times from outside each vertical blank
// interrupt taken in a poll.
func runPoll(t *testing.T, disc string, cost bool) pollRun {
	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(disc), emutest.AllowFaults())
	m.SetCPUCost(cost)
	m.RunUntilTTY("start", 30*emutest.Second)
	type access struct {
		what  string
		cycle uint64
	}
	var since []access // the accesses since the last read of timer 2
	var lastT2 uint64
	run := pollRun{calls: map[string]string{}}
	quiet := false
	m.OnBus(func(addr uint32, size int, v uint32, write bool) {
		if quiet {
			return
		}
		a := addr & 0x1fffffff
		now := m.CPU.TotalCycles
		if a == 0x1f801120 && !write {
			// A read of timer 2 after a call, with nothing between: keep
			// the call's accesses once for each display.
			if len(since) > 0 && now-lastT2 < 400 {
				var b strings.Builder
				for _, x := range since {
					fmt.Fprintf(&b, "%s@%d ", x.what, x.cycle-lastT2)
				}
				fmt.Fprintf(&b, "timer2@%d", now-lastT2)
				gpu := m.Read32(0x1f801814)
				key := "240p"
				if gpu&(1<<19) != 0 {
					key = "480i"
				}
				if _, ok := run.calls[key]; !ok && strings.Count(b.String(), "@") > 0 {
					run.calls[key] = b.String()
				}
			}
			since = since[:0]
			lastT2 = now
			return
		}
		if a >= 0x1f801000 && a < 0x1f802000 && len(since) < 16 {
			dir := "r"
			if write {
				dir = "w"
			}
			since = append(since, access{fmt.Sprintf("%s%x", dir, a&0xfff), now})
		}
	})
	pending := false
	budget := m.Cycles() + 20*emutest.Second
	for !strings.Contains(m.TTY(), "END") && m.Cycles() < budget {
		m.Step()
		if m.CPU.Excode == cpu.ExcInt && pending {
			pending = false
			if off := int64(m.CPU.TotalCycles - lastT2); off < 400 {
				b := pollBlank{taken: off}
				for _, x := range since {
					b.accesses = append(b.accesses, fmt.Sprintf("%s@%d", x.what, x.cycle-lastT2))
				}
				run.blanks = append(run.blanks, b)
			}
		}
		quiet = true
		stat := m.Read32(0x1f801070)
		quiet = false
		if stat&1 != 0 && !pending {
			pending = true
		}
	}
	run.tty = strings.ReplaceAll(m.TTY(), "\r", "")
	return run
}

// pollExpect is what Sony's library does in vsyncpoll, in the emulator,
// with the CPU's console costs off or on (spec 45).
type pollExpect struct {
	// call is an uninterrupted VSync(-1)'s accesses, in cycles from the
	// program's read of timer 2 before it, as runPoll writes them.
	call string
	// An interrupt taken in the call at counted cycles or before is in its
	// result; one at missed or after is not.
	counted, missed int64
	// The range of cycles from the end of the interrupt to VSync(0)'s read
	// of timer 1 on its way out at 240 lines, and from GPUSTAT bit 31's
	// change at 480 lines.
	ret240, ret480 [2]int64
}

var pollSony = map[bool]pollExpect{
	false: {
		call:    "r814@22 r110@24 timer2@58",
		counted: 38, missed: 42,
		ret240: [2]int64{38, 60}, ret480: [2]int64{24, 36},
	},
	true: {
		call:    "r814@29 r110@34 timer2@",
		counted: 49, missed: 56,
		ret240: [2]int64{75, 152}, ret480: [2]int64{46, 51},
	},
}

// TestVSyncPoll runs spec 45's probe, spec/probes/vsyncpoll, which polls
// VSync(-1) across the vertical blank at a different point each field and
// then waits in VSync(0) after a different delay each field, and checks
// from outside that the clean library does what Sony's does, with the CPU's
// console costs off and on: VSync(-1) reads GPUSTAT and timer 1 and then the
// count, at Sony's cycles, so that an interrupt taken in the call is in its
// result just when it is in Sony's; and VSync(0) leaves the program at
// Sony's cycles after the interrupt, at 240 lines, or after bit 31's change,
// at 480. Together they set where a polling program is when the next blank
// comes, and so on which poll it sees the count. Build the disc with
//
//	SIDES=clean exp/cleanlib/image/probe.sh vsyncpoll exp/cleanlib/spec/probes/vsyncpoll
//
// and pass -vsyncpoll=$PWD/.build/vsyncpoll-cimg-clean/vsyncpoll-cimg-clean.cue.
func TestVSyncPoll(t *testing.T) {
	if *vsyncpoll == "" {
		t.Skip("-vsyncpoll names the probe's disc")
	}
	if _, err := os.Stat(*vsyncpoll); err != nil {
		t.Fatal(err)
	}
	for _, cost := range []bool{false, true} {
		want := pollSony[cost]
		run := runPoll(t, *vsyncpoll, cost)
		var b strings.Builder
		fmt.Fprintf(&b, "cost %v\n", cost)
		for k, v := range run.calls {
			fmt.Fprintf(&b, "call %s: %s\n", k, v)
		}
		var polls []string
		for line := range strings.SplitSeq(run.tty, "\n") {
			if strings.HasPrefix(line, "POLL") && strings.Contains(line, "delay") {
				polls = append(polls, line)
			}
		}
		for i, x := range run.blanks {
			p := ""
			if i < len(polls) {
				p = polls[i]
			}
			fmt.Fprintf(&b, "taken %d after %v: %s\n", x.taken, x.accesses, p)
		}
		waits := map[string][]waitBlank{}
		for _, name := range []string{"240p", "480i"} {
			waits[name] = runWaits(t, *vsyncpoll, name, cost)
			for _, w := range waits[name] {
				fmt.Fprintf(&b, "wait %s: end %d field %d ret %d: ret-end %d ret-field %d\n", name, w.end, w.field, w.ret, w.ret-w.end, w.ret-w.field)
			}
		}
		t.Log("\n" + b.String())

		where := fmt.Sprintf("cost %v: ", cost)
		assert.Equal(t, true, strings.Contains(run.tty, "END"), where+"the probe ends")
		assert.Equal(t, 2, len(run.calls), where+"calls timed at both heights")
		for k, v := range run.calls {
			assert.Equal(t, true, strings.HasPrefix(v, want.call), where+k+": VSync(-1)'s accesses at Sony's cycles: "+v)
		}
		assert.Equal(t, len(polls), len(run.blanks), where+"a blank for each field")
		inCall := 0
		for i, x := range run.blanks {
			if i >= len(polls) || !strings.Contains(polls[i], "in call") {
				continue
			}
			inCall++
			counted := strings.HasSuffix(polls[i], "counted 1")
			name := fmt.Sprintf("%s%s, taken at %d", where, polls[i], x.taken)
			if x.taken <= want.counted {
				assert.Equal(t, true, counted, name+": in the result")
			}
			if x.taken >= want.missed {
				assert.Equal(t, false, counted, name+": not in the result")
			}
		}
		assert.Between(t, 20, 96, inCall, where+"blanks in a call")
		for _, w := range waits["240p"] {
			assert.Between(t, want.ret240[0], want.ret240[1], w.ret-w.end, where+"240 lines: VSync(0)'s return after the interrupt")
		}
		for _, w := range waits["480i"] {
			if w.field > w.end {
				assert.Between(t, want.ret480[0], want.ret480[1], w.ret-w.field, where+"480 lines: VSync(0)'s return after bit 31's change")
			}
		}
	}
}

// waitBlank is a vertical blank during one of vsyncpoll's waiting VSync(0)
// calls, in cycles from I_STAT bit 0: when the interrupt ended (the CPU's
// interrupts enabled again), when GPUSTAT bit 31 next changed, and when
// VSync read timer 1 on its way out.
type waitBlank struct {
	end, field, ret int64
}

// runWaits runs vsyncpoll on disc from its WAIT section for display name
// ("240p" or "480i") and times each blank in it.
func runWaits(t *testing.T, disc, name string, cost bool) []waitBlank {
	m := emutest.Boot(t, emutest.BIOS(emutest.BIOSFromEnv(t)), emutest.Disc(disc), emutest.AllowFaults())
	m.SetCPUCost(cost)
	m.RunUntilTTY("POLL "+name+" least", 60*emutest.Second)
	var out []waitBlank
	var cur *waitBlank
	var t0 uint64
	quiet := false
	m.OnBus(func(addr uint32, size int, v uint32, write bool) {
		if quiet || cur == nil {
			return
		}
		if addr&0x1fffffff == 0x1f801110 && !write && cur.end >= 0 && cur.ret < 0 {
			cur.ret = int64(m.CPU.TotalCycles - t0)
		}
	})
	pending, inInt := false, false
	quiet = true
	field := m.Read32(0x1f801814) >> 31
	quiet = false
	budget := m.Cycles() + 30*emutest.Second
	for !strings.Contains(m.TTY(), "WAIT "+name+" delay 0") && m.Cycles() < budget {
		m.Step()
		if m.CPU.Excode == cpu.ExcInt {
			inInt = true
		}
		quiet = true
		stat := m.Read32(0x1f801070)
		f := m.Read32(0x1f801814) >> 31
		quiet = false
		if cur != nil {
			if inInt && m.CPU.COP0[cpu.SR]&1 != 0 && cur.end < 0 {
				cur.end = int64(m.CPU.TotalCycles - t0)
			}
			if f != field && cur.field < 0 {
				cur.field = int64(m.CPU.TotalCycles - t0)
			}
		}
		if m.CPU.COP0[cpu.SR]&1 != 0 {
			inInt = false
		}
		field = f
		if stat&1 != 0 && !pending {
			pending = true
			if cur != nil {
				out = append(out, *cur)
			}
			t0 = m.CPU.TotalCycles
			cur = &waitBlank{end: -1, field: -1, ret: -1}
		} else if stat&1 == 0 {
			pending = false
		}
	}
	return out
}

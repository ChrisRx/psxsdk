//go:build !psx.onstore

package yaroze

// The compiler's onstore experiment is off (--experiment=noonstore, or a
// compiler without it, such as the go command's): nothing calls pack when a
// field is set, so drawing packs the attribute word each time.

func (sp *Sprite) repack()      { sp.pack() }
func (b *Box) repack()          { b.pack() }
func (l *Line) repack()         { l.pack() }
func (l *GradientLine) repack() { l.pack() }
func (bg *Background) repack()  { bg.pack() }

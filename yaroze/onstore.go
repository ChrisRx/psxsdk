//go:build psx.onstore

package yaroze

// The compiler's onstore experiment is on: it calls each type's pack after
// every store to the fields its //go:onstore pragma names, so the attribute
// words are always packed and drawing has nothing to do. Building with
// --experiment=noonstore leaves the experiment and this file out, and
// drawing packs them instead (onstore_off.go).

func (sp *Sprite) repack()      {}
func (b *Box) repack()          {}
func (l *Line) repack()         {}
func (l *GradientLine) repack() {}
func (bg *Background) repack()  {}

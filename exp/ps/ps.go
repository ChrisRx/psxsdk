// Package ps is the PlayStation as a Go program sees it: a screen drawn
// frame by frame, sprites and boxes on it, text, the controllers, and
// music from a sound bank. It is the Net Yaroze library under an ordinary Go
// surface: types with methods, strings, ints and errors, where the yaroze
// package keeps the library's C names and signatures, and which this sits on.
//
// A program builds with the library linked in, or with --libps resident laid
// over the image a Net Yaroze leaves in memory, from the same source.
//
//	var screen = ps.Screen{Background: ps.Color{R: 60, G: 120, B: 120}}
//
//	func main() {
//		screen.Open()
//		text := screen.Font(16, 16, 256, 16, 64)
//		ship := ps.LoadTIM(shipFile.Data()).Sprite()
//		for frame := range screen.Frames() {
//			if ps.Pad(0).Has(ps.Right) {
//				ship.X++
//			}
//			ship.Draw(frame, 0)
//			text.Print("frame ").Int(frame.Number())
//		}
//	}
//
// The package is experimental, and so is the library under it: nothing
// here is settled, and the yaroze package stays the default.
//
// On a host the package compiles whole, so an editor, go vet and a
// program's own tests see it, and each call on the library panics with
// a message that says so: a game's logic can be tested with go test,
// and the drawing only on the console or the emulator.
//
// Nothing in the package allocates after Open, and nothing may: the
// collector on this target frees nothing, so a program allocates at
// startup and keeps what it made.
package ps

//go:build !yaroze

package yaroze

import (
	"go.chrisrx.dev/psx/geom"
	"go.chrisrx.dev/psx/gpu"
)

// The host side of the Go API, for every build that is not for the
// console, where an editor, go vet and a program's own tests see it:
// the types are whole, and each call on the library panics through
// stub, as the library's own functions do.

func openScreen(s *Screen)                       { panic(stub("Screen.Open")) }
func resetGraph()                                { panic(stub("Screen.Close")) }
func beginFrame(s *Screen) *Frame                { panic(stub("Screen.Frames")) }
func endFrame(s *Screen) int32                   { panic(stub("Screen.Frames")) }
func vsync(n int32) int32                        { panic(stub("Screen.Wait")) }
func fontOpen(x, y, w, h, chars int) int32       { panic(stub("Screen.Font")) }
func fntWrite(id int32, s string)                { panic(stub("Text.Print")) }
func fntWriteBytes(id int32, b []byte)           { panic(stub("Text.Int")) }
func sortSprite(sp *Sprite, f *Frame, depth int) { panic(stub("Sprite.Draw")) }
func sortFastSprite(sp *Sprite, f *Frame, depth int) {
	panic(stub("Sprite.DrawFast"))
}
func sortBox(b *Box, f *Frame, depth int)   { panic(stub("Box.Draw")) }
func sortLine(l *Line, f *Frame, depth int) { panic(stub("Line.Draw")) }
func sortGradientLine(l *GradientLine, f *Frame, depth int) {
	panic(stub("GradientLine.Draw"))
}
func initBackground(bg *Background) { panic(stub("Background.Init")) }
func sortBackground(bg *Background, f *Frame, depth int) {
	panic(stub("Background.Draw"))
}
func loadTIM(words []uint32, at bool, x, y int) Image      { panic(stub("LoadTIM")) }
func allocPacket(n int) *uint32                            { panic(stub("Frame.Packet")) }
func linkPacket(f *Frame, depth int, tag *gpu.Tag)         { panic(stub("Frame.Packet")) }
func linkAfter(prev, tag *gpu.Tag)                         { panic(stub("Frame.PacketAfter")) }
func mainVolume(left, right int)                           { panic(stub("SetVolume")) }
func vabTransfer(vh, vb []uint32) int16                    { panic(stub("OpenBank")) }
func vabClose(id int16)                                    { panic(stub("Bank.Close")) }
func seqOpen(seq []uint32, vab int16) int16                { panic(stub("Bank.Sequence")) }
func seqPlay(id int16, times int16)                        { panic(stub("Sequence.Play")) }
func seqVolume(id int16, left, right int)                  { panic(stub("Sequence.SetVolume")) }
func seqPause(id int16)                                    { panic(stub("Sequence.Pause")) }
func seqResume(id int16)                                   { panic(stub("Sequence.Resume")) }
func seqStop(id int16)                                     { panic(stub("Sequence.Stop")) }
func seqPlaying(id int16) bool                             { panic(stub("Sequence.Playing")) }
func seqClose(id int16)                                    { panic(stub("Sequence.Close")) }
func randNext() int                                        { panic(stub("Random")) }
func randSeed(s uint32)                                    { panic(stub("Seed")) }
func origin() (x, y int)                                   { panic(stub("Screen.Bounds")) }
func mapModel(words []uint32)                              { panic(stub("LoadTMD")) }
func initCoord(c, super *coordImpl)                        { panic(stub("Object.Init")) }
func linkObject(o *objectImpl, m Model, n int)             { panic(stub("Object.Link")) }
func objectMatrices(o *Object, world, screen *geom.Matrix) { panic(stub("Object.Transform")) }
func sortObject(o *Object, f *Frame)                       { panic(stub("Object.Sort")) }
func setLightMatrix(m *geom.Matrix)                        { panic(stub("SetLightMatrix")) }
func setScreenMatrix(m *geom.Matrix)                       { panic(stub("SetScreenMatrix")) }
func setVideo(mode VideoMode)                              { panic(stub("Screen.SetVideo")) }
func shiftOffset(dx, dy int)                               { panic(stub("Screen.Shift")) }
func backdropOf(tim []uint32) (*uint32, int16, int16)      { panic(stub("Screen.SetBackdrop")) }
func copyRect(src geom.Rect, x, y int)                     { panic(stub("CopyRect")) }
func clearRect(r geom.Rect, c Color)                       { panic(stub("ClearRect")) }
func loadRect(r geom.Rect, words []uint32)                 { panic(stub("LoadRect")) }
func storeRect(r geom.Rect, words []uint32)                { panic(stub("StoreRect")) }
func setReverb(kind Reverb, left, right int)               { panic(stub("SetReverb")) }
func reverbOff()                                           { panic(stub("ReverbOff")) }
func setCamera(c *Camera)                                  { panic(stub("Camera.Set")) }
func setLight(n int, l Light)                              { panic(stub("SetLight")) }
func setAmbient(r, g, b geom.Fixed)                        { panic(stub("SetAmbient")) }
func modelVectors(m Model, i int) []Vertex                 { panic(stub("Model.Vertices")) }
func blend(vertices, diffs *Vertex, n int, p geom.Fixed)   { panic(stub("Blend")) }
func setFog(dqa int16, dqb int32, c Color)                 { panic(stub("SetFog")) }
func setLightMode(mode int32)                              { panic(stub("SetFog")) }
func startCounter(c Counter)                               { panic(stub("Counter.Start")) }
func resetCounter(c Counter)                               { panic(stub("Counter.Reset")) }
func readCounter(c Counter) int                            { panic(stub("Counter.Read")) }
func fieldCount() int                                      { panic(stub("FieldCount")) }
func linesSinceBlank() int                                 { panic(stub("LinesSinceBlank")) }
func videoMode() VideoMode                                 { panic(stub("Screen.Video")) }
func testCard(slot int) CardState                          { panic(stub("CardTest")) }
func fileOpen(name *byte, mode int32) int32                { panic(stub("CardWrite")) }
func fileRead(fd int32, buf *byte, n int32) int32          { panic(stub("CardRead")) }
func fileWrite(fd int32, buf *byte, n int32) int32         { panic(stub("CardWrite")) }
func fileClose(fd int32)                                   { panic(stub("CardWrite")) }
func fileFormat(name *byte) int32                          { panic(stub("CardFormat")) }
func fileDelete(name *byte) int32                          { panic(stub("CardDelete")) }
func fileFirst(name *byte, f *CardFile) bool               { panic(stub("CardFiles")) }
func fileNext(f *CardFile) bool                            { panic(stub("CardFiles")) }
func discVolume(left, right int)                           { panic(stub("SetDiscVolume")) }
func cdPlay(mode int, tracks []int, from int) int          { panic(stub("PlayTracks")) }
func kanjiOpen(x, y, w, h, tx, ty, cx, cy, n int) int32    { panic(stub("Screen.KanjiFont")) }
func kanjiPrint(id int32, s string)                        { panic(stub("KanjiText.Print")) }
func kanjiImage(s string, words []uint32, x, y, cx, cy int, fg, bg uint16) bool {
	panic(stub("KanjiImage"))
}
func useCamera()                                    { panic(stub("UseCamera")) }
func project3(a, b, c *Vertex, sxy *[3]gpu.XY) bool { panic(stub("Project3")) }
func project(v *Vertex, sxy *gpu.XY) bool           { panic(stub("Project")) }
func clipArea() int                                 { panic(stub("ClipArea")) }
func averageDepth3() int                            { panic(stub("AverageDepth3")) }
func averageDepth4() int                            { panic(stub("AverageDepth4")) }

//go:build yaroze

package main

import (
	"unsafe"

	"go.chrisrx.dev/psx/exp/libps"
)

var _ unsafe.Pointer

var keep = [...]func(){
	func() { libps.ApplyMatrix(opaque[*libps.MATRIX](), opaque[*libps.SVECTOR](), opaque[*libps.VECTOR]()) },
	func() { libps.ApplyMatrixLV(opaque[*libps.MATRIX](), opaque[*libps.VECTOR](), opaque[*libps.VECTOR]()) },
	func() {
		libps.ApplyMatrixSV(opaque[*libps.MATRIX](), opaque[*libps.SVECTOR](), opaque[*libps.SVECTOR]())
	},
	func() { libps.CdControl(opaque[uint8](), opaque[*uint8](), opaque[*uint8]()) },
	func() { libps.CdControlB(opaque[uint8](), opaque[*uint8](), opaque[*uint8]()) },
	func() { libps.CdControlF(opaque[uint8](), opaque[*uint8](), opaque[*uint8]()) },
	func() { libps.CdFlush() },
	func() { libps.CdInit() },
	func() { libps.CdIntToPos(opaque[int32](), opaque[*libps.CdlLOC]()) },
	func() { libps.CdLastCom() },
	func() { libps.CdLastPos() },
	func() { libps.CdMode() },
	func() { libps.CdPlay(opaque[int32](), opaque[*int32](), opaque[int32]()) },
	func() { libps.CdPosToInt(opaque[*libps.CdlLOC]()) },
	func() { libps.CdRead(opaque[int32](), opaque[*uint32](), opaque[int32]()) },
	func() { libps.CdReadExec(opaque[*byte]()) },
	func() { libps.CdReadFile(opaque[*byte](), opaque[*uint32](), opaque[int32]()) },
	func() { libps.CdReadSync(opaque[int32](), opaque[*uint8]()) },
	func() { libps.CdReady(opaque[int32](), opaque[*uint8]()) },
	func() { libps.CdSearchFile(opaque[*libps.CdlFILE](), opaque[*byte]()) },
	func() { libps.CdSetDebug(opaque[int32]()) },
	func() { libps.CdStatus() },
	func() { libps.CdSync(opaque[int32](), opaque[*uint8]()) },
	func() { libps.ClearImage(opaque[*libps.RECT](), opaque[uint8](), opaque[uint8](), opaque[uint8]()) },
	func() { libps.ClearOTagR(opaque[*uint32](), opaque[int32]()) },
	func() { libps.CompMatrix(opaque[*libps.MATRIX](), opaque[*libps.MATRIX](), opaque[*libps.MATRIX]()) },
	func() { libps.DrawOTag(opaque[*uint32]()) },
	func() { libps.DrawSync(opaque[int32]()) },
	func() { libps.EnterCriticalSection() },
	func() { libps.ExitCriticalSection() },
	func() { libps.FntFlush(opaque[int32]()) },
	func() { libps.FntLoad(opaque[int32](), opaque[int32]()) },
	func() {
		libps.FntOpen(opaque[int32](), opaque[int32](), opaque[int32](), opaque[int32](), opaque[int32](), opaque[int32]())
	},
	func() { libps.FntPrint(opaque[*byte]()) },
	func() { libps.GetClut(opaque[int32](), opaque[int32]()) },
	func() { libps.GetGraphDebug() },
	func() { libps.GetGraphType() },
	func() { libps.GetPadBuf(opaque[**uint8](), opaque[**uint8]()) },
	func() { libps.GetRCnt(opaque[uint32]()) },
	func() { libps.GetTPage(opaque[int32](), opaque[int32](), opaque[int32](), opaque[int32]()) },
	func() { libps.GetVideoMode() },
	func() { libps.GsClearOt(opaque[uint16](), opaque[uint16](), opaque[*libps.GsOT]()) },
	func() { libps.GsDefDispBuff(opaque[uint16](), opaque[uint16](), opaque[uint16](), opaque[uint16]()) },
	func() { libps.GsDrawOt(opaque[*libps.GsOT]()) },
	func() { libps.GsGetActiveBuff() },
	func() { libps.GsGetLs(opaque[*libps.GsCOORDINATE2](), opaque[*libps.MATRIX]()) },
	func() { libps.GsGetLw(opaque[*libps.GsCOORDINATE2](), opaque[*libps.MATRIX]()) },
	func() {
		libps.GsGetLws(opaque[*libps.GsCOORDINATE2](), opaque[*libps.MATRIX](), opaque[*libps.MATRIX]())
	},
	func() { libps.GsGetTimInfo(opaque[*uint32](), opaque[*libps.GsIMAGE]()) },
	func() { libps.GsGetWorkBase() },
	func() { libps.GsInit3D() },
	func() { libps.GsInitCoordinate2(opaque[*libps.GsCOORDINATE2](), opaque[*libps.GsCOORDINATE2]()) },
	func() { libps.GsInitFixBg16(opaque[*libps.GsBG](), opaque[*uint32]()) },
	func() {
		libps.GsInitGraph(opaque[uint16](), opaque[uint16](), opaque[uint16](), opaque[uint16](), opaque[uint16]())
	},
	func() { libps.GsLinkObject4(opaque[uint32](), opaque[*libps.GsDOBJ2](), opaque[int32]()) },
	func() { libps.GsMapModelingData(opaque[*uint32]()) },
	func() { libps.GsScaleScreen(opaque[*libps.SVECTOR]()) },
	func() { libps.GsSetAmbient(opaque[int32](), opaque[int32](), opaque[int32]()) },
	func() { libps.GsSetClip(opaque[*libps.RECT]()) },
	func() { libps.GsSetClip2D(opaque[*libps.RECT]()) },
	func() { libps.GsSetDrawBuffClip() },
	func() { libps.GsSetDrawBuffOffset() },
	func() { libps.GsSetFlatLight(opaque[int32](), opaque[*libps.GsF_LIGHT]()) },
	func() { libps.GsSetFogParam(opaque[*libps.GsFOGPARAM]()) },
	func() { libps.GsSetLightMatrix(opaque[*libps.MATRIX]()) },
	func() { libps.GsSetLightMode(opaque[int32]()) },
	func() { libps.GsSetLsMatrix(opaque[*libps.MATRIX]()) },
	func() { libps.GsSetOffset(opaque[int32](), opaque[int32]()) },
	func() { libps.GsSetOrign(opaque[int16](), opaque[int16]()) },
	func() { libps.GsSetProjection(opaque[int32]()) },
	func() { libps.GsSetRefView2(opaque[*libps.GsRVIEW2]()) },
	func() { libps.GsSetView2(opaque[*libps.GsVIEW2]()) },
	func() { libps.GsSetWorkBase(opaque[*libps.PACKET]()) },
	func() { libps.GsSortBoxFill(opaque[*libps.GsBOXF](), opaque[*libps.GsOT](), opaque[uint16]()) },
	func() { libps.GsSortClear(opaque[uint8](), opaque[uint8](), opaque[uint8](), opaque[*libps.GsOT]()) },
	func() { libps.GsSortFastSprite(opaque[*libps.GsSPRITE](), opaque[*libps.GsOT](), opaque[uint16]()) },
	func() {
		libps.GsSortFixBg16(opaque[*libps.GsBG](), opaque[*uint32](), opaque[*libps.GsOT](), opaque[uint16]())
	},
	func() { libps.GsSortGLine(opaque[*libps.GsGLINE](), opaque[*libps.GsOT](), opaque[uint16]()) },
	func() { libps.GsSortLine(opaque[*libps.GsLINE](), opaque[*libps.GsOT](), opaque[uint16]()) },
	func() {
		libps.GsSortObject4(opaque[*libps.GsDOBJ2](), opaque[*libps.GsOT](), opaque[int32](), opaque[*uint32]())
	},
	func() { libps.GsSortOt(opaque[*libps.GsOT](), opaque[*libps.GsOT]()) },
	func() { libps.GsSortSprite(opaque[*libps.GsSPRITE](), opaque[*libps.GsOT](), opaque[uint16]()) },
	func() { libps.GsSwapDispBuff() },
	func() {
		libps.GteMIMefunc(opaque[*libps.SVECTOR](), opaque[*libps.SVECTOR](), opaque[int32](), opaque[int32]())
	},
	func() { libps.InitHeap(opaque[unsafe.Pointer](), opaque[uint32]()) },
	func() { libps.KanjiFntClose() },
	func() { libps.KanjiFntFlush(opaque[int32]()) },
	func() {
		libps.KanjiFntOpen(opaque[int32](), opaque[int32](), opaque[int32](), opaque[int32](), opaque[int32](), opaque[int32](), opaque[int32](), opaque[int32](), opaque[int32](), opaque[int32]())
	},
	func() { libps.KanjiFntPrint(opaque[*byte]()) },
	func() { libps.Krom2RawAdd(opaque[uint32]()) },
	func() { libps.Krom2RawAdd2(opaque[uint32]()) },
	func() {
		libps.Krom2Tim(opaque[*uint8](), opaque[*uint32](), opaque[int32](), opaque[int32](), opaque[int32](), opaque[int32](), opaque[uint32](), opaque[uint32]())
	},
	func() {
		libps.Krom2Tim2(opaque[*uint8](), opaque[*uint32](), opaque[int32](), opaque[int32](), opaque[int32](), opaque[int32](), opaque[uint32](), opaque[uint32]())
	},
	func() { libps.LoadImage(opaque[*libps.RECT](), opaque[*uint32]()) },
	func() { libps.MoveImage(opaque[*libps.RECT](), opaque[int32](), opaque[int32]()) },
	func() { libps.MulMatrix(opaque[*libps.MATRIX](), opaque[*libps.MATRIX]()) },
	func() { libps.MulMatrix0(opaque[*libps.MATRIX](), opaque[*libps.MATRIX](), opaque[*libps.MATRIX]()) },
	func() { libps.PopMatrix() },
	func() { libps.PushMatrix() },
	func() { libps.PutDispEnv(opaque[*libps.DISPENV]()) },
	func() { libps.PutDrawEnv(opaque[*libps.DRAWENV]()) },
	func() { libps.ResetGraph(opaque[int32]()) },
	func() { libps.ResetRCnt(opaque[uint32]()) },
	func() { libps.RotMatrix(opaque[*libps.SVECTOR](), opaque[*libps.MATRIX]()) },
	func() { libps.RotMatrixX(opaque[int32](), opaque[*libps.MATRIX]()) },
	func() { libps.RotMatrixY(opaque[int32](), opaque[*libps.MATRIX]()) },
	func() { libps.RotMatrixZ(opaque[int32](), opaque[*libps.MATRIX]()) },
	func() { libps.ScaleMatrix(opaque[*libps.MATRIX](), opaque[*libps.VECTOR]()) },
	func() { libps.ScaleMatrixL(opaque[*libps.MATRIX](), opaque[*libps.VECTOR]()) },
	func() { libps.SetBackColor(opaque[int32](), opaque[int32](), opaque[int32]()) },
	func() { libps.SetColorMatrix(opaque[*libps.MATRIX]()) },
	func() { libps.SetDispMask(opaque[int32]()) },
	func() { libps.SetFarColor(opaque[int32](), opaque[int32](), opaque[int32]()) },
	func() { libps.SetGeomOffset(opaque[int32](), opaque[int32]()) },
	func() { libps.SetGeomScreen(opaque[int32]()) },
	func() { libps.SetGraphDebug(opaque[int32]()) },
	func() { libps.SetLightMatrix(opaque[*libps.MATRIX]()) },
	func() { libps.SetVideoMode(opaque[int32]()) },
	func() { libps.SquareRoot0(opaque[int32]()) },
	func() { libps.SsGetMVol(opaque[*libps.SndVolume]()) },
	func() { libps.SsGetMute() },
	func() { libps.SsGetSerialAttr(opaque[uint8](), opaque[uint8]()) },
	func() { libps.SsGetSerialVol(opaque[uint8](), opaque[*libps.SndVolume]()) },
	func() { libps.SsInit() },
	func() { libps.SsIsEos(opaque[int16](), opaque[int16]()) },
	func() { libps.SsPlayBack(opaque[int16](), opaque[int16](), opaque[int16]()) },
	func() { libps.SsSeqClose(opaque[int16]()) },
	func() { libps.SsSeqGetVol(opaque[int16](), opaque[int16](), opaque[*int16](), opaque[*int16]()) },
	func() { libps.SsSeqOpen(opaque[*uint32](), opaque[int16]()) },
	func() { libps.SsSeqPause(opaque[int16]()) },
	func() { libps.SsSeqPlay(opaque[int16](), opaque[uint8](), opaque[int16]()) },
	func() { libps.SsSeqReplay(opaque[int16]()) },
	func() { libps.SsSeqSetAccelerando(opaque[int16](), opaque[int32](), opaque[int32]()) },
	func() { libps.SsSeqSetNext(opaque[int16](), opaque[int16]()) },
	func() { libps.SsSeqSetRitardando(opaque[int16](), opaque[int32](), opaque[int32]()) },
	func() { libps.SsSeqSetVol(opaque[int16](), opaque[int16](), opaque[int16]()) },
	func() { libps.SsSeqStop(opaque[int16]()) },
	func() { libps.SsSetLoop(opaque[int16](), opaque[int16](), opaque[int16]()) },
	func() { libps.SsSetMVol(opaque[int16](), opaque[int16]()) },
	func() { libps.SsSetMute(opaque[uint8]()) },
	func() { libps.SsSetSerialAttr(opaque[uint8](), opaque[uint8](), opaque[uint8]()) },
	func() { libps.SsSetSerialVol(opaque[uint8](), opaque[int16](), opaque[int16]()) },
	func() { libps.SsSetTempo(opaque[int16](), opaque[int16](), opaque[int16]()) },
	func() { libps.SsSetTickMode(opaque[int32]()) },
	func() { libps.SsStart() },
	func() { libps.SsStart2() },
	func() { libps.SsUtAllKeyOff(opaque[int16]()) },
	func() {
		libps.SsUtChangePitch(opaque[int16](), opaque[int16](), opaque[int16](), opaque[int16](), opaque[int16](), opaque[int16](), opaque[int16]())
	},
	func() { libps.SsUtGetReverbType() },
	func() { libps.SsUtGetVVol(opaque[int16](), opaque[*int16](), opaque[*int16]()) },
	func() {
		libps.SsUtKeyOff(opaque[int16](), opaque[int16](), opaque[int16](), opaque[int16](), opaque[int16]())
	},
	func() {
		libps.SsUtKeyOn(opaque[int16](), opaque[int16](), opaque[int16](), opaque[int16](), opaque[int16](), opaque[int16](), opaque[int16]())
	},
	func() {
		libps.SsUtPitchBend(opaque[int16](), opaque[int16](), opaque[int16](), opaque[int16](), opaque[int16]())
	},
	func() { libps.SsUtReverbOff() },
	func() { libps.SsUtReverbOn() },
	func() { libps.SsUtSetReverbDelay(opaque[int16]()) },
	func() { libps.SsUtSetReverbDepth(opaque[int16](), opaque[int16]()) },
	func() { libps.SsUtSetReverbFeedback(opaque[int16]()) },
	func() { libps.SsUtSetReverbType(opaque[int16]()) },
	func() { libps.SsUtSetVVol(opaque[int16](), opaque[int16](), opaque[int16]()) },
	func() { libps.SsVabClose(opaque[int16]()) },
	func() { libps.SsVabTransfer(opaque[*uint8](), opaque[*uint8](), opaque[int16](), opaque[int16]()) },
	func() { libps.StartRCnt(opaque[uint32]()) },
	func() { libps.StoreImage(opaque[*libps.RECT](), opaque[*uint32]()) },
	func() { libps.TestCard(opaque[int32]()) },
	func() { libps.TransMatrix(opaque[*libps.MATRIX](), opaque[*libps.VECTOR]()) },
	func() { libps.TransposeMatrix(opaque[*libps.MATRIX](), opaque[*libps.MATRIX]()) },
	func() { libps.VSync(opaque[int32]()) },
	func() { libps.VSyncCallback(opaque[uintptr]()) },
}

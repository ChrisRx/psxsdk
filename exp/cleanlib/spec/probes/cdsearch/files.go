//go:build yaroze

package main

import "go.chrisrx.dev/psx/disc"

// The files the probe looks up, generated with the assets.
var (
	file0  = disc.Register("ROOT.BIN", "assets/ROOT.BIN")
	file1  = disc.Register("DATA/ONE.BIN", "assets/ONE.BIN")
	file2  = disc.Register("DATA/SUB/TWO.BIN", "assets/TWO.BIN")
	file3  = disc.Register("DATA/EMPTY.BIN", "assets/EMPTY.BIN")
	file4  = disc.Register("DATA/SUB/DEEP/LONGNAME.DAT", "assets/LONGNAME.DAT")
	file5  = disc.Register("MANY/F00.BIN", "assets/F00.BIN")
	file6  = disc.Register("MANY/F01.BIN", "assets/F01.BIN")
	file7  = disc.Register("MANY/F02.BIN", "assets/F02.BIN")
	file8  = disc.Register("MANY/F03.BIN", "assets/F03.BIN")
	file9  = disc.Register("MANY/F04.BIN", "assets/F04.BIN")
	file10 = disc.Register("MANY/F05.BIN", "assets/F05.BIN")
	file11 = disc.Register("MANY/F06.BIN", "assets/F06.BIN")
	file12 = disc.Register("MANY/F07.BIN", "assets/F07.BIN")
	file13 = disc.Register("MANY/F08.BIN", "assets/F08.BIN")
	file14 = disc.Register("MANY/F09.BIN", "assets/F09.BIN")
	file15 = disc.Register("MANY/F10.BIN", "assets/F10.BIN")
	file16 = disc.Register("MANY/F11.BIN", "assets/F11.BIN")
	file17 = disc.Register("MANY/F12.BIN", "assets/F12.BIN")
	file18 = disc.Register("MANY/F13.BIN", "assets/F13.BIN")
	file19 = disc.Register("MANY/F14.BIN", "assets/F14.BIN")
	file20 = disc.Register("MANY/F15.BIN", "assets/F15.BIN")
	file21 = disc.Register("MANY/F16.BIN", "assets/F16.BIN")
	file22 = disc.Register("MANY/F17.BIN", "assets/F17.BIN")
	file23 = disc.Register("MANY/F18.BIN", "assets/F18.BIN")
	file24 = disc.Register("MANY/F19.BIN", "assets/F19.BIN")
	file25 = disc.Register("MANY/F20.BIN", "assets/F20.BIN")
	file26 = disc.Register("MANY/F21.BIN", "assets/F21.BIN")
	file27 = disc.Register("MANY/F22.BIN", "assets/F22.BIN")
	file28 = disc.Register("MANY/F23.BIN", "assets/F23.BIN")
	file29 = disc.Register("MANY/F24.BIN", "assets/F24.BIN")
	file30 = disc.Register("MANY/F25.BIN", "assets/F25.BIN")
	file31 = disc.Register("MANY/F26.BIN", "assets/F26.BIN")
	file32 = disc.Register("MANY/F27.BIN", "assets/F27.BIN")
	file33 = disc.Register("MANY/F28.BIN", "assets/F28.BIN")
	file34 = disc.Register("MANY/F29.BIN", "assets/F29.BIN")
	file35 = disc.Register("MANY/F30.BIN", "assets/F30.BIN")
	file36 = disc.Register("MANY/F31.BIN", "assets/F31.BIN")
	file37 = disc.Register("MANY/F32.BIN", "assets/F32.BIN")
	file38 = disc.Register("MANY/F33.BIN", "assets/F33.BIN")
	file39 = disc.Register("MANY/F34.BIN", "assets/F34.BIN")
	file40 = disc.Register("MANY/F35.BIN", "assets/F35.BIN")
	file41 = disc.Register("MANY/F36.BIN", "assets/F36.BIN")
	file42 = disc.Register("MANY/F37.BIN", "assets/F37.BIN")
	file43 = disc.Register("MANY/F38.BIN", "assets/F38.BIN")
	file44 = disc.Register("MANY/F39.BIN", "assets/F39.BIN")
	file45 = disc.Register("MANY/F40.BIN", "assets/F40.BIN")
	file46 = disc.Register("MANY/F41.BIN", "assets/F41.BIN")
	file47 = disc.Register("MANY/F42.BIN", "assets/F42.BIN")
	file48 = disc.Register("MANY/F43.BIN", "assets/F43.BIN")
	file49 = disc.Register("MANY/F44.BIN", "assets/F44.BIN")
	file50 = disc.Register("MANY/F45.BIN", "assets/F45.BIN")
	file51 = disc.Register("MANY/F46.BIN", "assets/F46.BIN")
	file52 = disc.Register("MANY/F47.BIN", "assets/F47.BIN")
	file53 = disc.Register("MANY/F48.BIN", "assets/F48.BIN")
	file54 = disc.Register("MANY/F49.BIN", "assets/F49.BIN")
	file55 = disc.Register("MANY/F50.BIN", "assets/F50.BIN")
	file56 = disc.Register("MANY/F51.BIN", "assets/F51.BIN")
	file57 = disc.Register("MANY/F52.BIN", "assets/F52.BIN")
	file58 = disc.Register("MANY/F53.BIN", "assets/F53.BIN")
	file59 = disc.Register("MANY/F54.BIN", "assets/F54.BIN")
	file60 = disc.Register("MANY/F55.BIN", "assets/F55.BIN")
	file61 = disc.Register("MANY/F56.BIN", "assets/F56.BIN")
	file62 = disc.Register("MANY/F57.BIN", "assets/F57.BIN")
	file63 = disc.Register("MANY/F58.BIN", "assets/F58.BIN")
	file64 = disc.Register("MANY/F59.BIN", "assets/F59.BIN")
	file65 = disc.Register("MANY/ZZZ/LATE.BIN", "assets/LATE.BIN")
	big0   = disc.Register("BIG/D00/X.BIN", "assets/ROOT.BIN")
	big1   = disc.Register("BIG/D01/X.BIN", "assets/ROOT.BIN")
	big2   = disc.Register("BIG/D02/X.BIN", "assets/ROOT.BIN")
	big3   = disc.Register("BIG/D03/X.BIN", "assets/ROOT.BIN")
	big4   = disc.Register("BIG/D04/X.BIN", "assets/ROOT.BIN")
	big5   = disc.Register("BIG/D05/X.BIN", "assets/ROOT.BIN")
	big6   = disc.Register("BIG/D06/X.BIN", "assets/ROOT.BIN")
	big7   = disc.Register("BIG/D07/X.BIN", "assets/ROOT.BIN")
	big8   = disc.Register("BIG/D08/X.BIN", "assets/ROOT.BIN")
	big9   = disc.Register("BIG/D09/X.BIN", "assets/ROOT.BIN")
	big10  = disc.Register("BIG/D10/X.BIN", "assets/ROOT.BIN")
	big11  = disc.Register("BIG/D11/X.BIN", "assets/ROOT.BIN")
	big12  = disc.Register("BIG/D12/X.BIN", "assets/ROOT.BIN")
	big13  = disc.Register("BIG/D13/X.BIN", "assets/ROOT.BIN")
	big14  = disc.Register("BIG/D14/X.BIN", "assets/ROOT.BIN")
	big15  = disc.Register("BIG/D15/X.BIN", "assets/ROOT.BIN")
	big16  = disc.Register("BIG/D16/X.BIN", "assets/ROOT.BIN")
	big17  = disc.Register("BIG/D17/X.BIN", "assets/ROOT.BIN")
	big18  = disc.Register("BIG/D18/X.BIN", "assets/ROOT.BIN")
	big19  = disc.Register("BIG/D19/X.BIN", "assets/ROOT.BIN")
	big20  = disc.Register("BIG/D20/X.BIN", "assets/ROOT.BIN")
	big21  = disc.Register("BIG/D21/X.BIN", "assets/ROOT.BIN")
	big22  = disc.Register("BIG/D22/X.BIN", "assets/ROOT.BIN")
	big23  = disc.Register("BIG/D23/X.BIN", "assets/ROOT.BIN")
	big24  = disc.Register("BIG/D24/X.BIN", "assets/ROOT.BIN")
	big25  = disc.Register("BIG/D25/X.BIN", "assets/ROOT.BIN")
	big26  = disc.Register("BIG/D26/X.BIN", "assets/ROOT.BIN")
	big27  = disc.Register("BIG/D27/X.BIN", "assets/ROOT.BIN")
	big28  = disc.Register("BIG/D28/X.BIN", "assets/ROOT.BIN")
	big29  = disc.Register("BIG/D29/X.BIN", "assets/ROOT.BIN")
	big30  = disc.Register("BIG/D30/X.BIN", "assets/ROOT.BIN")
	big31  = disc.Register("BIG/D31/X.BIN", "assets/ROOT.BIN")
	big32  = disc.Register("BIG/D32/X.BIN", "assets/ROOT.BIN")
	big33  = disc.Register("BIG/D33/X.BIN", "assets/ROOT.BIN")
	big34  = disc.Register("BIG/D34/X.BIN", "assets/ROOT.BIN")
	big35  = disc.Register("BIG/D35/X.BIN", "assets/ROOT.BIN")
	big36  = disc.Register("BIG/D36/X.BIN", "assets/ROOT.BIN")
	big37  = disc.Register("BIG/D37/X.BIN", "assets/ROOT.BIN")
	big38  = disc.Register("BIG/D38/X.BIN", "assets/ROOT.BIN")
	big39  = disc.Register("BIG/D39/X.BIN", "assets/ROOT.BIN")
	big40  = disc.Register("BIG/D40/X.BIN", "assets/ROOT.BIN")
	big41  = disc.Register("BIG/D41/X.BIN", "assets/ROOT.BIN")
	big42  = disc.Register("BIG/D42/X.BIN", "assets/ROOT.BIN")
	big43  = disc.Register("BIG/D43/X.BIN", "assets/ROOT.BIN")
	big44  = disc.Register("BIG/D44/X.BIN", "assets/ROOT.BIN")
)

var files = [...]*disc.File{
	&file0,
	&file1,
	&file2,
	&file3,
	&file4,
	&file5,
	&file6,
	&file7,
	&file8,
	&file9,
	&file10,
	&file11,
	&file12,
	&file13,
	&file14,
	&file15,
	&file16,
	&file17,
	&file18,
	&file19,
	&file20,
	&file21,
	&file22,
	&file23,
	&file24,
	&file25,
	&file26,
	&file27,
	&file28,
	&file29,
	&file30,
	&file31,
	&file32,
	&file33,
	&file34,
	&file35,
	&file36,
	&file37,
	&file38,
	&file39,
	&file40,
	&file41,
	&file42,
	&file43,
	&file44,
	&file45,
	&file46,
	&file47,
	&file48,
	&file49,
	&file50,
	&file51,
	&file52,
	&file53,
	&file54,
	&file55,
	&file56,
	&file57,
	&file58,
	&file59,
	&file60,
	&file61,
	&file62,
	&file63,
	&file64,
	&file65,
}

var paths = [...]string{
	"ROOT.BIN",
	"DATA/ONE.BIN",
	"DATA/SUB/TWO.BIN",
	"DATA/EMPTY.BIN",
	"DATA/SUB/DEEP/LONGNAME.DAT",
	"MANY/F00.BIN",
	"MANY/F01.BIN",
	"MANY/F02.BIN",
	"MANY/F03.BIN",
	"MANY/F04.BIN",
	"MANY/F05.BIN",
	"MANY/F06.BIN",
	"MANY/F07.BIN",
	"MANY/F08.BIN",
	"MANY/F09.BIN",
	"MANY/F10.BIN",
	"MANY/F11.BIN",
	"MANY/F12.BIN",
	"MANY/F13.BIN",
	"MANY/F14.BIN",
	"MANY/F15.BIN",
	"MANY/F16.BIN",
	"MANY/F17.BIN",
	"MANY/F18.BIN",
	"MANY/F19.BIN",
	"MANY/F20.BIN",
	"MANY/F21.BIN",
	"MANY/F22.BIN",
	"MANY/F23.BIN",
	"MANY/F24.BIN",
	"MANY/F25.BIN",
	"MANY/F26.BIN",
	"MANY/F27.BIN",
	"MANY/F28.BIN",
	"MANY/F29.BIN",
	"MANY/F30.BIN",
	"MANY/F31.BIN",
	"MANY/F32.BIN",
	"MANY/F33.BIN",
	"MANY/F34.BIN",
	"MANY/F35.BIN",
	"MANY/F36.BIN",
	"MANY/F37.BIN",
	"MANY/F38.BIN",
	"MANY/F39.BIN",
	"MANY/F40.BIN",
	"MANY/F41.BIN",
	"MANY/F42.BIN",
	"MANY/F43.BIN",
	"MANY/F44.BIN",
	"MANY/F45.BIN",
	"MANY/F46.BIN",
	"MANY/F47.BIN",
	"MANY/F48.BIN",
	"MANY/F49.BIN",
	"MANY/F50.BIN",
	"MANY/F51.BIN",
	"MANY/F52.BIN",
	"MANY/F53.BIN",
	"MANY/F54.BIN",
	"MANY/F55.BIN",
	"MANY/F56.BIN",
	"MANY/F57.BIN",
	"MANY/F58.BIN",
	"MANY/F59.BIN",
	"MANY/ZZZ/LATE.BIN",
}

var bigs = [...]*disc.File{
	&big0,
	&big1,
	&big2,
	&big3,
	&big4,
	&big5,
	&big6,
	&big7,
	&big8,
	&big9,
	&big10,
	&big11,
	&big12,
	&big13,
	&big14,
	&big15,
	&big16,
	&big17,
	&big18,
	&big19,
	&big20,
	&big21,
	&big22,
	&big23,
	&big24,
	&big25,
	&big26,
	&big27,
	&big28,
	&big29,
	&big30,
	&big31,
	&big32,
	&big33,
	&big34,
	&big35,
	&big36,
	&big37,
	&big38,
	&big39,
	&big40,
	&big41,
	&big42,
	&big43,
	&big44,
}

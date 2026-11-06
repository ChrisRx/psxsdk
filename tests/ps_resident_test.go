package tests

import "testing"

// The programs on the exp/ps API, built for the resident library (task
// build:discs:resident), which have to draw what the same programs do linked
// statically. The API is one for both.

func TestInvadersScreenshotPSResident(t *testing.T) { invadersScreenshot(t, "invaders-ps-resident") }
func TestDinoScreenshotPSResident(t *testing.T)     { dinoScreenshot(t, "dino-ps-resident") }
func TestQuestScreenshotPSResident(t *testing.T)    { questScreenshot(t, "quest-ps-resident") }
func TestQuestChestPSResident(t *testing.T)         { questChest(t, "quest-ps-resident") }
func TestQuestMeadowPSResident(t *testing.T)        { questMeadow(t, "quest-ps-resident") }
func TestQuestLivePSResident(t *testing.T) {
	questLive(t, "quest-ps-resident", "go.chrisrx.dev/psx/exp/ps/scene.live")
}
func TestJumpcubePSResident(t *testing.T)          { jumpcube(t, "jumpcube-ps-resident") }
func TestGalleryScreenshotPSResident(t *testing.T) { galleryScreenshot(t, "gallery-ps-resident") }
func TestSquareScreenshotPSResident(t *testing.T)  { squareScreenshot(t, "square-ps-resident") }
func TestPadsScreenshotPSResident(t *testing.T)    { padsScreenshot(t, "pads-ps-resident") }
func TestCardScreenshotPSResident(t *testing.T)    { cardScreenshot(t, "card-ps-resident") }
func TestCDDAScreenshotPSResident(t *testing.T)    { cddaScreenshot(t, "cdda-ps-resident") }
func TestTimersScreenshotPSResident(t *testing.T)  { timersScreenshot(t, "timers-ps-resident") }
func TestTilesScreenshotPSResident(t *testing.T)   { tilesScreenshot(t, "tiles-ps-resident") }

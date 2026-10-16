package main

import (
	"go.chrisrx.dev/psx/examples/internal/demo"
	"go.chrisrx.dev/psx/yaroze"
)

// Super modes: flying the shuttle, or working the menus.
const (
	moveMode  = 0
	menusMode = 1
)

// Selection modes, the main menu.
const (
	moveShip = iota
	changeLights
	setAmbientLight
	fogging
	viewDistortion
	selectObject
	highestSelectionMode = selectObject
)

// What can be done to the selected object.
const (
	moveObject = iota
	changeSpin
	changeVolume
	changeDisplay
	objectViews
	maxObjectSelectionMode = objectViews
)

// View modes.
const (
	generalDistortion = iota
	moveBothAbsolutely
	moveReference
	moveViewingPoint
	changeSuper
	maxViewMode = changeSuper
)

// Frames a menu waits between changes.
const (
	normalTimeDelay = 7
	longTimeDelay   = 15
)

var (
	superMode           int32
	selectionMode       int32
	objectSelectionMode int32
	selectedObject      int32
	viewMode            int32

	// The highlighted line of the menu, counting from the main mode at 0,
	// how many lines can be highlighted, and how many have been printed
	// so far this frame.
	selectedOption  int32
	numberOfOptions int32
	stringCounter   int32

	framesSinceLastSwitch            uint32
	framesSinceLastSelection         uint32
	framesSinceLastLightChange       uint32
	framesSinceLastLightModeChanged  uint32
	numberFramesSinceViewpointChange uint32
	numberFramesSinceViewModeChange  uint32

	// Which object's coordinate system the view hangs off: -1 for the
	// world and -2 for the shuttle.
	mainViewObjectID int32

	padStatus  uint32
	resetCount int32
)

// The menu text goes out a line at a time through the library's font.
// A line is highlighted by switching the font's colour before it and back
// after it, with ~cRGB.
var (
	lineBuf [128]byte
	line    yaroze.Buffer
)

func say(s string) {
	line.Reset()
	line.WriteString(s).Print()
}

func printInt(label string, n int32, end string) {
	line.Reset()
	line.WriteString(label).WriteInt(int(n)).WriteString(end).Print()
}

func print3(label string, a, b, c int32, end string) {
	line.Reset()
	line.WriteString(label).WriteInt(int(a)).WriteString(" ").WriteInt(int(b)).WriteString(" ").WriteInt(int(c)).WriteString(end).Print()
}

// before and after bracket a line that can be highlighted.
func before() {
	if stringCounter == selectedOption {
		say("~cf0f")
	}
}

func after() {
	if stringCounter == selectedOption {
		say("~cfff")
	}
	stringCounter++
}

// item prints a line that can be highlighted.
func item(s string) {
	before()
	say(s)
	after()
}

func selectionModeString() string {
	switch selectionMode {
	case moveShip:
		return "Moving around\n"
	case changeLights:
		return "Change lights\n"
	case setAmbientLight:
		return "Ambient light\n"
	case fogging:
		return "Change fogging\n"
	case viewDistortion:
		return "Distort viewpoint\n"
	case selectObject:
		return "Select object\n"
	}
	return ""
}

func printMenus() {
	stringCounter = 0
	printInt("~c0ffFrame ", int32(frameNumber), "\n~cfff")
	if superMode == moveMode {
		say("super mode: moving\n")
	} else {
		say("super mode: menus\n")
	}
	before()
	say("Main Mode :-\n")
	say(selectionModeString())
	after()
	printModeInfo()
}

func printModeInfo() {
	switch selectionMode {
	case moveShip:
		before()
		t := &playersShuttle.coord.Coord.T
		print3("Ship pos: ", t[0], t[1], t[2], "\n")
		r := &playersShuttle.rotate
		print3("Ship ang: ", int32(r.Vx), int32(r.Vy), int32(r.Vz), "\n")
		after()
	case changeLights:
		l := &lights[selectedLight]
		before()
		printInt("Selected light: ", selectedLight, "\n")
		after()
		before()
		print3("Direction: ", l.Vx, l.Vy, l.Vz, "\n")
		after()
		before()
		line.Reset()
		line.WriteString("Colour: r ").WriteInt(int(l.R)).WriteString(", g ").WriteInt(int(l.G)).WriteString(", b ").WriteInt(int(l.B)).WriteString("\n").Print()
		after()
	case setAmbientLight:
		say("Ambient light colours:-\n")
		before()
		printInt("Red: ", int32(ambientRed), "\n")
		after()
		before()
		printInt("Green: ", int32(ambientGreen), "\n")
		after()
		before()
		printInt("Blue: ", int32(ambientBlue), "\n")
		after()
	case fogging:
		before()
		printInt("dqa ", int32(fog.Dqa), "\n")
		after()
		before()
		printInt("dqb ", fog.Dqb, "\n")
		after()
		before()
		line.Reset()
		line.WriteString("Fog colours: \nr ").WriteInt(int(fog.Rfc)).WriteString(" g ").WriteInt(int(fog.Gfc)).WriteString(" b ").WriteInt(int(fog.Bfc)).WriteString("\n").Print()
		after()
		before()
		printInt("Lighting mode: ", lightMode, "\n")
		if lightMode == 0 {
			say("I.e. fogging is OFF\n")
		} else {
			say("I.e. fogging is ON\n")
		}
		after()
	case viewDistortion:
		printViewInfo()
	case selectObject:
		before()
		say("submode: ")
		say(objectSelectionModeString())
		after()
		before()
		printInt("Selected object id: ", selectedObject, "\n")
		after()
		writeObjectSelectionInfo()
	}
}

func printViewInfo() {
	switch viewMode {
	case generalDistortion:
		item("view mode :-\nGeneral\n")
		before()
		printInt("rz: ", mainView.Rz, "\n")
		after()
		before()
		printInt("Projection: ", int32(projection), "\n")
		after()
	case moveBothAbsolutely:
		item("view mode :-\nMove both absolutely\n")
		before()
		print3("Reference point :-\n", mainView.Vrx, mainView.Vry, mainView.Vrz, "\n")
		after()
		before()
		print3("Viewing point :-\n ", mainView.Vpx, mainView.Vpy, mainView.Vpz, "\n")
		after()
	case moveReference:
		item("view mode :-\nMove reference\n")
		before()
		print3("Reference point :-\n", mainView.Vrx, mainView.Vry, mainView.Vrz, "\n")
		after()
	case moveViewingPoint:
		item("view mode :-\nMove viewing point\n")
		before()
		print3("Viewing point :-\n ", mainView.Vpx, mainView.Vpy, mainView.Vpz, "\n")
		after()
	case changeSuper:
		item("view mode :-\nChange super\n")
		before()
		switch mainViewObjectID {
		case -2:
			say("View super: player's ship\n")
		case -1:
			say("View super: WORLD\n")
		default:
			printInt("View super: object id ", mainViewObjectID, "\n")
		}
		after()
	}
}

func objectSelectionModeString() string {
	switch objectSelectionMode {
	case moveObject:
		return "Move object\n"
	case changeSpin:
		return "Change spin\n"
	case changeVolume:
		return "Change volume\n"
	case changeDisplay:
		return "Change display\n"
	case objectViews:
		return "Change views\n"
	}
	return ""
}

func boolInt(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

func writeObjectSelectionInfo() {
	o := objects[selectedObject]
	switch objectSelectionMode {
	case moveObject:
		if o.inOrbit {
			say("object is in orbit\n")
			before()
			printInt("orbital radius: ", o.radius, "\n")
			after()
			if o.whichWay == clockwise {
				item("orbit clockwise\n")
			} else {
				item("orbit anticlockwise\n")
			}
		} else {
			say("object is not in orbit\n")
			before()
			print3("position: ", o.position.Vx, o.position.Vy, o.position.Vz, "\n")
			after()
			before()
			printInt("momentum: ", boolInt(o.movementMomentum), "\n")
			after()
		}
	case changeSpin:
		before()
		print3("Angle: ", int32(o.rotate.Vx), int32(o.rotate.Vy), int32(o.rotate.Vz), "\n")
		after()
		before()
		printInt("Rotational momentum: ", boolInt(o.rotationMomentum), "\n")
		after()
	case changeVolume:
		before()
		printInt("scale X: ", o.scaleX, "\n")
		after()
		before()
		printInt("scale Y: ", o.scaleY, "\n")
		after()
		before()
		printInt("scale Z: ", o.scaleZ, "\n")
		after()
	case changeDisplay:
		a := o.handler.Attribute
		if a&gsFOG != 0 {
			item("fog only\n")
		} else {
			item("normal lighting\n")
		}
		if a&yaroze.GsLOFF != 0 {
			item("light calculations OFF\n")
		} else {
			item("light calculations ON\n")
		}
		before()
		printInt("semi-trans type: ", int32(a>>28&3), "\n")
		after()
		if a&yaroze.GsALON != 0 {
			item("semi-transparency ON\n")
		} else {
			item("semi-transparency OFF\n")
		}
		if a&yaroze.GsDOFF != 0 {
			item("display OFF\n")
		} else {
			item("display ON\n")
		}
	case objectViews:
		writeObjectViewInfo()
	}
}

func writeObjectViewInfo() {
	if !specialViewFlag {
		item("no special view\n")
		return
	}
	switch specialViewType {
	case externalView:
		item("external view\n")
		item([...]string{"From left\n", "From right\n", "From below\n", "From above\n", "From back\n", "From front\n"}[whichExternalView])
		before()
		printInt("external distance: ", externalViewDistance, "\n")
		after()
	case circlingView:
		item("Circling view\n")
		item([...]string{"circle in x-y plane\n", "circle in x-z plane\n", "circle in y-z plane\n"}[whichCircularView])
		if whichWayRoundCirclingView == clockwise {
			item("orbiting clockwise\n")
		} else {
			item("orbiting anticlockwise\n")
		}
		before()
		printInt("orbit radius: ", circleViewRadius, "\n")
		after()
		printInt("orbit angle: ", circleViewAngle, "\n")
		before()
		printInt("angular speed: ", circleViewAngleIncrement, "\n")
		after()
	case flybyView:
		item("Fly-by view\n")
		if typeOfFlybyView == circular {
			say("object is in orbit\n")
			return
		}
		say("object is not in orbit\n")
		before()
		printInt("path distance: ", pathDistance, "\n")
		after()
		before()
		printInt("path angle: ", pathAngle, "\n")
		after()
		before()
		printInt("cycle time: ", flybyCycleTime, "\n")
		after()
	case directorsView:
		item("Director's cut\n")
	case localView:
		item("local view\n")
		item([...]string{"Looking right\n", "Looking left\n", "Looking down\n", "Looking up\n", "Looking backward\n", "Looking forwards\n"}[whichLocalView])
		before()
		printInt("local view distance: ", localViewDistance, "\n")
		after()
	}
}

// pressed reports whether a button is held.
func pressed(b uint32) bool { return padStatus&b != 0 }

// speed is the fast amount with Select held, and the slow one without.
func speed(slow, fast int32) int32 {
	if pressed(demo.PADselect) {
		return fast
	}
	return slow
}

// dealWithController acts on the pad, read at the end of the frame before,
// and reports false when the program is to quit.
func dealWithController() bool {
	if superMode == menusMode {
		switch selectionMode {
		case moveShip:
			handleShipMoving()
		case changeLights:
			handleLightControls()
		case setAmbientLight:
			handleAmbientControls()
		case fogging:
			handleFoggingControls()
		case viewDistortion:
			handleViewControls()
		case selectObject:
			handleObjectSelection()
		}
	} else {
		handleShipMoving()
	}

	// Up and down the menu.
	if superMode == menusMode && framesSinceLastSwitch > normalTimeDelay {
		if pressed(demo.PADRup) {
			selectedOption = (selectedOption + numberOfOptions - 1) % numberOfOptions
			framesSinceLastSwitch = 0
		}
	}
	if superMode == menusMode && pressed(demo.PADRdown) && framesSinceLastSwitch > normalTimeDelay {
		selectedOption = (selectedOption + 1) % numberOfOptions
		framesSinceLastSwitch = 0
	}

	// Round the main modes, with the first line highlighted.
	if selectedOption == 0 && superMode == menusMode {
		for _, way := range [2]struct {
			button uint32
			step   int32
		}{{demo.PADRright, 1}, {demo.PADRleft, highestSelectionMode}} {
			if !pressed(way.button) || framesSinceLastSwitch <= normalTimeDelay {
				continue
			}
			if selectionMode == selectObject {
				objects[selectedObject].unhighlight()
			}
			selectionMode = (selectionMode + way.step) % (highestSelectionMode + 1)
			if selectionMode == selectObject {
				objects[selectedObject].highlight()
			}
			resetSelectionData()
			framesSinceLastSwitch = 0
		}
	}

	// L2 and R2 held a while: start again.
	if pressed(demo.PADL2) && pressed(demo.PADR2) {
		resetCount++
		if resetCount > longTimeDelay {
			resetAll()
			resetCount = 0
		}
	} else {
		resetCount = 0
	}

	if pressed(demo.PADselect) && pressed(demo.PADstart) {
		return false
	}

	// L2 and R2: step slowly.
	if pressed(demo.PADL2) && pressed(demo.PADR2) {
		now := yaroze.VSync(-1)
		for yaroze.VSync(-1) < now+25 {
		}
	}

	// Start: between flying and the menus.
	if pressed(demo.PADstart) && framesSinceLastSwitch > normalTimeDelay {
		superMode = 1 - superMode
		framesSinceLastSwitch = 0
	}
	return true
}

func resetSelectionData() {
	numberOfOptions = findNumberOfOptions()
	selectedOption = 0
}

// findNumberOfOptions is how many lines of the menu as it stands can be
// highlighted.
func findNumberOfOptions() int32 {
	switch selectionMode {
	case moveShip:
		return 2
	case changeLights, setAmbientLight:
		return 4
	case fogging:
		return 5
	case viewDistortion:
		if viewMode == generalDistortion || viewMode == moveBothAbsolutely {
			return 4
		}
		return 3
	case selectObject:
		switch objectSelectionMode {
		case moveObject, changeSpin:
			return 5
		case changeVolume:
			return 6
		case changeDisplay:
			return 8
		case objectViews:
			if !specialViewFlag {
				return 4
			}
			switch specialViewType {
			case externalView:
				return 6
			case circlingView:
				return 8
			case flybyView:
				if objects[selectedObject].inOrbit {
					return 4
				}
				return 7
			case directorsView:
				return 4
			case localView:
				return 7
			}
		}
	}
	return 1
}

// handleShipMoving flies the shuttle: the directional buttons turn it, L1
// and L2 move it forward and back.
func handleShipMoving() {
	if superMode == menusMode {
		return
	}
	s := &playersShuttle
	rot := rotationSpeed * speed(1, 10)
	move := movementSpeed * speed(1, 10)
	moved := false
	if pressed(demo.PADLleft) {
		s.twist.Vz -= rot
		moved = true
	}
	if pressed(demo.PADLright) {
		s.twist.Vz += rot
		moved = true
	}
	if pressed(demo.PADLup) {
		s.twist.Vx -= rot
		moved = true
	}
	if pressed(demo.PADLdown) {
		s.twist.Vx += rot
		moved = true
	}
	if pressed(demo.PADL1) {
		s.velocity.Vz -= move
		moved = true
	}
	if pressed(demo.PADL2) {
		s.velocity.Vz += move
		moved = true
	}
	if moved && mainView.Super == &s.coord {
		viewChanged = true
	}
}

// The directional buttons, and L1 and L2, in the order the original tests
// them: one each way along x, y and z.
var axisButtons = [6]uint32{demo.PADLleft, demo.PADLright, demo.PADLup, demo.PADLdown, demo.PADL1, demo.PADL2}

// adjust adds by[i] to *v[i/2] for each of axisButtons held, and reports
// whether any was.
func adjust[T int32 | uint8](x, y, z *T, by [6]T) bool {
	changed := false
	v := [3]*T{x, y, z}
	for i, b := range axisButtons {
		if pressed(b) {
			*v[i/2] += by[i]
			changed = true
		}
	}
	return changed
}

func handleLightControls() {
	change := speed(1, 10)
	l := &lights[selectedLight]
	switch selectedOption {
	case 1: // which light
		if pressed(demo.PADLleft) && framesSinceLastLightChange > normalTimeDelay {
			selectedLight = (selectedLight + 2) % 3
			framesSinceLastLightChange = 0
		}
		if pressed(demo.PADLright) && framesSinceLastLightChange > normalTimeDelay {
			selectedLight = (selectedLight + 1) % 3
			framesSinceLastLightChange = 0
		}
	case 2: // its direction
		d := 10 * change
		if adjust(&l.Vx, &l.Vy, &l.Vz, [6]int32{-d, d, d, -d, d, -d}) {
			lightsChanged = true
		}
	case 3: // its colour: left and right red, up and down green, L1 and L2 blue
		c := uint8(change)
		if adjust(&l.R, &l.G, &l.B, [6]uint8{-c, c, c, -c, c, -c}) {
			lightsChanged = true
		}
	}
}

func handleAmbientControls() {
	change := uint16(speed(200, 2000))
	var c *uint16
	switch selectedOption {
	case 1:
		c = &ambientRed
	case 2:
		c = &ambientGreen
	case 3:
		c = &ambientBlue
	default:
		return
	}
	if pressed(demo.PADLup) {
		*c += change
		ambientChanged = true
	}
	if pressed(demo.PADLdown) {
		*c -= change
		ambientChanged = true
	}
}

func handleFoggingControls() {
	change := speed(2, 20)
	switch selectedOption {
	case 1:
		if pressed(demo.PADLup) {
			fog.Dqa += int16(change)
			fogChanged = true
		}
		if pressed(demo.PADLdown) {
			fog.Dqa -= int16(change)
			fogChanged = true
		}
	case 2:
		if pressed(demo.PADLup) {
			fog.Dqb += change
			fogChanged = true
		}
		if pressed(demo.PADLdown) {
			fog.Dqb -= change
			fogChanged = true
		}
	case 3: // the colour things fade into
		c := uint8(change)
		if adjust(&fog.Rfc, &fog.Gfc, &fog.Bfc, [6]uint8{-c, c, c, -c, c, -c}) {
			fogChanged = true
		}
		if pressed(demo.PADR1) { // black
			fog.Rfc, fog.Gfc, fog.Bfc = 0, 0, 0
			fogChanged = true
		}
		if pressed(demo.PADR2) { // white
			fog.Rfc, fog.Gfc, fog.Bfc = 255, 255, 255
			fogChanged = true
		}
	case 4: // fog on or off
		if pressed(demo.PADLup) && framesSinceLastLightModeChanged > normalTimeDelay {
			lightMode = 1 - lightMode
			framesSinceLastLightModeChanged = 0
			lightModeChanged = true
		}
	}
}

func handleViewControls() {
	change := speed(1, 10)
	if selectedOption == 1 && numberFramesSinceViewModeChange > normalTimeDelay {
		step := int32(0)
		if pressed(demo.PADLright) {
			step = 1
		} else if pressed(demo.PADLleft) {
			step = maxViewMode
		}
		if step != 0 {
			viewMode = (viewMode + step) % (maxViewMode + 1)
			numberFramesSinceViewModeChange = 0
			resetSelectionData()
			selectedOption = 1
			return
		}
	}

	switch viewMode {
	case generalDistortion:
		if selectedOption == 2 {
			if pressed(demo.PADLup) {
				mainView.Rz += 10 * change
				viewChanged = true
			}
			if pressed(demo.PADLdown) {
				mainView.Rz -= 10 * change
				viewChanged = true
			}
		}
		if selectedOption == 3 {
			if pressed(demo.PADLup) {
				projection += uint32(10 * change)
				projectionChanged = true
			}
			if pressed(demo.PADLdown) {
				projection -= uint32(10 * change)
				projectionChanged = true
			}
		}
	case moveBothAbsolutely:
		// Left moves along +x here, against the other menus.
		d := 20 * change
		by := [6]int32{d, -d, d, -d, d, -d}
		if selectedOption == 2 && adjust(&mainView.Vrx, &mainView.Vry, &mainView.Vrz, by) {
			viewChanged = true
		}
		if selectedOption == 3 && adjust(&mainView.Vpx, &mainView.Vpy, &mainView.Vpz, by) {
			viewChanged = true
		}
	case changeSuper:
		if selectedOption != 2 || numberFramesSinceViewpointChange <= normalTimeDelay {
			return
		}
		switch {
		case pressed(demo.PADLup):
			if mainViewObjectID >= 0 {
				mainViewObjectID = findNextHighestObjectID(mainViewObjectID)
			} else {
				mainViewObjectID = findNextHighestObjectID(-1)
			}
			mainView.Super = &objects[mainViewObjectID].coord
		case pressed(demo.PADLdown):
			if mainViewObjectID >= 0 {
				mainViewObjectID = findNextLowestObjectID(mainViewObjectID)
			} else {
				mainViewObjectID = findNextLowestObjectID(maxObjects)
			}
			mainView.Super = &objects[mainViewObjectID].coord
		case pressed(demo.PADLleft):
			mainViewObjectID = -1
			mainView.Super = nil
		case pressed(demo.PADLright):
			mainViewObjectID = -2
			mainView.Super = &playersShuttle.coord
		default:
			return
		}
		viewChanged = true
		numberFramesSinceViewpointChange = 0
	}
}

func handleObjectSelection() {
	if selectedOption == 1 && framesSinceLastSelection > normalTimeDelay {
		step := int32(0)
		if pressed(demo.PADLright) {
			step = 1
		} else if pressed(demo.PADLleft) {
			step = maxObjectSelectionMode
		}
		if step != 0 {
			objectSelectionMode = (objectSelectionMode + step) % (maxObjectSelectionMode + 1)
			framesSinceLastSelection = 0
			resetSelectionData()
			selectedOption = 1
			return
		}
	}
	if selectedOption == 2 && framesSinceLastSelection > normalTimeDelay {
		if pressed(demo.PADLup) || pressed(demo.PADLdown) {
			objects[selectedObject].unhighlight()
			if pressed(demo.PADLup) {
				selectedObject = findNextHighestObjectID(selectedObject)
			} else {
				selectedObject = findNextLowestObjectID(selectedObject)
			}
			objects[selectedObject].highlight()
			framesSinceLastSelection = 0
			return
		}
	}

	switch objectSelectionMode {
	case moveObject:
		handleMovingAnObject()
	case changeSpin:
		handleChangingAnObjectsSpin()
	case changeVolume:
		handleChangingAnObjectsVolume()
	case changeDisplay:
		handleChangingAnObjectsDisplay()
	case objectViews:
		if selectedOption == 3 && framesSinceLastSelection > normalTimeDelay {
			step := int32(0)
			if pressed(demo.PADLleft) {
				step = 1
			} else if pressed(demo.PADLright) {
				step = maxSpecialView
			}
			if step != 0 {
				if !specialViewFlag {
					specialViewFlag = true
					specialViewType = externalView
				} else {
					specialViewType = (specialViewType + step) % (maxSpecialView + 1)
				}
				framesSinceLastSelection = 0
				resetSelectionData()
				selectedOption = 3
				setUpSpecialView()
				return
			}
			if pressed(demo.PADLdown) {
				specialViewFlag = false
				specialViewType = none
				mainView.Super = nil
				viewChanged = true
				framesSinceLastSelection = 0
				resetSelectionData()
				selectedOption = 3
				return
			}
		}
		if specialViewFlag {
			switch specialViewType {
			case externalView:
				handleExternalView()
			case circlingView:
				handleCirclingView()
			case flybyView:
				handleFlybyView()
			case localView:
				handleLocalView()
			}
		}
	}
}

func handleMovingAnObject() {
	o := objects[selectedObject]
	mult := speed(1, 10)
	if o.inOrbit {
		if selectedOption == 3 {
			if pressed(demo.PADLup) {
				o.radius += mult
			}
			if pressed(demo.PADLdown) {
				o.radius -= mult
			}
		}
		if selectedOption == 4 && pressed(demo.PADLup) {
			if framesSinceLastSelection > normalTimeDelay {
				if o.whichWay == clockwise {
					o.whichWay = anticlockwise
				} else {
					o.whichWay = clockwise
				}
			}
			// As the original has it, the wait starts again every frame
			// the button is held, so holding it toggles once.
			framesSinceLastSelection = 0
		}
		return
	}
	if selectedOption == 3 {
		shift := uint(0)
		if o.movementMomentum {
			shift = 4
		}
		b := movementSpeed * mult >> shift
		adjust(&o.velocity.Vx, &o.velocity.Vy, &o.velocity.Vz, [6]int32{-b, b, -b, b, -b, b})
	}
	if selectedOption == 4 && framesSinceLastSelection > normalTimeDelay {
		if pressed(demo.PADLleft) {
			o.movementMomentum = false
			framesSinceLastSelection = 0
		}
		if pressed(demo.PADLright) {
			o.movementMomentum = true
			framesSinceLastSelection = 0
		}
	}
}

func handleChangingAnObjectsSpin() {
	o := objects[selectedObject]
	by := rotationSpeed * speed(1, 10)
	if selectedOption == 3 {
		if pressed(demo.PADLleft) {
			o.twist.Vy -= by
		}
		if pressed(demo.PADLright) {
			o.twist.Vy += by
		}
		if pressed(demo.PADLup) {
			o.twist.Vx -= by
		}
		if pressed(demo.PADLdown) {
			o.twist.Vx += by
		}
		if pressed(demo.PADL1) {
			o.twist.Vz -= by
		}
		if pressed(demo.PADL2) {
			o.twist.Vz += by
		}
	}
	if selectedOption == 4 && framesSinceLastSelection > normalTimeDelay {
		if pressed(demo.PADLleft) {
			o.rotationMomentum = false
			framesSinceLastSelection = 0
		}
		if pressed(demo.PADLright) {
			o.rotationMomentum = true
			framesSinceLastSelection = 0
		}
	}
}

func handleChangingAnObjectsVolume() {
	o := objects[selectedObject]
	by := speed(10, 100)
	var s *int32
	switch selectedOption {
	case 3:
		s = &o.scaleX
	case 4:
		s = &o.scaleY
	case 5:
		s = &o.scaleZ
	default:
		return
	}
	if pressed(demo.PADLup) {
		*s += by
	}
	if pressed(demo.PADLdown) {
		*s -= by
	}
}

// gsFOG is the attribute bit that leaves only the fog.
const gsFOG = 1 << 3

// handleChangingAnObjectsDisplay toggles the selected object's attribute
// bits with Up: fog only, lighting (which selection uses to highlight, so
// it shows no change), the semi-transparency type, semi-transparency and
// display.
func handleChangingAnObjectsDisplay() {
	if !pressed(demo.PADLup) || framesSinceLastSelection <= normalTimeDelay {
		return
	}
	a := &objects[selectedObject].handler.Attribute
	switch selectedOption {
	case 3:
		*a ^= gsFOG
	case 4:
		*a ^= yaroze.GsLOFF
	case 5:
		t := (*a>>28&3 + 1) % 4
		*a = *a&^(3<<28) | t<<28
	case 6:
		*a ^= yaroze.GsALON
	case 7:
		*a ^= yaroze.GsDOFF
	default:
		return
	}
	framesSinceLastSelection = 0
}

func handleExternalView() {
	mult := speed(1, 10)
	if selectedOption == 4 && framesSinceLastSelection > normalTimeDelay {
		if pressed(demo.PADLup) {
			whichExternalView = (whichExternalView + 5) % 6
			framesSinceLastSelection = 0
		}
		if pressed(demo.PADLdown) && framesSinceLastSelection > normalTimeDelay {
			whichExternalView = (whichExternalView + 1) % 6
			framesSinceLastSelection = 0
		}
	}
	if selectedOption == 5 {
		if pressed(demo.PADLup) {
			externalViewDistance += 50 * mult
		}
		if pressed(demo.PADLdown) {
			externalViewDistance = max(externalViewDistance-50*mult, 100)
		}
	}
}

func handleCirclingView() {
	mult := speed(1, 10)
	switch selectedOption {
	case 4:
		if pressed(demo.PADLup) && framesSinceLastSelection > normalTimeDelay {
			whichCircularView = (whichCircularView + 1) % 3
			framesSinceLastSelection = 0
		}
	case 5:
		if pressed(demo.PADLup) && framesSinceLastSelection > normalTimeDelay {
			if whichWayRoundCirclingView == clockwise {
				whichWayRoundCirclingView = anticlockwise
			} else {
				whichWayRoundCirclingView = clockwise
			}
			framesSinceLastSelection = 0
		}
	case 6:
		if pressed(demo.PADLup) {
			circleViewRadius += 20 * mult
		}
		if pressed(demo.PADLdown) {
			circleViewRadius = max(circleViewRadius-20*mult, 200)
		}
	case 7:
		if pressed(demo.PADLup) {
			circleViewAngleIncrement += mult
		}
		if pressed(demo.PADLdown) {
			circleViewAngleIncrement = max(circleViewAngleIncrement-mult, 1)
		}
	}
}

func handleFlybyView() {
	if typeOfFlybyView == circular {
		return
	}
	mult := speed(1, 10)
	switch selectedOption {
	case 4:
		if pressed(demo.PADLup) {
			pathDistance += 20 * mult
		}
		if pressed(demo.PADLdown) {
			pathDistance = max(pathDistance-20*mult, 10)
		}
	case 5:
		if pressed(demo.PADLup) {
			pathAngle += 4 * mult
		}
		if pressed(demo.PADLdown) {
			pathAngle -= 4 * mult
		}
	case 6:
		if pressed(demo.PADLup) {
			flybyCycleTime++
		}
		if pressed(demo.PADLdown) {
			flybyCycleTime--
		}
	}
}

func handleLocalView() {
	mult := speed(1, 10)
	if selectedOption == 4 && framesSinceLastSelection > normalTimeDelay {
		if pressed(demo.PADLup) {
			whichLocalView = (whichLocalView + 5) % 6
			framesSinceLastSelection = 0
		}
		if pressed(demo.PADLdown) && framesSinceLastSelection > normalTimeDelay {
			whichLocalView = (whichLocalView + 1) % 6
			framesSinceLastSelection = 0
		}
	}
	if selectedOption == 5 {
		if pressed(demo.PADLup) {
			localViewDistance += 50 * mult
		}
		if pressed(demo.PADLdown) {
			localViewDistance = max(localViewDistance-50*mult, 100)
		}
	}
}

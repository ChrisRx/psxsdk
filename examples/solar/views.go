package main

import (
	"go.chrisrx.dev/psx/examples/internal/lpge"
	"go.chrisrx.dev/psx/yaroze"
)

// Special views, of the selected object.
const (
	externalView = iota
	circlingView
	flybyView
	directorsView
	localView
	maxSpecialView = localView
)

// The ends of the axes, for the external and local views.
const (
	plusX = iota
	minusX
	plusY
	minusY
	plusZ
	minusZ
)

// Planes for the circling view.
const (
	xyPlane = iota
	xzPlane
	yzPlane
)

// How an object moves, for the fly-by view.
const (
	circular = iota
	linear
)

var (
	specialViewFlag bool
	specialViewType int32

	whichExternalView    int32
	externalViewDistance int32

	whichCircularView         int32
	whichWayRoundCirclingView int32
	circleViewRadius          int32
	circleViewAngle           int32
	circleViewAngleIncrement  int32

	typeOfFlybyView        int32
	pathDistance           int32
	pathAngle              int32
	flybyCycleTime         int32
	numberFlybyFramesSoFar int32

	whichLocalView    int32
	localViewDistance int32

	// The director's cut runs through a local, a circling and a fly-by
	// view in turn, with settings of its own.
	sequencePosition                  int32
	singleViewCycleTime               int32
	directorFrameCounter              int32
	directorLocalView                 int32
	directorLocalViewDistance         int32
	directorWhichCircularView         int32
	directorWhichWayRoundCirclingView int32
	directorCircleViewRadius          int32
	directorCircleViewAngleIncrement  int32
	directorPathDistance              int32
	directorPathAngle                 int32
	directorSequence                  = [3]int32{localView, circlingView, flybyView}
)

func setUpSpecialView() {
	switch specialViewType {
	case externalView, circlingView, localView:
		// Their settings stand as they are.
	case flybyView:
		numberFlybyFramesSoFar = 0
	case directorsView:
		setUpDirectorsView()
	}
}

func setUpDirectorsView() {
	sequencePosition = rand() % 3
	singleViewCycleTime = 30
	directorFrameCounter = 0
	directorLocalView = minusZ // looking forward
	directorWhichCircularView = xzPlane
	directorWhichWayRoundCirclingView = anticlockwise
	directorCircleViewRadius = 5000
	directorCircleViewAngleIncrement = 24
	directorPathDistance = 2000
	directorPathAngle = 0 // from the side
}

func dealWithSpecialView() {
	switch specialViewType {
	case externalView:
		dealWithExternalView()
	case circlingView:
		dealWithCirclingView()
	case flybyView:
		dealWithFlybyView()
	case directorsView:
		dealWithDirectorsView()
	case localView:
		dealWithLocalView()
	}
}

// axisEnd is the point distance along one end of an axis.
func axisEnd(which, distance int32) yaroze.VECTOR {
	var d yaroze.VECTOR
	switch which {
	case plusX:
		d.Vx = distance
	case minusX:
		d.Vx = -distance
	case plusY:
		d.Vy = distance
	case minusY:
		d.Vy = -distance
	case plusZ:
		d.Vz = distance
	case minusZ:
		d.Vz = -distance
	}
	return d
}

// viewSelected points the view's coordinate system at the selected object.
func viewSelected() {
	mainView.Super = &objects[selectedObject].coord
}

// dealWithExternalView looks at the selected object from along one of its
// axes.
func dealWithExternalView() {
	d := axisEnd(whichExternalView, externalViewDistance)
	viewSelected()
	mainView.Vrx, mainView.Vry, mainView.Vrz = 0, 0, 0
	mainView.Vpx, mainView.Vpy, mainView.Vpz = d.Vx, d.Vy, d.Vz
	viewChanged = true
}

// dealWithCirclingView looks at the selected object from a point going
// round it in one of its planes.
func dealWithCirclingView() {
	var d yaroze.VECTOR
	c := circleViewRadius * lpge.Rcos(circleViewAngle) >> 12
	s := circleViewRadius * lpge.Rsin(circleViewAngle) >> 12
	switch whichCircularView {
	case xyPlane:
		d.Vx, d.Vy = c, s
	case xzPlane:
		d.Vx, d.Vz = c, s
	case yzPlane:
		d.Vz, d.Vy = c, s
	}
	switch whichWayRoundCirclingView {
	case clockwise:
		circleViewAngle -= circleViewAngleIncrement
	case anticlockwise:
		circleViewAngle += circleViewAngleIncrement
	}
	viewSelected()
	mainView.Vrx, mainView.Vry, mainView.Vrz = 0, 0, 0
	mainView.Vpx, mainView.Vpy, mainView.Vpz = d.Vx, d.Vy, d.Vz
	viewChanged = true
}

// dealWithFlybyView watches the selected object go by from a path beside
// its own, going round each cycle. An object in orbit is watched from
// wherever the view was; the original has given that case up.
func dealWithFlybyView() {
	o := objects[selectedObject]
	viewSelected()
	typeOfFlybyView = linear
	if o.inOrbit {
		typeOfFlybyView = circular
	}
	if typeOfFlybyView == linear {
		speed := findObjectSpeed(o)
		mainView.Vrx, mainView.Vry, mainView.Vrz = 0, 0, 0
		mainView.Vpx = pathDistance * lpge.Rcos(pathAngle) >> 12
		mainView.Vpy = pathDistance * lpge.Rsin(pathAngle) >> 12
		mainView.Vpz = -(speed*flybyCycleTime)/2 + speed*numberFlybyFramesSoFar
	}
	numberFlybyFramesSoFar++
	if numberFlybyFramesSoFar >= flybyCycleTime {
		numberFlybyFramesSoFar = 0
	}
	viewChanged = true
}

// dealWithDirectorsView runs the director's sequence a second a view,
// imposing its own settings on each and putting the viewer's back after.
func dealWithDirectorsView() {
	switch directorSequence[sequencePosition] {
	case localView:
		oldWhich, oldDistance := whichLocalView, localViewDistance
		whichLocalView, localViewDistance = directorLocalView, directorLocalViewDistance
		dealWithLocalView()
		whichLocalView, localViewDistance = oldWhich, oldDistance
	case circlingView:
		oldPlane, oldWay := whichCircularView, whichWayRoundCirclingView
		oldRadius, oldIncrement := circleViewRadius, circleViewAngleIncrement
		whichCircularView, whichWayRoundCirclingView = directorWhichCircularView, directorWhichWayRoundCirclingView
		circleViewRadius, circleViewAngleIncrement = directorCircleViewRadius, directorCircleViewAngleIncrement
		dealWithCirclingView()
		whichCircularView, whichWayRoundCirclingView = oldPlane, oldWay
		circleViewRadius, circleViewAngleIncrement = oldRadius, oldIncrement
	case flybyView:
		oldDistance, oldAngle, oldCycle := pathDistance, pathAngle, flybyCycleTime
		pathDistance, pathAngle, flybyCycleTime = directorPathDistance, directorPathAngle, singleViewCycleTime
		dealWithFlybyView()
		pathDistance, pathAngle, flybyCycleTime = oldDistance, oldAngle, oldCycle
	}

	directorFrameCounter++
	if directorFrameCounter%singleViewCycleTime == 0 {
		sequencePosition = (sequencePosition + 1) % 3
		numberFlybyFramesSoFar = 0
		// Circle the other way and fly by from the other side next time.
		if directorWhichWayRoundCirclingView == clockwise {
			directorWhichWayRoundCirclingView = anticlockwise
		} else {
			directorWhichWayRoundCirclingView = clockwise
		}
		if directorPathAngle == 0 {
			directorPathAngle = 2048
		} else {
			directorPathAngle = 0
		}
		directorFrameCounter = 0
	}
}

// dealWithLocalView looks out from the selected object along one of its
// axes, from just off its middle.
func dealWithLocalView() {
	d := axisEnd(whichLocalView, localViewDistance)
	viewSelected()
	mainView.Vpx, mainView.Vpy, mainView.Vpz = d.Vx/20, d.Vy/20, d.Vz/20
	mainView.Vrx, mainView.Vry, mainView.Vrz = d.Vx, d.Vy, d.Vz
	viewChanged = true
}

package yaroze

// The GTE, the console's geometry coprocessor, for a program that
// transforms its own vertices and builds its own packets rather than
// leaving every model to Object.Draw: the commands a polygon takes, in
// their order. The matrix they transform through is the camera's, which
// UseCamera loads, or an object's, which Object.Transform loads.
//
//	yaroze.UseCamera()
//	if yaroze.Project3(&a, &b, &c, &sxy) && yaroze.ClipArea() > 0 {
//		depth := yaroze.AverageDepth3()
//		p := frame.Packet[yaroze.PolyF3](depth)
//		p.XY0, p.XY1, p.XY2 = sxy[0], sxy[1], sxy[2]
//	}

// UseCamera loads the world-to-screen matrix of the camera last set,
// so that the projections below take points in the world.
func UseCamera() { useCamera() }

// Project3 projects three vertices to the screen, into sxy, and reports
// whether every result was in range. Their depths stay in the
// coprocessor for AverageDepth3, and their points for ClipArea.
func Project3(a, b, c *Vertex, sxy *[3]XY) bool { return project3(a, b, c, sxy) }

// Project projects one vertex to the screen, into sxy, and reports
// whether it was in range. After Project3 it makes a quad's fourth
// corner, which AverageDepth4 then takes in.
func Project(v *Vertex, sxy *XY) bool { return project(v, sxy) }

// ClipArea is the signed area the last three screen points enclose:
// positive when they turn clockwise on the screen, a polygon facing the
// viewer, and zero or negative for one to leave out.
func ClipArea() int { return clipArea() }

// AverageDepth3 and AverageDepth4 are the average of the last three or
// four depths projected, scaled to the frame's depths as Object.Draw
// sorts models: the depth to draw a packet at.
func AverageDepth3() int { return averageDepth3() }
func AverageDepth4() int { return averageDepth4() }

// Clut is the texture's palette as a packet takes it.
func (t Texture) Clut() uint16 { return uint16(t.CY)<<6 | uint16(t.CX)>>4 }

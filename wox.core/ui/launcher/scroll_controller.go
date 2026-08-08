package launcher

// scrollController owns the geometry and offset shared by ordinary scroll surfaces.
type scrollController struct {
	offset   float32
	viewport float32
	content  float32
}

func (c *scrollController) reset() {
	*c = scrollController{}
}

// withGeometry returns a clamped controller without mutating a render snapshot.
func (c scrollController) withGeometry(viewport, content float32) scrollController {
	c.setGeometry(viewport, content)
	return c
}

// setGeometry records the latest extents and keeps the offset within the scrollable range.
func (c *scrollController) setGeometry(viewport, content float32) {
	c.viewport = max(float32(0), viewport)
	c.content = max(float32(0), content)
	c.offset = min(max(float32(0), c.offset), c.maxOffset())
}

func (c *scrollController) scrollBy(delta float32) {
	c.offset = min(max(float32(0), c.offset+delta), c.maxOffset())
}

// ensureVisible minimally moves the viewport only when the requested range crosses an edge.
func (c *scrollController) ensureVisible(top, bottom float32) {
	if c.viewport <= 0 || c.content <= c.viewport {
		c.offset = 0
		return
	}
	if top < c.offset {
		c.offset = top
	} else if bottom > c.offset+c.viewport {
		c.offset = bottom - c.viewport
	}
	c.offset = min(max(float32(0), c.offset), c.maxOffset())
}

func (c scrollController) maxOffset() float32 {
	return max(float32(0), c.content-c.viewport)
}

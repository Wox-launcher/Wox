package window

// TransformScene maps a standalone raster/vector scene into its viewport after recording.
// It is used by the screenshot editor, whose document coordinates must survive display changes.
func (d *DisplayList) TransformScene(scale float32, offset Point) {
	transform := func(rect Rect) Rect {
		return Rect{X: offset.X + rect.X*scale, Y: offset.Y + rect.Y*scale, Width: rect.Width * scale, Height: rect.Height * scale}
	}
	for index := range d.commands {
		command := &d.commands[index]
		command.rect = transform(command.rect)
		command.radius *= scale
		command.stroke *= scale
		command.style.Size *= scale
		if len(command.points) > 0 {
			points := make([]Point, len(command.points))
			for i, point := range command.points {
				points[i] = Point{X: offset.X + point.X*scale, Y: offset.Y + point.Y*scale}
			}
			command.points = points
		}
	}
	if d.damage.Width > 0 {
		d.damage = transform(d.damage)
	}
	if d.nativeDamage.Width > 0 {
		d.nativeDamage = transform(d.nativeDamage)
	}
}

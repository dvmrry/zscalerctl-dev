package tea

type viewportState struct {
	Selected int
	Offset   int
}

func (v *viewportState) Clamp(total, height int) {
	if total <= 0 {
		v.Selected = 0
		v.Offset = 0
		return
	}
	if v.Selected < 0 {
		v.Selected = 0
	}
	if v.Selected >= total {
		v.Selected = total - 1
	}
	if height <= 0 {
		v.Offset = v.Selected
		return
	}
	maxOffset := total - height
	if maxOffset < 0 {
		maxOffset = 0
	}
	if v.Offset < 0 {
		v.Offset = 0
	}
	if v.Offset > maxOffset {
		v.Offset = maxOffset
	}
	if v.Selected < v.Offset {
		v.Offset = v.Selected
	}
	if v.Selected >= v.Offset+height {
		v.Offset = v.Selected - height + 1
	}
	if v.Offset > maxOffset {
		v.Offset = maxOffset
	}
}

func (v *viewportState) Move(delta, total, height int) {
	v.Selected += delta
	v.Clamp(total, height)
}

func (v *viewportState) Page(delta, total, height int) {
	step := height
	if step <= 0 {
		step = 1
	}
	v.Selected += delta * step
	v.Clamp(total, height)
}

func (v *viewportState) Home(total, height int) {
	v.Selected = 0
	v.Clamp(total, height)
}

func (v *viewportState) End(total, height int) {
	v.Selected = total - 1
	v.Clamp(total, height)
}

func (v viewportState) VisibleRange(total, height int) (int, int) {
	v.Clamp(total, height)
	if total <= 0 || height <= 0 {
		return 0, 0
	}
	start := v.Offset
	end := start + height
	if end > total {
		end = total
	}
	return start, end
}

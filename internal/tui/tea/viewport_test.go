package tea

import "testing"

func TestViewportStateMovementAndResizeClamping(t *testing.T) {
	var v viewportState

	v.Move(10, 200, 5)
	if got, want := v.Selected, 10; got != want {
		t.Fatalf("Move selected = %d, want %d", got, want)
	}
	if got, want := v.Offset, 6; got != want {
		t.Fatalf("Move offset = %d, want %d", got, want)
	}

	v.Page(1, 200, 5)
	if got, want := v.Selected, 15; got != want {
		t.Fatalf("Page selected = %d, want %d", got, want)
	}
	if got, want := v.Offset, 11; got != want {
		t.Fatalf("Page offset = %d, want %d", got, want)
	}

	v.Home(200, 5)
	if got := v; got != (viewportState{}) {
		t.Fatalf("Home viewport = %+v, want zero viewport", got)
	}

	v.End(200, 5)
	if got, want := v.Selected, 199; got != want {
		t.Fatalf("End selected = %d, want %d", got, want)
	}
	if got, want := v.Offset, 195; got != want {
		t.Fatalf("End offset = %d, want %d", got, want)
	}

	v.Clamp(200, 50)
	if got, want := v.Offset, 150; got != want {
		t.Fatalf("Clamp after resize offset = %d, want %d", got, want)
	}

	v.Clamp(0, 50)
	if got := v; got != (viewportState{}) {
		t.Fatalf("Clamp empty viewport = %+v, want zero viewport", got)
	}
}

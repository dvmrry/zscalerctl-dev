//go:build !race

package dump

// raceEnabled reports whether the race detector is compiled in. See
// race_on_test.go.
const raceEnabled = false

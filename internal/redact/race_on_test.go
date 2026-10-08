//go:build race

package redact_test

// raceEnabled reports whether the race detector is compiled in. The scan
// linearity checks skip under race: they compare timings, which race
// instrumentation distorts (and slows ~10x, enough to exceed the package test
// timeout).
const raceEnabled = true

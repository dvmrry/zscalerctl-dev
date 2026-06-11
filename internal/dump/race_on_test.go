//go:build race

package dump

// raceEnabled reports whether the race detector is compiled in. The
// large-tenant baseline skips under race: its purpose is measuring memory and
// throughput, both of which race instrumentation distorts (and slows ~20x,
// which would dominate the CI race shard).
const raceEnabled = true

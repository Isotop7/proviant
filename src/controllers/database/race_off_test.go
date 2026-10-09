//go:build !race

package database

// raceEnabled marks the build as running under the race detector. See
// race_on_test.go.
const raceEnabled = false

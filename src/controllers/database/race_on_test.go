//go:build race

package database

// raceEnabled marks the build as running under the race detector. Its shadow
// memory bookkeeping inflates every row scan several-fold, so wall-clock
// budgets tuned on a clean build must be scaled rather than treated as
// regressions.
const raceEnabled = true

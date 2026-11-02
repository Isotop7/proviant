package testutil

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm/logger"
)

// QueryCounter counts the SQL statements a GORM session runs, so a test can
// assert that the number of reads does not grow with the number of rows a
// request handles.
type QueryCounter struct {
	logger.Interface

	mutex   sync.Mutex
	queries []string
}

// NewQueryCounter returns a counter that discards the statements it records.
func NewQueryCounter() *QueryCounter {
	return &QueryCounter{Interface: logger.Discard}
}

// Trace records the statement of a finished query.
func (c *QueryCounter) Trace(_ context.Context, _ time.Time, fc func() (string, int64), _ error) {
	sql, _ := fc()
	c.mutex.Lock()
	c.queries = append(c.queries, sql)
	c.mutex.Unlock()
}

// CountOf returns how many recorded statements mention the given table.
func (c *QueryCounter) CountOf(table string) int {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	count := 0
	for _, query := range c.queries {
		if strings.Contains(query, table) {
			count++
		}
	}
	return count
}

// Statements returns the recorded statements, so a test can assert what a
// query looked like and not only how many there were.
func (c *QueryCounter) Statements() []string {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	return slices.Clone(c.queries)
}

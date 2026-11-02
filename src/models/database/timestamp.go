package database

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/Isotop7/proviant/util"
)

// Date is a simple wrapper for time.Time
type Date time.Time

// Timestamp is the model definition for timestamp
type Timestamp struct {
	Timestamp Date `json:"timestamp" binding:"required"`
}

// UnmarshalJSON parses JSON into Date. A date-only string names a calendar day
// in the container's zone, the zone the app renders dates in.
func (d *Date) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	t, err := time.ParseInLocation(util.DefaultDateFormatParseStr, s, time.Local)
	if err != nil {
		return err
	}
	*d = Date(t)
	return nil
}

// MarshalJSON generates JSON from a Date
func (d Date) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(d))
}

// Format returns a formatted string of Date
func (d Date) Format(s string) string {
	t := time.Time(d)
	return t.Format(s)
}

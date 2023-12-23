package models

import (
	"encoding/json"
	"strings"
	"time"
)

type Date time.Time

type Timestamp struct {
	Timestamp Date `json:"timestamp"`
}

func (d *Date) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), "\"")
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return err
	}
	*d = Date(t)
	return nil
}

func (d Date) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Time(d))
}

func (d Date) Format(s string) string {
	t := time.Time(d)
	return t.Format(s)
}

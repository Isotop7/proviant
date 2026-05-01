package database

import (
	"encoding/json"
	"testing"
	"time"

	"codeberg.org/isotop7/proviant/util"
)

func TestDateUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    Date
		wantErr bool
	}{
		{
			name:  "valid date",
			input: `"2023-12-25"`,
			want:  Date(time.Date(2023, 12, 25, 0, 0, 0, 0, time.UTC)),
		},
		{
			name:  "valid date - start of year",
			input: `"2023-01-01"`,
			want:  Date(time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)),
		},
		{
			name:  "valid date - end of year",
			input: `"2023-12-31"`,
			want:  Date(time.Date(2023, 12, 31, 0, 0, 0, 0, time.UTC)),
		},
		{
			name:    "invalid format - no quotes",
			input:   `2023-12-25`,
			wantErr: true,
		},
		{
			name:    "invalid format - wrong date format",
			input:   `"12/25/2023"`,
			wantErr: true,
		},
		{
			name:    "invalid format - incomplete date",
			input:   `"2023-12"`,
			wantErr: true,
		},
		{
			name:    "invalid format - garbage",
			input:   `"not-a-date"`,
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   `""`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var d Date
			err := json.Unmarshal([]byte(tt.input), &d)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if d != tt.want {
					t.Errorf("got %v, want %v", d, tt.want)
				}
			}
		})
	}
}

func TestDateMarshalJSON(t *testing.T) {
	now := time.Date(2023, 12, 25, 14, 30, 45, 0, time.UTC)
	d := Date(now)

	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var unmarshaledTime time.Time
	err = json.Unmarshal(data, &unmarshaledTime)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if now != unmarshaledTime {
		t.Errorf("got %v, want %v", unmarshaledTime, now)
	}
}

func TestDateFormat(t *testing.T) {
	tests := []struct {
		name string
		date Date
		fmt  string
		want string
	}{
		{
			name: "default format",
			date: Date(time.Date(2023, 12, 25, 14, 30, 0, 0, time.UTC)),
			fmt:  util.DefaultDateFormatParseStr,
			want: "2023-12-25",
		},
		{
			name: "custom format - day month year",
			date: Date(time.Date(2023, 12, 25, 14, 30, 0, 0, time.UTC)),
			fmt:  "02.01.2006",
			want: "25.12.2023",
		},
		{
			name: "custom format - with time",
			date: Date(time.Date(2023, 12, 25, 14, 30, 0, 0, time.UTC)),
			fmt:  "2006-01-02 15:04:05",
			want: "2023-12-25 14:30:00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.date.Format(tt.fmt)
			if result != tt.want {
				t.Errorf("got %v, want %v", result, tt.want)
			}
		})
	}
}

func TestTimestampUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "valid timestamp",
			input: `{"timestamp":"2023-12-25"}`,
			want:  "2023-12-25",
		},
		{
			name:    "invalid JSON",
			input:   `{invalid}`,
			wantErr: true,
		},
		{
			name:    "missing timestamp field (succeeds with zero Date)",
			input:   `{}`,
			wantErr: false,
			want:    "0001-01-01",
		},
		{
			name:    "invalid date format",
			input:   `{"timestamp":"12/25/2023"}`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ts Timestamp
			err := json.Unmarshal([]byte(tt.input), &ts)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				got := time.Time(ts.Timestamp).Format(util.DefaultDateFormatParseStr)
				if got != tt.want {
					t.Errorf("got %v, want %v", got, tt.want)
				}
			}
		})
	}
}

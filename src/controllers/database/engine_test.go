package database

import "testing"

func TestSupportedEnginesFromString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected SupportedEngines
	}{
		{
			name:     "mariadb lowercase",
			input:    "mariadb",
			expected: MariaDB,
		},
		{
			name:     "sqlite lowercase",
			input:    "sqlite",
			expected: SQLite,
		},
		{
			name:     "MariaDB mixed case",
			input:    "MariaDB",
			expected: InvalidEngine,
		},
		{
			name:     "SQLITE uppercase",
			input:    "SQLITE",
			expected: InvalidEngine,
		},
		{
			name:     "empty string",
			input:    "",
			expected: InvalidEngine,
		},
		{
			name:     "unknown engine",
			input:    "postgresql",
			expected: InvalidEngine,
		},
		{
			name:     "postgres",
			input:    "postgres",
			expected: InvalidEngine,
		},
		{
			name:     "mysql",
			input:    "mysql",
			expected: InvalidEngine,
		},
		{
			name:     "whitespace",
			input:    "   ",
			expected: InvalidEngine,
		},
		{
			name:     "random string",
			input:    "not-an-engine",
			expected: InvalidEngine,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SupportedEnginesFromString(tt.input)
			if result != tt.expected {
				t.Errorf("SupportedEnginesFromString(%v) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSupportedEnginesValues(t *testing.T) {
	t.Run("InvalidEngine is zero", func(t *testing.T) {
		if InvalidEngine != SupportedEngines(0) {
			t.Errorf("InvalidEngine = %v, want 0", InvalidEngine)
		}
	})

	t.Run("MariaDB is one", func(t *testing.T) {
		if MariaDB != SupportedEngines(1) {
			t.Errorf("MariaDB = %v, want 1", MariaDB)
		}
	})

	t.Run("SQLite is two", func(t *testing.T) {
		if SQLite != SupportedEngines(2) {
			t.Errorf("SQLite = %v, want 2", SQLite)
		}
	})
}

func TestSupportedEnginesStringConversion(t *testing.T) {
	t.Run("MariaDB to string and back", func(t *testing.T) {
		engines := []SupportedEngines{MariaDB, SQLite, InvalidEngine}
		for _, engine := range engines {
			if engine < InvalidEngine || engine > SQLite {
				t.Errorf("Unexpected engine value: %v", engine)
			}
		}
	})
}

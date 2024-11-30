package database

// SupportedEngines is a int value specifying a valid database engine
type SupportedEngines int

// Enum of supported engines
const (
	InvalidEngine SupportedEngines = iota
	MariaDB
	SQLite
)

// SupportedEnginesFromString parses and converts a given string to the matching enum value
// If the enum value can't be matched, enum value 'InvalidEngine' is used
func SupportedEnginesFromString(str string) SupportedEngines {
	switch str {
	case "mariadb":
		return MariaDB
	case "sqlite":
		return SQLite
	default:
		return InvalidEngine
	}
}

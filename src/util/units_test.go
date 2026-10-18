package util

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"testing"
)

func TestParseUnit(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantSymbol string
		wantKind   string
		wantOK     bool
	}{
		{name: "canonical gram", input: "g", wantSymbol: "g", wantKind: UnitKindMass, wantOK: true},
		{name: "uppercase litre", input: "L", wantSymbol: "l", wantKind: UnitKindVolume, wantOK: true},
		{name: "spelled out ounce", input: "Ounce", wantSymbol: "oz", wantKind: UnitKindMass, wantOK: true},
		{name: "fluid ounce with period", input: "Fl. Oz.", wantSymbol: "fl oz", wantKind: UnitKindVolume, wantOK: true},
		{name: "collapsed whitespace", input: "  fl   oz ", wantSymbol: "fl oz", wantKind: UnitKindVolume, wantOK: true},
		{name: "pound plural", input: "pounds", wantSymbol: "lb", wantKind: UnitKindMass, wantOK: true},
		{name: "count unit", input: "pcs", wantSymbol: "pcs", wantKind: UnitKindCount, wantOK: true},
		{name: "milligram is not convertible", input: "mg", wantOK: false},
		{name: "unknown unit", input: "handful", wantOK: false},
		{name: "empty unit", input: "  ", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unit, ok := ParseUnit(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("ParseUnit(%q) ok = %v, want %v", tt.input, ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if unit.Symbol != tt.wantSymbol {
				t.Errorf("ParseUnit(%q) symbol = %q, want %q", tt.input, unit.Symbol, tt.wantSymbol)
			}
			if unit.Kind != tt.wantKind {
				t.Errorf("ParseUnit(%q) kind = %q, want %q", tt.input, unit.Kind, tt.wantKind)
			}
		})
	}
}

func TestToBase(t *testing.T) {
	tests := []struct {
		name       string
		amount     int
		unit       string
		wantAmount int
		wantUnit   string
	}{
		{name: "gram unchanged", amount: 500, unit: "g", wantAmount: 500, wantUnit: "g"},
		{name: "milligram passes through unconverted", amount: 500, unit: "mg", wantAmount: 500, wantUnit: "mg"},
		{name: "kilogram to grams", amount: 2, unit: "kg", wantAmount: 2000, wantUnit: "g"},
		{name: "pound rounds half up", amount: 1, unit: "lb", wantAmount: 454, wantUnit: "g"},
		{name: "ounce rounds", amount: 1, unit: "oz", wantAmount: 28, wantUnit: "g"},
		{name: "litre to millilitres", amount: 1, unit: "L", wantAmount: 1000, wantUnit: "ml"},
		{name: "cup to millilitres", amount: 1, unit: "cup", wantAmount: 240, wantUnit: "ml"},
		{name: "tablespoon to millilitres", amount: 2, unit: "tbsp", wantAmount: 30, wantUnit: "ml"},
		{name: "count unchanged", amount: 3, unit: "pkg", wantAmount: 3, wantUnit: "pkg"},
		{name: "unknown passthrough", amount: 3, unit: "bunch of", wantAmount: 3, wantUnit: "bunch of"},
		{name: "empty passthrough", amount: 3, unit: "", wantAmount: 3, wantUnit: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			amount, unit := ToBase(tt.amount, tt.unit)
			if amount != tt.wantAmount {
				t.Errorf("ToBase(%d, %q) amount = %d, want %d", tt.amount, tt.unit, amount, tt.wantAmount)
			}
			if unit != tt.wantUnit {
				t.Errorf("ToBase(%d, %q) unit = %q, want %q", tt.amount, tt.unit, unit, tt.wantUnit)
			}
		})
	}
}

func TestToBaseIsIdempotentForBaseUnits(t *testing.T) {
	tests := []struct {
		amount int
		unit   string
	}{
		{amount: 0, unit: "g"},
		{amount: 999, unit: "g"},
		{amount: 1500, unit: "ml"},
		{amount: 7, unit: "pcs"},
	}

	for _, tt := range tests {
		t.Run(tt.unit, func(t *testing.T) {
			amount, unit := ToBase(tt.amount, tt.unit)
			againAmount, againUnit := ToBase(amount, unit)
			if againAmount != amount || againUnit != unit {
				t.Errorf("ToBase not idempotent: (%d, %q) -> (%d, %q) -> (%d, %q)",
					tt.amount, tt.unit, amount, unit, againAmount, againUnit)
			}
		})
	}
}

func TestConvert(t *testing.T) {
	tests := []struct {
		name   string
		amount int
		from   string
		to     string
		want   int
		wantOK bool
	}{
		{name: "grams to ounces", amount: 100, from: "g", to: "oz", want: 4, wantOK: true},
		{name: "ounces to grams", amount: 4, from: "oz", to: "g", want: 113, wantOK: true},
		{name: "millilitres to cups", amount: 480, from: "ml", to: "cup", want: 2, wantOK: true},
		{name: "tablespoons to teaspoons", amount: 1, from: "tbsp", to: "tsp", want: 3, wantOK: true},
		{name: "kilogram to grams", amount: 1, from: "kg", to: "g", want: 1000, wantOK: true},
		{name: "same factor", amount: 5, from: "cup", to: "cup", want: 5, wantOK: true},
		{name: "same count unit", amount: 3, from: "pcs", to: "pcs", want: 3, wantOK: true},
		{name: "different count units rejected", amount: 1, from: "pcs", to: "pkg", wantOK: false},
		{name: "mass to volume rejected", amount: 100, from: "g", to: "ml", wantOK: false},
		{name: "unknown unit rejected", amount: 1, from: "blob", to: "g", wantOK: false},
		{name: "empty unit rejected", amount: 1, from: "", to: "g", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := Convert(tt.amount, tt.from, tt.to)
			if ok != tt.wantOK {
				t.Fatalf("Convert(%d, %q, %q) ok = %v, want %v", tt.amount, tt.from, tt.to, ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Errorf("Convert(%d, %q, %q) = %d, want %d", tt.amount, tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestFormatUnit(t *testing.T) {
	tests := []struct {
		name   string
		amount int
		unit   string
		system string
		want   string
	}{
		{name: "empty unit", amount: 3, unit: "", system: UnitSystemMetric, want: "3"},
		{name: "grams metric", amount: 500, unit: "g", system: UnitSystemMetric, want: "500 g"},
		{name: "grams to kilo", amount: 1500, unit: "g", system: UnitSystemMetric, want: "1.5 kg"},
		{name: "exact kilo", amount: 2000, unit: "g", system: UnitSystemMetric, want: "2 kg"},
		{name: "grams imperial small", amount: 100, unit: "g", system: UnitSystemImperial, want: "3.5 oz"},
		{name: "grams imperial pound", amount: 454, unit: "g", system: UnitSystemImperial, want: "1 lb"},
		{name: "millilitres metric", amount: 750, unit: "ml", system: UnitSystemMetric, want: "750 ml"},
		{name: "millilitres volume litre", amount: 2000, unit: "ml", system: UnitSystemMetric, want: "2 l"},
		{name: "millilitres imperial cup", amount: 480, unit: "ml", system: UnitSystemImperial, want: "2 cup"},
		{name: "millilitres imperial tablespoon", amount: 30, unit: "ml", system: UnitSystemImperial, want: "2 tbsp"},
		{name: "millilitres imperial teaspoon", amount: 10, unit: "ml", system: UnitSystemImperial, want: "2 tsp"},
		{name: "count unchanged", amount: 4, unit: "pkg", system: UnitSystemImperial, want: "4 pkg"},
		{name: "unknown passthrough", amount: 4, unit: "bunch", system: UnitSystemMetric, want: "4 bunch"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatUnit(tt.amount, tt.unit, tt.system); got != tt.want {
				t.Errorf("FormatUnit(%d, %q, %q) = %q, want %q", tt.amount, tt.unit, tt.system, got, tt.want)
			}
		})
	}
}

func TestFromBaseEmptySystemDefaultsToMetric(t *testing.T) {
	value, symbol := FromBase(1500, "g", "")
	if symbol != "kg" || value != 1.5 {
		t.Errorf("FromBase(1500, g, \"\") = %v %s, want 1.5 kg", value, symbol)
	}
}

// UnitSystemFromContextValue normalises the raw gin-context value for a page
// render; anything that is not a stored preference means metric.
func TestUnitSystemFromContextValue(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  string
	}{
		{name: "missing", value: nil, want: UnitSystemMetric},
		{name: "non-string", value: 5, want: UnitSystemMetric},
		{name: "empty", value: "", want: UnitSystemMetric},
		{name: "metric", value: UnitSystemMetric, want: UnitSystemMetric},
		{name: "imperial", value: UnitSystemImperial, want: UnitSystemImperial},
		{name: "unknown", value: "us", want: UnitSystemMetric},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := UnitSystemFromContextValue(tt.value); got != tt.want {
				t.Errorf("UnitSystemFromContextValue(%v) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}

// UnitConversionJSON feeds the client-side table; it must serialise every entry
// with the symbol/kind/factor keys proviant.js reads, or the client silently
// falls back to raw display.
func TestUnitConversionJSON(t *testing.T) {
	var decoded []UnitConversion
	if err := json.Unmarshal([]byte(UnitConversionJSON()), &decoded); err != nil {
		t.Fatalf("UnitConversionJSON() is not valid JSON: %v", err)
	}
	if len(decoded) != len(conversionUnits) {
		t.Fatalf("entries = %d, want %d", len(decoded), len(conversionUnits))
	}
	for i := range conversionUnits {
		if decoded[i] != conversionUnits[i] {
			t.Errorf("entry %d = %+v, want %+v", i, decoded[i], conversionUnits[i])
		}
	}
}

func TestValidUnitSystem(t *testing.T) {
	tests := []struct {
		system string
		want   bool
	}{
		{system: "", want: true},
		{system: UnitSystemMetric, want: true},
		{system: UnitSystemImperial, want: true},
		{system: "imperial ", want: false},
		{system: "us", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.system, func(t *testing.T) {
			if got := ValidUnitSystem(tt.system); got != tt.want {
				t.Errorf("ValidUnitSystem(%q) = %v, want %v", tt.system, got, tt.want)
			}
		})
	}
}

// StorageUnitSymbols backs the unit-normalisation migration's skip predicate: a
// symbol must be listed exactly when converting it is the identity, so a listed
// row needs no rewrite and an unlisted one is never skipped by mistake.
func TestStorageUnitSymbolsMatchConversionIdentity(t *testing.T) {
	storage := make(map[string]bool, len(conversionUnits))
	for _, symbol := range StorageUnitSymbols() {
		storage[symbol] = true
		amount, unit := ToBase(1, symbol)
		if amount != 1 || unit != symbol {
			t.Errorf("StorageUnitSymbols() lists %q, but ToBase(1, %q) = (%d, %q)", symbol, symbol, amount, unit)
		}
	}
	for _, unit := range conversionUnits {
		amount, converted := ToBase(1, unit.Symbol)
		isIdentity := amount == 1 && converted == unit.Symbol
		if isIdentity != storage[unit.Symbol] {
			t.Errorf("StorageUnitSymbols() membership of %q = %v, want %v", unit.Symbol, storage[unit.Symbol], isIdentity)
		}
	}
}

func TestUnitOptionsCoverEveryTableEntry(t *testing.T) {
	options := UnitOptions()
	if len(options) != len(conversionUnits) {
		t.Fatalf("UnitOptions() returned %d options, want %d", len(options), len(conversionUnits))
	}
	for _, option := range options {
		if _, ok := ParseUnit(option.Value); !ok {
			t.Errorf("UnitOptions() value %q is not a recognised unit", option.Value)
		}
	}
}

// TestFrontendUnitTableMatchesServerTable pins the UNIT_TABLE copy in
// tests/js/helpers/loadScript.js to conversionUnits: the fixture exists because
// vitest cannot import Go, so a factor edited on one side must fail here
// instead of silently validating stale math.
func TestFrontendUnitTableMatchesServerTable(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "tests", "js", "helpers", "loadScript.js")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read frontend fixture: %v", err)
	}

	entry := regexp.MustCompile(`symbol:\s*"([^"]+)",\s*kind:\s*"([^"]+)",\s*factor:\s*([0-9.]+)`)
	matches := entry.FindAllStringSubmatch(string(source), -1)
	if len(matches) != len(conversionUnits) {
		t.Fatalf("frontend UNIT_TABLE has %d entries, want %d", len(matches), len(conversionUnits))
	}
	for i, match := range matches {
		want := conversionUnits[i]
		if match[1] != want.Symbol || match[2] != want.Kind {
			t.Errorf("frontend entry %d = %s/%s, want %s/%s", i, match[1], match[2], want.Symbol, want.Kind)
			continue
		}
		factor, parseErr := strconv.ParseFloat(match[3], 64)
		if parseErr != nil {
			t.Fatalf("frontend entry %d factor %q: %v", i, match[3], parseErr)
		}
		if factor != want.Factor {
			t.Errorf("frontend entry %d (%s) factor = %v, want %v", i, want.Symbol, factor, want.Factor)
		}
	}
}

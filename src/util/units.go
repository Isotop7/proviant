package util

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// Measurement systems select the unit a base amount is rendered in. An empty
// system means metric, which is also the storage base for mass and volume.
const (
	UnitSystemMetric   = "metric"
	UnitSystemImperial = "imperial"

	// Unit kinds. Conversion is only defined within one kind; mass never
	// converts to volume because the density of the substance is unknown.
	UnitKindMass   = "mass"
	UnitKindVolume = "volume"
	UnitKindCount  = "count"

	// BaseUnitMass and BaseUnitVolume are the canonical storage units. Every
	// mass product is stored in grams and every volume product in millilitres,
	// so amounts across a household aggregate without a unit lookup.
	BaseUnitMass   = "g"
	BaseUnitVolume = "ml"
)

// UnitConversion is one supported unit. Factor is the multiplier to the base
// unit of its kind (1 for the base itself); count units carry Factor 1 and are
// never scaled.
type UnitConversion struct {
	Symbol string  `json:"symbol"`
	Kind   string  `json:"kind"`
	Factor float64 `json:"factor"`
}

// UnitOption is one entry of the unit pickers fed to templates and the API.
type UnitOption struct {
	Value string
	Label string
}

// conversionUnits is the single conversion table. Amounts are stored in the
// base unit of the kind, so a factor of 1 must stay exact for the base rows:
// the data migration in src/migrations normalises existing rows through it and
// relies on the base rows being a no-op.
var conversionUnits = []UnitConversion{
	// mg is deliberately absent: the storage base is 1 g, so a milligram
	// amount rounds to zero grams and the migration would destroy it. An
	// "mg" unit falls through as an unknown unit and is displayed as-is.
	{Symbol: "g", Kind: UnitKindMass, Factor: 1},
	{Symbol: "kg", Kind: UnitKindMass, Factor: 1000},
	{Symbol: "oz", Kind: UnitKindMass, Factor: 28.349523125},
	{Symbol: "lb", Kind: UnitKindMass, Factor: 453.59237},

	{Symbol: "ml", Kind: UnitKindVolume, Factor: 1},
	{Symbol: "l", Kind: UnitKindVolume, Factor: 1000},
	{Symbol: "tsp", Kind: UnitKindVolume, Factor: 5},
	{Symbol: "tbsp", Kind: UnitKindVolume, Factor: 15},
	{Symbol: "cup", Kind: UnitKindVolume, Factor: 240},
	{Symbol: "fl oz", Kind: UnitKindVolume, Factor: 29.5735295625},

	{Symbol: "pcs", Kind: UnitKindCount, Factor: 1},
	{Symbol: "pkg", Kind: UnitKindCount, Factor: 1},
	{Symbol: "can", Kind: UnitKindCount, Factor: 1},
	{Symbol: "box", Kind: UnitKindCount, Factor: 1},
	{Symbol: "jar", Kind: UnitKindCount, Factor: 1},
	{Symbol: "bottle", Kind: UnitKindCount, Factor: 1},
	{Symbol: "bunch", Kind: UnitKindCount, Factor: 1},
	{Symbol: "slice", Kind: UnitKindCount, Factor: 1},
}

// unitAliases maps the spellings a user or an imported/OCR value may carry to
// the canonical symbol. Keys are already normalised (lowercase, no trailing
// dot, single spaces).
var unitAliases = map[string]string{
	"g": "g", "gr": "g", "gram": "g", "grams": "g", "gramm": "g",
	"kg": "kg", "kilo": "kg", "kilos": "kg", "kilogram": "kg", "kilograms": "kg",
	"oz": "oz", "ounce": "oz", "ounces": "oz",
	"lb": "lb", "lbs": "lb", "pound": "lb", "pounds": "lb",

	"ml": "ml", "milliliter": "ml", "milliliters": "ml", "millilitre": "ml", "millilitres": "ml",
	"l": "l", "liter": "l", "liters": "l", "litre": "l", "litres": "l",
	"tsp": "tsp", "teaspoon": "tsp", "teaspoons": "tsp",
	"tbsp": "tbsp", "tbs": "tbsp", "tablespoon": "tbsp", "tablespoons": "tbsp",
	"cup": "cup", "cups": "cup",
	"fl oz": "fl oz", "floz": "fl oz", "fl. oz": "fl oz", "fluid ounce": "fl oz", "fluid ounces": "fl oz",

	"pcs": "pcs", "pc": "pcs", "piece": "pcs", "pieces": "pcs",
	"pkg": "pkg", "package": "pkg", "packages": "pkg", "pack": "pkg", "packs": "pkg",
	"can": "can", "cans": "can",
	"box": "box", "boxes": "box",
	"jar": "jar", "jars": "jar",
	"bottle": "bottle", "bottles": "bottle",
	"bunch": "bunch", "bunches": "bunch",
	"slice": "slice", "slices": "slice",
}

var conversionBySymbol = func() map[string]UnitConversion {
	bySymbol := make(map[string]UnitConversion, len(conversionUnits))
	for _, unit := range conversionUnits {
		bySymbol[unit.Symbol] = unit
	}
	return bySymbol
}()

// normalizeUnitSymbol trims, lowercases, collapses inner whitespace and strips
// a trailing period so that "Fl. Oz." and "fl oz" resolve alike.
func normalizeUnitSymbol(raw string) string {
	trimmed := strings.TrimSpace(strings.ToLower(raw))
	trimmed = strings.TrimSuffix(trimmed, ".")
	return strings.Join(strings.Fields(trimmed), " ")
}

// ParseUnit resolves a raw unit string to its canonical conversion entry. The
// second result is false for an empty or unrecognised unit, which callers treat
// as a non-convertible count.
func ParseUnit(raw string) (UnitConversion, bool) {
	symbol, ok := unitAliases[normalizeUnitSymbol(raw)]
	if !ok {
		return UnitConversion{}, false
	}
	unit, ok := conversionBySymbol[symbol]
	return unit, ok
}

// ToBase converts an amount expressed in unit to the canonical base unit of its
// kind, rounding half away from zero. A unit that is empty, unknown or a plain
// count passes through unchanged (only the symbol is normalised when known), so
// OCR and import paths that carry arbitrary strings stay lossless.
func ToBase(amount int, raw string) (int, string) {
	unit, ok := ParseUnit(raw)
	if !ok {
		return amount, strings.TrimSpace(raw)
	}
	if unit.Kind == UnitKindCount || unit.Factor == 1 {
		return amount, unit.Symbol
	}
	return roundToInt(float64(amount) * unit.Factor), unitBaseSymbol(unit.Kind)
}

// Convert converts an amount from one unit to another of the same kind,
// rounding half away from zero. The second result is false when the units are
// unknown, empty, belong to different kinds, or are two different count
// denominations: a package and a piece are both count units, but neither has a
// size, so one is not convertible to the other.
func Convert(amount int, fromRaw, toRaw string) (int, bool) {
	from, fromOK := ParseUnit(fromRaw)
	to, toOK := ParseUnit(toRaw)
	if !fromOK || !toOK || from.Kind != to.Kind {
		return 0, false
	}
	if from.Kind == UnitKindCount {
		if from.Symbol != to.Symbol {
			return 0, false
		}
		return amount, true
	}
	if from.Factor == to.Factor {
		return amount, true
	}
	return roundToInt(float64(amount) * from.Factor / to.Factor), true
}

// IsMeasuredUnit reports whether a unit denotes a physical measurement (mass or
// volume) rather than a countable item. A measured product row is one package,
// so the UI shows its count instead of the net contents. Unknown and empty
// units are not measured: they are displayed exactly as stored.
func IsMeasuredUnit(raw string) bool {
	unit, ok := ParseUnit(raw)
	if !ok {
		return false
	}
	return unit.Kind == UnitKindMass || unit.Kind == UnitKindVolume
}

// IsPackageQuantity reports whether an amount of unit is whole packages for
// display: a mass or volume unit is always one package, its net contents shown
// as secondary text, and a single count item is one package too. Unknown and
// empty units are not packages, so "4 handful" stays as stored.
func IsPackageQuantity(amount int, raw string) bool {
	unit, ok := ParseUnit(raw)
	if !ok {
		return false
	}
	if unit.Kind == UnitKindMass || unit.Kind == UnitKindVolume {
		return true
	}
	return unit.Kind == UnitKindCount && amount == 1
}

// FromBase renders a canonical base amount in the requested measurement system,
// picking the unit a person would write. Count units and unknown units are
// returned as-is. An empty system means metric.
func FromBase(amount int, raw string, system string) (float64, string) {
	unit, ok := ParseUnit(raw)
	if !ok {
		return float64(amount), strings.TrimSpace(raw)
	}
	switch unit.Kind {
	case UnitKindMass:
		return fromBaseMass(float64(amount), system)
	case UnitKindVolume:
		return fromBaseVolume(float64(amount), system)
	default:
		return float64(amount), unit.Symbol
	}
}

func fromBaseMass(base float64, system string) (float64, string) {
	if system == UnitSystemImperial {
		if math.Abs(base) >= conversionBySymbol["lb"].Factor {
			return base / conversionBySymbol["lb"].Factor, "lb"
		}
		return base / conversionBySymbol["oz"].Factor, "oz"
	}
	if math.Abs(base) >= 1000 {
		return base / 1000, "kg"
	}
	return base, "g"
}

func fromBaseVolume(base float64, system string) (float64, string) {
	if system == UnitSystemImperial {
		switch {
		case math.Abs(base) >= conversionBySymbol["cup"].Factor:
			return base / conversionBySymbol["cup"].Factor, "cup"
		case math.Abs(base) >= conversionBySymbol["tbsp"].Factor:
			return base / conversionBySymbol["tbsp"].Factor, "tbsp"
		case math.Abs(base) >= conversionBySymbol["tsp"].Factor:
			return base / conversionBySymbol["tsp"].Factor, "tsp"
		default:
			return base / conversionBySymbol["fl oz"].Factor, "fl oz"
		}
	}
	if math.Abs(base) >= 1000 {
		return base / 1000, "l"
	}
	return base, "ml"
}

// FormatUnit renders an amount and its unit in the requested system, e.g.
// "500 g" or "1.1 lb". A unit-less amount is rendered as its bare quantity, so
// a count with no unit still shows a value instead of an empty label.
func FormatUnit(amount int, raw string, system string) string {
	if strings.TrimSpace(raw) == "" {
		return formatUnitQuantity(float64(amount))
	}
	value, symbol := FromBase(amount, raw, system)
	return formatUnitQuantity(value) + " " + symbol
}

// UnitOptions returns the curated picker list. Metric units come first so the
// default option of a metric household is also the first entry.
func UnitOptions() []UnitOption {
	options := make([]UnitOption, 0, len(conversionUnits))
	for _, unit := range conversionUnits {
		options = append(options, UnitOption{Value: unit.Symbol, Label: unit.Symbol})
	}
	return options
}

// UnitSymbols lists every recognised symbol, for tests and validation.
func UnitSymbols() []string {
	symbols := make([]string, 0, len(conversionUnits))
	for _, unit := range conversionUnits {
		symbols = append(symbols, unit.Symbol)
	}
	return symbols
}

// StorageUnitSymbols lists the symbols an amount is already stored in: the base
// unit of each measurable kind (g, ml) plus every count unit, all of which carry
// Factor 1 and convert to themselves. Rows carrying one of these need no
// conversion, which lets the data migration skip them.
func StorageUnitSymbols() []string {
	symbols := make([]string, 0, len(conversionUnits))
	for _, unit := range conversionUnits {
		if unit.Factor == 1 {
			symbols = append(symbols, unit.Symbol)
		}
	}
	return symbols
}

// UnitOptionsJSON renders the picker symbols as a JSON array for embedding in a
// page for the client-side unit selects.
func UnitOptionsJSON() string {
	encoded, err := json.Marshal(UnitSymbols())
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

// UnitConversionJSON renders the whole conversion table as a JSON array for the
// frontend, which needs the same factors to format a stepper-updated label and
// to rescale the amount field when the picker changes. The table has one source
// — this package — so the client can never drift from the server factors.
func UnitConversionJSON() string {
	encoded, err := json.Marshal(conversionUnits)
	if err != nil {
		return "[]"
	}
	return string(encoded)
}

// UnitAliasJSON renders the alias map for the frontend, which needs to resolve
// the same spellings the server does ("pc", "Millilitres") to a canonical
// symbol before it looks the unit up in the conversion table.
func UnitAliasJSON() string {
	encoded, err := json.Marshal(unitAliases)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}

// ValidUnitSystem reports whether a stored household preference is usable;
// the empty string is valid and means metric.
func ValidUnitSystem(system string) bool {
	return system == "" || system == UnitSystemMetric || system == UnitSystemImperial
}

// ResolveUnitSystem normalises a household preference for display; an empty or
// unknown value falls back to metric.
func ResolveUnitSystem(system string) string {
	if system == UnitSystemImperial {
		return UnitSystemImperial
	}
	return UnitSystemMetric
}

// UnitSystemFromContextValue normalises the value stored under
// ContextKeyUnitSystem; a missing or non-string value means metric.
func UnitSystemFromContextValue(value any) string {
	system, ok := value.(string)
	if !ok {
		return UnitSystemMetric
	}
	return ResolveUnitSystem(system)
}

func unitBaseSymbol(kind string) string {
	if kind == UnitKindVolume {
		return BaseUnitVolume
	}
	return BaseUnitMass
}

func roundToInt(value float64) int {
	return int(math.Round(value))
}

// formatUnitQuantity renders a converted amount with at most one decimal and no
// trailing zero, so integral values read as "2" rather than "2.0".
func formatUnitQuantity(value float64) string {
	rounded := math.Round(value*10) / 10
	if rounded == math.Trunc(rounded) {
		return fmt.Sprintf("%d", int64(rounded))
	}
	return fmt.Sprintf("%.1f", rounded)
}

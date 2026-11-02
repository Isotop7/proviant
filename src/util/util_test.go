package util

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func TestDateFormatConstants(t *testing.T) {
	tests := []struct {
		name   string
		format string
		input  string
		want   time.Time
	}{
		{
			name:   "DefaultDateFormatParseStr",
			format: DefaultDateFormatParseStr,
			input:  "2026-10-09",
			want:   time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC),
		},
		{
			name:   "DefaultDateFormatMonthStr",
			format: DefaultDateFormatMonthStr,
			input:  "2026-10",
			want:   time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := time.Parse(tt.format, tt.input)
			if err != nil {
				t.Fatalf("time.Parse(%q, %q) error: %v", tt.format, tt.input, err)
			}
			if !parsed.Equal(tt.want) {
				t.Errorf("parsed = %v, want %v", parsed, tt.want)
			}
		})
	}

	t.Run("DefaultDateFormatParseStr rejects invalid format", func(t *testing.T) {
		if _, err := time.Parse(DefaultDateFormatParseStr, "09.10.2026"); err == nil {
			t.Errorf("time.Parse should reject %q", "09.10.2026")
		}
	})

	t.Run("DefaultDateFormatParseStr rejects empty input", func(t *testing.T) {
		if _, err := time.Parse(DefaultDateFormatParseStr, ""); err == nil {
			t.Errorf("time.Parse should reject empty input")
		}
	})
}

func TestCsvImportColumnAliases(t *testing.T) {
	canonical := map[string]bool{
		CsvColumnName:            true,
		CsvColumnBarcode:         true,
		CsvColumnQuantity:        true,
		CsvColumnPackSize:        true,
		CsvColumnCategory:        true,
		CsvColumnStorageLocation: true,
		CsvColumnExpiryDate:      true,
		CsvColumnAddedAt:         true,
	}

	for alias, target := range CsvImportColumnAliases {
		t.Run("alias/"+alias, func(t *testing.T) {
			if !canonical[target] {
				t.Errorf("alias %q maps to non-canonical column %q", alias, target)
			}
		})
	}

	for col := range canonical {
		t.Run("canonical/"+col, func(t *testing.T) {
			got, ok := CsvImportColumnAliases[col]
			if !ok || got != col {
				t.Errorf("canonical column %q maps to %q (present=%v), want itself", col, got, ok)
			}
		})
	}

	// A header of the pre-count-model format must not be aliased: its
	// quantity column held a net content, and reading it as a pack count
	// would turn a net content of 500 into 500 packs.
	for _, legacy := range CsvLegacyColumnHeaders {
		t.Run("legacy/"+legacy, func(t *testing.T) {
			if target, ok := CsvImportColumnAliases[legacy]; ok {
				t.Errorf("legacy column %q is aliased to %q, want no alias", legacy, target)
			}
		})
	}
}

func TestCsvImportBarcodePattern(t *testing.T) {
	re := regexp.MustCompile(CsvImportBarcodePattern)

	valid := []string{"1234567890123", "ABC-123_xyz", "4006381333931"}
	for _, v := range valid {
		t.Run("valid/"+v, func(t *testing.T) {
			if !re.MatchString(v) {
				t.Errorf("pattern should match %q", v)
			}
		})
	}

	invalid := []string{"", "has space", "with/slash", "dot.name", "semi;colon"}
	for _, v := range invalid {
		t.Run("invalid/"+v, func(t *testing.T) {
			if re.MatchString(v) {
				t.Errorf("pattern should not match %q", v)
			}
		})
	}

	t.Run("boundary lengths match pattern", func(t *testing.T) {
		minBarcode := strings.Repeat("1", CsvImportMinBarcodeLength)
		maxBarcode := strings.Repeat("1", CsvImportMaxBarcodeLength)
		if !re.MatchString(minBarcode) {
			t.Errorf("pattern should match min-length barcode %q", minBarcode)
		}
		if !re.MatchString(maxBarcode) {
			t.Errorf("pattern should match max-length barcode %q", maxBarcode)
		}
	})

	t.Run("min length not above max length", func(t *testing.T) {
		if CsvImportMinBarcodeLength > CsvImportMaxBarcodeLength {
			t.Errorf("CsvImportMinBarcodeLength %d > CsvImportMaxBarcodeLength %d",
				CsvImportMinBarcodeLength, CsvImportMaxBarcodeLength)
		}
	})
}

func TestLimitsArePositive(t *testing.T) {
	tests := []struct {
		name string
		got  int
	}{
		{"CsvImportMaxRows", CsvImportMaxRows},
		{"CsvImportMaxNewLocations", CsvImportMaxNewLocations},
		{"CsvImportOpenFoodFactsLookups", CsvImportOpenFoodFactsLookups},
		{"BulkCreateMaxItems", BulkCreateMaxItems},
		{"ReceiptBulkMaxItems", ReceiptBulkMaxItems},
		{"ReceiptScanMaxTokens", ReceiptScanMaxTokens},
		{"ReceiptScanDefaultTimeout", ReceiptScanDefaultTimeout},
		{"ReceiptItemMaxNameLength", ReceiptItemMaxNameLength},
		{"ReceiptItemMaxCategoriesLength", ReceiptItemMaxCategoriesLength},
		{"MaxPackSizeLength", MaxPackSizeLength},
		{"MaxProductNameLength", MaxProductNameLength},
		{"FrequentProductMinPurchases", FrequentProductMinPurchases},
		{"FrequentProductLimit", FrequentProductLimit},
		{"MinProductQuantity", MinProductQuantity},
		{"MaxProductQuantity", MaxProductQuantity},
		{"DefaultMaxUploadSizeMB", DefaultMaxUploadSizeMB},
		{"ConsumptionHistoryWindowDays", ConsumptionHistoryWindowDays},
		{"ConsumptionMinSamples", ConsumptionMinSamples},
		{"ConsumptionMinSpanDays", ConsumptionMinSpanDays},
		{"ConsumptionSpanFloorDays", ConsumptionSpanFloorDays},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got < 1 {
				t.Errorf("%s = %d, want >= 1", tt.name, tt.got)
			}
		})
	}

	t.Run("ReceiptBulkMaxBodyBytes covers items plus envelope", func(t *testing.T) {
		want := ReceiptBulkMaxItems*4096 + 4096
		if ReceiptBulkMaxBodyBytes != want {
			t.Errorf("ReceiptBulkMaxBodyBytes = %d, want %d", ReceiptBulkMaxBodyBytes, want)
		}
	})

	t.Run("multipart slack positive", func(t *testing.T) {
		if CsvImportMultipartSlackBytes < 1 || ReceiptScanMultipartSlackBytes < 1 {
			t.Errorf("multipart slack bytes must be positive, got %d and %d",
				CsvImportMultipartSlackBytes, ReceiptScanMultipartSlackBytes)
		}
	})

	t.Run("open food facts budget positive", func(t *testing.T) {
		if CsvImportOpenFoodFactsBudget <= 0 {
			t.Errorf("CsvImportOpenFoodFactsBudget = %v, want > 0", CsvImportOpenFoodFactsBudget)
		}
	})

	t.Run("min span not above history window", func(t *testing.T) {
		if ConsumptionMinSpanDays > ConsumptionHistoryWindowDays {
			t.Errorf("ConsumptionMinSpanDays %d > ConsumptionHistoryWindowDays %d",
				ConsumptionMinSpanDays, ConsumptionHistoryWindowDays)
		}
	})

	t.Run("receipt max price positive", func(t *testing.T) {
		if ReceiptItemMaxPrice <= 0 {
			t.Errorf("ReceiptItemMaxPrice = %v, want > 0", ReceiptItemMaxPrice)
		}
	})
}

func TestStringConstants(t *testing.T) {
	t.Run("routes start with slash", func(t *testing.T) {
		routes := map[string]string{
			"RouteUnsubscribe":    RouteUnsubscribe,
			"RouteAuth":           RouteAuth,
			"RouteForgotPassword": RouteForgotPassword,
			"RouteResetPassword":  RouteResetPassword,
		}
		for name, route := range routes {
			if !strings.HasPrefix(route, "/") {
				t.Errorf("%s = %q, want leading slash", name, route)
			}
		}
	})

	t.Run("RetiredTokenPassword is non-empty", func(t *testing.T) {
		if RetiredTokenPassword == "" {
			t.Errorf("RetiredTokenPassword is empty")
		}
	})

	t.Run("DefaultTheMealDBRecipeAPIURL is https", func(t *testing.T) {
		if !strings.HasPrefix(DefaultTheMealDBRecipeAPIURL, "https://") {
			t.Errorf("DefaultTheMealDBRecipeAPIURL = %q, want https prefix", DefaultTheMealDBRecipeAPIURL)
		}
	})

	t.Run("period values are distinct", func(t *testing.T) {
		periods := map[string]string{
			"PeriodValueMonth":    PeriodValueMonth,
			"PeriodValue3Months":  PeriodValue3Months,
			"PeriodValue6Months":  PeriodValue6Months,
			"PeriodValue12Months": PeriodValue12Months,
		}
		seen := map[string]string{}
		for name, value := range periods {
			if value == "" {
				t.Errorf("%s is empty", name)
			}
			if prev, dup := seen[value]; dup {
				t.Errorf("%s and %s share value %q", prev, name, value)
			}
			seen[value] = name
		}
	})

	t.Run("csv import template filename has csv extension", func(t *testing.T) {
		if !strings.HasSuffix(CsvImportTemplateFilename, ".csv") {
			t.Errorf("CsvImportTemplateFilename = %q, want .csv suffix", CsvImportTemplateFilename)
		}
	})
}

func TestOffDumpSQLiteFilename(t *testing.T) {
	tests := []struct {
		name         string
		databaseName string
		want         string
	}{
		{
			name:         "empty falls back to the default dump file",
			databaseName: "",
			want:         OffDumpDBFilename,
		},
		{
			name:         "default name keeps the documented path",
			databaseName: OffDumpDefaultDatabase,
			want:         OffDumpDBFilename,
		},
		{
			name:         "bare name lives beside the main database",
			databaseName: "proviant_off_dump",
			want:         filepath.Join(filepath.Dir(OffDumpDBFilename), "proviant_off_dump.db"),
		},
		{
			name:         "absolute path is used verbatim",
			databaseName: "/var/opt/off_dump",
			want:         "/var/opt/off_dump",
		},
		{
			name:         "relative path is used verbatim",
			databaseName: "var/opt/off_dump",
			want:         "var/opt/off_dump",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := OffDumpSQLiteFilename(test.databaseName); got != test.want {
				t.Errorf("OffDumpSQLiteFilename(%q) = %q, want %q", test.databaseName, got, test.want)
			}
		})
	}
}

func TestOffDumpStagingFilename(t *testing.T) {
	tests := []struct {
		name         string
		databaseName string
		want         string
	}{
		{
			name:         "empty falls back to the default name",
			databaseName: "",
			want:         OffDumpDefaultDatabase + OffDumpStagingSuffix,
		},
		{
			name:         "default name keeps the documented file",
			databaseName: OffDumpDefaultDatabase,
			want:         "off_dump" + OffDumpStagingSuffix,
		},
		{
			name:         "bare name is carried over",
			databaseName: "proviant_off_dump",
			want:         "proviant_off_dump" + OffDumpStagingSuffix,
		},
		{
			name:         "a path contributes only its base name",
			databaseName: "/var/opt/off_dump",
			want:         "off_dump" + OffDumpStagingSuffix,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := OffDumpStagingFilename(test.databaseName); got != test.want {
				t.Errorf("OffDumpStagingFilename(%q) = %q, want %q", test.databaseName, got, test.want)
			}
		})
	}
}

func TestIsHTTPSBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		want    bool
	}{
		{
			name:    "https deployment is secure",
			baseURL: "https://proviant.local.de",
			want:    true,
		},
		{
			name:    "https with a path stays secure",
			baseURL: "https://proviant.local.de/proviant",
			want:    true,
		},
		{
			name:    "uppercase scheme is still secure",
			baseURL: "HTTPS://proviant.local.de",
			want:    true,
		},
		{
			name:    "surrounding whitespace is ignored",
			baseURL: "  https://proviant.local.de  ",
			want:    true,
		},
		{
			name:    "http deployment is not secure",
			baseURL: "http://127.0.0.1:5114",
			want:    false,
		},
		{
			name:    "empty base URL is not secure",
			baseURL: "",
			want:    false,
		},
		{
			name:    "a scheme in the host does not count",
			baseURL: "proviant.local.de/https",
			want:    false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := IsHTTPSBaseURL(test.baseURL); got != test.want {
				t.Errorf("IsHTTPSBaseURL(%q) = %v, want %v", test.baseURL, got, test.want)
			}
		})
	}
}

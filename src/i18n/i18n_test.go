package i18n

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

func TestCatalogsLoad(t *testing.T) {
	load()
	if loadErr != nil {
		t.Fatalf("load() error = %v", loadErr)
	}
	for _, want := range []string{LanguageEN, LanguageDE} {
		if !Supported(want) {
			t.Errorf("Supported(%q) = false, want true", want)
		}
	}
}

func TestTFallbackChain(t *testing.T) {
	if got := T(LanguageDE, "common.ok"); got != "OK" {
		t.Errorf("T(de, common.ok) = %q, want %q", got, "OK")
	}
	if got := T(LanguageDE, "does.not.exist"); got != "does.not.exist" {
		t.Errorf("T(de, missing) = %q, want key fallback", got)
	}
	if got := T("de-AT", "common.cancel"); got != "Abbrechen" {
		t.Errorf("T(de-AT, common.cancel) = %q, want %q", got, "Abbrechen")
	}
	if got := T("xx", "common.cancel"); got != "Cancel" {
		t.Errorf("T(xx, common.cancel) = %q, want default-language fallback", got)
	}
}

func TestTFormatArgs(t *testing.T) {
	if got := T(LanguageEN, "error.title", 404); got != "Error 404" {
		t.Errorf("T(en, error.title, 404) = %q, want %q", got, "Error 404")
	}
	if got := T(LanguageDE, "error.title", 500); got != "Fehler 500" {
		t.Errorf("T(de, error.title, 500) = %q, want %q", got, "Fehler 500")
	}
}

func TestTnPluralSelection(t *testing.T) {
	load()
	catalogs[LanguageEN]["test.plain_one"] = "%d item left"
	catalogs[LanguageEN]["test.plain_other"] = "%d items left"
	catalogs[LanguageEN]["test.plain"] = "%d total"
	t.Cleanup(func() {
		delete(catalogs[LanguageEN], "test.plain_one")
		delete(catalogs[LanguageEN], "test.plain_other")
		delete(catalogs[LanguageEN], "test.plain")
	})

	if got := Tn(LanguageEN, "test.plain", 1); got != "1 item left" {
		t.Errorf("Tn(n=1) = %q, want %q", got, "1 item left")
	}
	if got := Tn(LanguageEN, "test.plain", 3); got != "3 items left" {
		t.Errorf("Tn(n=3) = %q, want %q", got, "3 items left")
	}
	if got := Tn(LanguageEN, "test.singularOnly", 2); got != "test.singularOnly" {
		t.Errorf("Tn(missing) = %q, want key fallback", got)
	}
}

func TestResolveLanguageChain(t *testing.T) {
	tests := []struct {
		user, household, header, want string
	}{
		{"de", "en", "en", "de"},
		{"", "de", "en", "de"},
		{"", "", "de-DE,de;q=0.9,en;q=0.8", "de"},
		{"", "", "fr-FR,fr;q=0.9", LanguageEN},
		{"", "", "", LanguageEN},
		{"xx", "yy", "", LanguageEN},
	}
	for _, tt := range tests {
		if got := ResolveLanguage(tt.user, tt.household, tt.header); got != tt.want {
			t.Errorf("ResolveLanguage(%q, %q, %q) = %q, want %q", tt.user, tt.household, tt.header, got, tt.want)
		}
	}
}

func TestCatalogParity(t *testing.T) {
	load()
	if loadErr != nil {
		t.Fatalf("load() error = %v", loadErr)
	}
	baseline := catalogs[DefaultLanguage]
	for lang, catalog := range catalogs {
		if lang == DefaultLanguage {
			continue
		}
		for key := range catalog {
			if _, ok := baseline[key]; !ok {
				t.Errorf("language %q has key %q missing from %q", lang, key, DefaultLanguage)
			}
		}
		for key := range baseline {
			if _, ok := catalog[key]; !ok {
				t.Errorf("language %q is missing key %q", lang, key)
			}
		}
	}
}

var (
	templateKeyPattern = regexp.MustCompile(`\{\{\s*t[n]?\s+"([^"]+)"`)
	scriptKeyPattern   = regexp.MustCompile(`proviant\.t[n]?\("([^"]+)"`)
	pageTitlePattern   = regexp.MustCompile(`eq \.Title "([^"]+)"`)
)

func collectSourceFiles(t *testing.T, roots ...string) []string {
	t.Helper()
	var files []string
	for _, root := range roots {
		walkErr := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if strings.Contains(path, ".min.") {
				return nil
			}
			switch filepath.Ext(path) {
			case ".tmpl", ".html", ".txt", ".js", ".go":
				files = append(files, path)
			}
			return nil
		})
		if walkErr != nil {
			t.Fatalf("walk %s: %v", root, walkErr)
		}
	}
	sort.Strings(files)
	return files
}

// TestReferencedKeysExist pins every i18n key used in templates, frontend
// scripts and notification templates against the catalogs.
func TestReferencedKeysExist(t *testing.T) {
	load()
	if loadErr != nil {
		t.Fatalf("load() error = %v", loadErr)
	}
	baseline := catalogs[DefaultLanguage]

	roots := []string{"../templates", "../assets/js"}
	for _, root := range roots {
		if _, err := os.Stat(root); err != nil {
			t.Fatalf("source root %s unavailable: %v", root, err)
		}
	}

	for _, file := range collectSourceFiles(t, roots...) {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		content := string(raw)

		for _, pattern := range []*regexp.Regexp{templateKeyPattern, scriptKeyPattern} {
			for _, match := range pattern.FindAllStringSubmatch(content, -1) {
				key := match[1]
				if _, ok := baseline[key]; !ok {
					t.Errorf("%s references key %q missing from %q catalog", file, key, DefaultLanguage)
				}
			}
		}

		if strings.HasSuffix(file, ".tmpl") {
			for _, match := range pageTitlePattern.FindAllStringSubmatch(content, -1) {
				key := "pageTitle." + strings.ReplaceAll(match[1], " ", "")
				if _, ok := baseline[key]; !ok {
					t.Errorf("%s compares .Title %q but key %q is missing", file, match[1], key)
				}
			}
		}
	}
}

// TestWebHandlerPageTitlesExist pins the translated page titles for every
// Title the web handlers hand to the templates.
func TestWebHandlerPageTitlesExist(t *testing.T) {
	load()
	baseline := catalogs[DefaultLanguage]

	titlePattern := regexp.MustCompile(`Title:\s*"([^"]+)"`)
	files := collectSourceFiles(t, "../web")
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		for _, match := range titlePattern.FindAllStringSubmatch(string(raw), -1) {
			key := "pageTitle." + strings.ReplaceAll(match[1], " ", "")
			if _, ok := baseline[key]; !ok {
				t.Errorf("%s sets Title %q but key %q is missing", file, match[1], key)
			}
		}
	}
}

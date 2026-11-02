// Package i18n provides message catalogs and language resolution for the UI.
package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"maps"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

//go:embed all:catalogs
var catalogFiles embed.FS

const (
	LanguageEN      = "en"
	LanguageDE      = "de"
	DefaultLanguage = LanguageEN
	// PlaceholderFormat documents the catalog placeholder style (%s, %d)
	// shared by T/Tf and the frontend proviant.t helper.
	PlaceholderFormat = "%s"
)

var (
	loadOnce  sync.Once
	catalogs  map[string]map[string]string
	languages []string
	loadErr   error
)

func load() {
	loadOnce.Do(func() {
		catalogs = map[string]map[string]string{}
		languages = nil

		langDirs, dirErr := fs.ReadDir(catalogFiles, "catalogs")
		if dirErr != nil {
			loadErr = dirErr
			return
		}

		for _, langDir := range langDirs {
			if !langDir.IsDir() {
				continue
			}
			lang := langDir.Name()
			merged := map[string]string{}

			files, fileErr := fs.ReadDir(catalogFiles, filepath.Join("catalogs", lang))
			if fileErr != nil {
				loadErr = fileErr
				return
			}
			for _, file := range files {
				if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
					continue
				}
				entries := map[string]string{}
				raw, readErr := fs.ReadFile(catalogFiles, filepath.Join("catalogs", lang, file.Name()))
				if readErr != nil {
					loadErr = readErr
					return
				}
				if unmarshalErr := json.Unmarshal(raw, &entries); unmarshalErr != nil {
					loadErr = fmt.Errorf("i18n: %s/%s: %w", lang, file.Name(), unmarshalErr)
					return
				}
				for key, value := range entries {
					if _, exists := merged[key]; exists {
						loadErr = fmt.Errorf("i18n: duplicate key %q in language %s", key, lang)
						return
					}
					merged[key] = value
				}
			}

			catalogs[lang] = merged
			languages = append(languages, lang)
		}

		sort.Strings(languages)
	})
}

// Languages returns the sorted list of languages with a catalog.
func Languages() []string {
	load()
	if loadErr != nil {
		panic(loadErr)
	}
	out := make([]string, len(languages))
	copy(out, languages)
	return out
}

// Supported reports whether lang has a catalog.
func Supported(lang string) bool {
	load()
	_, ok := catalogs[Normalize(lang)]
	return ok
}

// Normalize maps a language tag to its base catalog language ("de-AT" -> "de").
// Unknown languages normalize to the empty string.
func Normalize(lang string) string {
	load()
	base := strings.ToLower(strings.TrimSpace(lang))
	if base == "" {
		return ""
	}
	if i := strings.IndexAny(base, "-_"); i > 0 {
		base = base[:i]
	}
	if _, ok := catalogs[base]; ok {
		return base
	}
	return ""
}

// T returns the catalog text for key, falling back to the default language and
// finally to the key itself. vars replace %s / %d placeholders in order.
func T(lang, key string, args ...any) string {
	load()
	text, ok := lookup(lang, key)
	if !ok {
		return key
	}
	return format(text, args)
}

// Tn returns the plural catalog text for key. It looks up key_one for n == 1
// and key_other otherwise, falling back to the plain key. n is the first
// format argument.
func Tn(lang, key string, n int, args ...any) string {
	suffix := "_other"
	if n == 1 {
		suffix = "_one"
	}
	formatArgs := append([]any{n}, args...)
	if T(lang, key+suffix) != key+suffix {
		return T(lang, key+suffix, formatArgs...)
	}
	return T(lang, key, formatArgs...)
}

// Has reports whether key exists in lang or the default language.
func Has(lang, key string) bool {
	load()
	_, ok := lookup(lang, key)
	return ok
}

func lookup(lang, key string) (string, bool) {
	if base := Normalize(lang); base != "" {
		if text, ok := catalogs[base][key]; ok {
			return text, true
		}
	}
	if text, ok := catalogs[DefaultLanguage][key]; ok {
		return text, true
	}
	return "", false
}

func format(text string, args []any) string {
	if len(args) == 0 {
		return text
	}
	var out strings.Builder
	argIndex := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '%' && i+1 < len(text) && argIndex < len(args) {
			switch text[i+1] {
			case 's', 'd', 'v':
				fmt.Fprintf(&out, "%"+string(text[i+1]), args[argIndex])
				argIndex++
				i++
				continue
			case '%':
				out.WriteByte('%')
				i++
				continue
			}
		}
		out.WriteByte(text[i])
	}
	return out.String()
}

// JSON returns the merged catalog for lang (with default-language fallback
// entries included) as a JSON object for frontend consumption.
func JSON(lang string) string {
	load()
	merged := map[string]string{}
	maps.Copy(merged, catalogs[DefaultLanguage])
	if base := Normalize(lang); base != "" && base != DefaultLanguage {
		maps.Copy(merged, catalogs[base])
	}
	raw, err := json.Marshal(merged)
	if err != nil {
		return "{}"
	}
	return string(raw)
}

// FuncMap returns template functions "t", "tn" and "lang" bound to lang.
func FuncMap(lang string) template.FuncMap {
	return template.FuncMap{
		"t":     func(key string, args ...any) string { return T(lang, key, args...) },
		"tn":    func(key string, n int, args ...any) string { return Tn(lang, key, n, args...) },
		"tlang": func() string { return lang },
	}
}

// LanguageOption is one selectable UI language for the settings forms.
type LanguageOption struct {
	Code string
	Name string
}

var languageNames = map[string]string{
	LanguageEN: "English",
	LanguageDE: "Deutsch",
}

// LanguageOptions returns the selectable languages as endonym labels.
func LanguageOptions() []LanguageOption {
	options := make([]LanguageOption, 0, len(languages))
	for _, lang := range Languages() {
		name, ok := languageNames[lang]
		if !ok {
			name = lang
		}
		options = append(options, LanguageOption{Code: lang, Name: name})
	}
	return options
}

// ResolveLanguage picks the UI language: explicit user choice, then the
// household default, then the Accept-Language header, then the default.
func ResolveLanguage(userLang, householdLang, acceptLanguage string) string {
	if base := Normalize(userLang); base != "" {
		return base
	}
	if base := Normalize(householdLang); base != "" {
		return base
	}
	if base := parseAcceptLanguage(acceptLanguage); base != "" {
		return base
	}
	return DefaultLanguage
}

func parseAcceptLanguage(header string) string {
	type candidate struct {
		tag string
		q   float64
	}
	var candidates []candidate
	for part := range strings.SplitSeq(header, ",") {
		fields := strings.Split(part, ";")
		tag := strings.TrimSpace(fields[0])
		if tag == "" {
			continue
		}
		q := 1.0
		for _, param := range fields[1:] {
			param = strings.TrimSpace(param)
			if strings.HasPrefix(param, "q=") {
				var parsed float64
				if _, err := fmt.Sscanf(param[2:], "%g", &parsed); err == nil {
					q = parsed
				}
			}
		}
		candidates = append(candidates, candidate{tag: tag, q: q})
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].q > candidates[j].q })
	for _, c := range candidates {
		if c.tag == "*" {
			return DefaultLanguage
		}
		if base := Normalize(c.tag); base != "" {
			return base
		}
	}
	return ""
}

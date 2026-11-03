// reference_doc.go renders the configuration reference and reads the shipped
// configuration templates, so the document and the templates can be compared
// against each other.
package configuration

import (
	"bytes"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

// TemplateFile is a shipped configuration template.
type TemplateFile struct {
	// Name labels the template in the reference, for example "SQLite".
	Name string
	// Content is the raw YAML of the template.
	Content []byte
}

// TemplateValues returns the value each template sets for every key, keyed by
// the lower-cased key path. A key the templates set to different values is
// rendered with every value, labelled by the template it belongs to, so the
// reference shows what each template ships.
func TemplateValues(files []TemplateFile) (map[string]string, error) {
	perTemplate := make([]map[string]string, 0, len(files))
	paths := make(map[string]bool)
	for _, file := range files {
		values, err := ParseConfigValues(file.Content)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file.Name, err)
		}
		perTemplate = append(perTemplate, values)
		for path := range values {
			paths[path] = true
		}
	}

	combined := make(map[string]string, len(paths))
	for path := range paths {
		rendered, err := renderTemplateValue(path, files, perTemplate)
		if err != nil {
			return nil, err
		}
		combined[path] = rendered
	}
	return combined, nil
}

// renderTemplateValue renders the value the templates set for one key.
func renderTemplateValue(path string, files []TemplateFile, perTemplate []map[string]string) (string, error) {
	type templateEntry struct {
		name  string
		value string
	}
	var entries []templateEntry
	for index, values := range perTemplate {
		value, ok := values[path]
		if !ok {
			continue
		}
		entries = append(entries, templateEntry{name: files[index].Name, value: value})
	}
	if len(entries) == 0 {
		return "", nil
	}

	same := true
	for _, candidate := range entries[1:] {
		if candidate.value != entries[0].value {
			same = false
			break
		}
	}
	if same {
		return entries[0].value, nil
	}
	parts := make([]string, 0, len(entries))
	for _, candidate := range entries {
		parts = append(parts, candidate.value+" ("+candidate.name+")")
	}
	return strings.Join(parts, " / "), nil
}

// ParseConfigValues parses a YAML configuration file and returns the value it
// sets for every key, keyed by the lower-cased key path.
func ParseConfigValues(content []byte) (map[string]string, error) {
	parser := viper.New()
	parser.SetConfigType("yaml")
	if err := parser.ReadConfig(bytes.NewReader(content)); err != nil {
		return nil, err
	}
	keys := parser.AllKeys()
	values := make(map[string]string, len(keys))
	for _, key := range keys {
		values[strings.ToLower(key)] = renderConfigValue(parser.Get(key))
	}
	return values, nil
}

// renderConfigValue renders a configuration value for the reference.
func renderConfigValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return "empty"
	case string:
		if typed == "" {
			return "empty"
		}
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case []string:
		return renderConfigList(typed)
	case []any:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			items = append(items, fmt.Sprint(item))
		}
		return renderConfigList(items)
	default:
		return fmt.Sprint(value)
	}
}

// renderConfigList renders a list value for the reference.
func renderConfigList(items []string) string {
	if len(items) == 0 {
		return "empty"
	}
	return strings.Join(items, ", ")
}

// referenceSections is the order the configuration blocks appear in. A block
// that is not listed is appended in alphabetical order.
var referenceSections = []string{
	"server",
	"server.authentication",
	"server.twoFactor",
	"server.cors",
	"server.securityHeaders",
	"server.rateLimit",
	"calendar",
	"database",
	"database.sqlite",
	"database.mariadb",
	"logging",
	"notification",
	"notification.smtp",
	"notification.ntfy",
	"notification.monthlyWasteReport",
	"notification.telegram",
	"notification.mailDigest",
	"openfoodfacts",
	"ocr",
	"ocr.receipt",
	"expiry",
	"recipe_api",
	"shopping_list_sync",
	"backup",
	"restore",
	"offlineSync",
}

// referenceSectionTitles names the configuration blocks for the reference.
var referenceSectionTitles = map[string]string{
	"server":                          "Server",
	"server.authentication":           "Authentication",
	"server.twoFactor":                "Two-factor authentication",
	"server.cors":                     "CORS",
	"server.securityHeaders":          "Security headers",
	"server.rateLimit":                "Rate limits",
	"calendar":                        "Calendar feed",
	"database":                        "Database",
	"database.sqlite":                 "SQLite",
	"database.mariadb":                "MariaDB",
	"logging":                         "Logging",
	"notification":                    "Notifications",
	"notification.smtp":               "SMTP",
	"notification.ntfy":               "Ntfy",
	"notification.monthlyWasteReport": "Monthly waste report",
	"notification.telegram":           "Telegram",
	"notification.mailDigest":         "Mail digest",
	"openfoodfacts":                   "Open Food Facts",
	"ocr":                             "OCR (expiry date scanning)",
	"ocr.receipt":                     "Receipt scanning",
	"expiry":                          "Expiry thresholds",
	"recipe_api":                      "Recipe suggestions",
	"shopping_list_sync":              "Shopping list sync",
	"backup":                          "Scheduled backups",
	"restore":                         "Restore uploads",
	"offlineSync":                     "Queued offline sync",
}

// referencePreamble introduces the reference.
var referencePreamble = []string{
	"# Configuration reference",
	"",
	"Every key the server understands: what it does, what it resolves to when the",
	"configuration file leaves it out, and what the validator rejects.",
	"",
	"The document is generated from the configuration itself. A key that the code",
	"reads but this document does not mention fails the test suite, so the two",
	"cannot drift apart.",
	"",
	"## How a value is resolved",
	"",
	"1. **The environment variable.** The name is the key path with the dots",
	"   replaced by underscores, upper-cased and prefixed with `PROVIANT_`,",
	"   so `server.authentication.tokenPassword` is",
	"   `PROVIANT_SERVER_AUTHENTICATION_TOKENPASSWORD`.",
	"2. **The configuration file.** `config.yaml` in the working directory of the",
	"   process. The shipped templates are `src/config.yaml.sqlite.tmpl` and",
	"   `src/config.yaml.mariadb.tmpl`.",
	"3. **The built-in default.** The **Default** column names it.",
	"",
	"A value in the environment wins over the same key in the file, and a key in",
	"the file wins over the default. An environment variable that is set to an",
	"empty string counts as unset.",
	"",
	"## The columns",
	"",
	"| Column | What it holds |",
	"|---|---|",
	"| Key | The path of the key in the configuration file. |",
	"| Environment variable | The variable that overrides it. |",
	"| Type | The value type: `string`, `int`, `bool` or `list`. |",
	"| Default | The value the key resolves to when the configuration file omits it. |",
	"| Config template | The value the shipped templates set. Where the templates differ, both are named. |",
	"| Constraint | What the validator rejects. A key the server cannot start without says so. |",
	"| Effect | What the key does. |",
	"",
	"## Secrets",
	"",
	"A key whose value is a credential carries a placeholder here and never its",
	"value. The credentials of this repository — the shipped placeholders the",
	"server refuses or warns about — are not part of this document either.",
	"",
	"The Telegram bot token and the VAPID keys are not configuration keys: the",
	"bot token belongs to the account that sets it in the app, and the VAPID keys",
	"are generated into the database on the first run.",
	"",
}

// RenderReference renders the operator reference for the given keys.
func RenderReference(keys []ConfigKey, templates map[string]string) string {
	var out strings.Builder
	for _, line := range referencePreamble {
		out.WriteString(line)
		out.WriteString("\n")
	}

	grouped := groupConfigKeys(keys)
	for _, section := range orderedConfigSections(grouped) {
		title, ok := referenceSectionTitles[section]
		if !ok {
			title = section
		}
		out.WriteString("\n## " + title + "\n\n")
		out.WriteString("| Key | Environment variable | Type | Default | Config template | Constraint | Effect |\n")
		out.WriteString("|---|---|---|---|---|---|---|\n")
		for _, key := range grouped[section] {
			out.WriteString(renderConfigKeyRow(key, templates))
		}
	}
	return out.String()
}

// renderConfigKeyRow renders one row of the reference table.
func renderConfigKeyRow(key *ConfigKey, templates map[string]string) string {
	templateValue := templates[strings.ToLower(key.Path)]
	if key.Secret {
		templateValue = secretTemplateValue(templateValue)
	}
	if templateValue == "" {
		templateValue = "not set"
	}
	constraint := key.Constraint
	if constraint == "" {
		constraint = "—"
	}
	return "| `" + key.Path + "` | `" + key.Env + "` | " + key.Type + " | " + key.Default +
		" | " + templateValue + " | " + constraint + " | " + key.Effect + " |\n"
}

// secretTemplateValue hides the value a template sets for a secret key.
func secretTemplateValue(value string) string {
	if value == "" || value == "empty" {
		return "empty"
	}
	return "a placeholder — replace it"
}

// groupConfigKeys groups the keys by the block they belong to.
func groupConfigKeys(keys []ConfigKey) map[string][]*ConfigKey {
	grouped := make(map[string][]*ConfigKey)
	for index := range keys {
		key := &keys[index]
		section := key.Path
		if separator := strings.LastIndex(key.Path, "."); separator >= 0 {
			section = key.Path[:separator]
		}
		grouped[section] = append(grouped[section], key)
	}
	return grouped
}

// orderedConfigSections returns the blocks in the order the reference lists
// them: the known blocks first, the rest in alphabetical order.
func orderedConfigSections(grouped map[string][]*ConfigKey) []string {
	sections := make([]string, 0, len(grouped))
	seen := make(map[string]bool, len(grouped))
	for _, section := range referenceSections {
		if _, ok := grouped[section]; ok {
			sections = append(sections, section)
			seen[section] = true
		}
	}
	var rest []string
	for section := range grouped {
		if !seen[section] {
			rest = append(rest, section)
		}
	}
	slices.Sort(rest)
	return append(sections, rest...)
}

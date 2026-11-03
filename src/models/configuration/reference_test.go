package configuration

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Isotop7/proviant/util"

	"github.com/spf13/viper"
)

const (
	sqliteTemplatePath  = "../../config.yaml.sqlite.tmpl"
	mariadbTemplatePath = "../../config.yaml.mariadb.tmpl"
	localConfigPath     = "../../config.yaml"
	docsDirectoryPath   = "../../../docs"
	referencePath       = "../../../docs/configuration.md"
	readmePath          = "../../../README.md"
)

// TestEveryConfigKeyIsDocumented fails when a configuration key has no
// documentation entry or when an entry names a key the struct does not have.
// A key the code reads but the reference does not mention would reach an
// operator as a mystery.
func TestEveryConfigKeyIsDocumented(t *testing.T) {
	known := make(map[string]bool)
	for _, key := range ConfigKeys() {
		known[key.Path] = true
		doc, ok := configKeyDocs[key.Path]
		if !ok {
			t.Errorf("%s has no documentation entry", key.Path)
			continue
		}
		if doc.Effect == "" {
			t.Errorf("%s documents no effect", key.Path)
		}
	}
	for path := range configKeyDocs {
		if !known[path] {
			t.Errorf("the documentation names %q, which is no configuration key", path)
		}
	}
}

// TestConfigDefaultsResolveToRealKeys fails when the default table names a key
// the configuration struct does not read. Such a default is dropped by viper
// without a word, which is how the password policy ended up with a minimum
// length of zero.
func TestConfigDefaultsResolveToRealKeys(t *testing.T) {
	known := make(map[string]bool)
	for _, key := range ConfigKeys() {
		known[key.Path] = true
	}
	for path := range Defaults() {
		if !known[path] {
			t.Errorf("the default table names %q, which is no configuration key", path)
		}
	}
}

// TestConfigTemplatesCarryTheSameKeys fails when a key is in one shipped
// template and not in the other. The database subtree is exempt: its two
// engines name different keys.
func TestConfigTemplatesCarryTheSameKeys(t *testing.T) {
	sqlite := templateKeys(t, sqliteTemplatePath)
	mariadb := templateKeys(t, mariadbTemplatePath)

	for path := range sqlite {
		if strings.HasPrefix(path, "database.") {
			continue
		}
		if _, ok := mariadb[path]; !ok {
			t.Errorf("%s sets %q, %s does not", sqliteTemplatePath, path, mariadbTemplatePath)
		}
	}
	for path := range mariadb {
		if strings.HasPrefix(path, "database.") {
			continue
		}
		if _, ok := sqlite[path]; !ok {
			t.Errorf("%s sets %q, %s does not", mariadbTemplatePath, path, sqliteTemplatePath)
		}
	}
}

// TestConfigReferenceCoversEveryTemplateKey fails when a key exists in a
// configuration file but not in the reference.
func TestConfigReferenceCoversEveryTemplateKey(t *testing.T) {
	documented := make(map[string]bool)
	for _, key := range ConfigKeys() {
		documented[strings.ToLower(key.Path)] = true
	}
	for _, path := range []string{sqliteTemplatePath, mariadbTemplatePath, localConfigPath} {
		for key := range templateKeys(t, path) {
			if !documented[key] {
				t.Errorf("%s sets %q, the configuration reference does not document it", path, key)
			}
		}
	}
}

// TestLocalConfigCarriesOnlyRealKeys fails when the local configuration file
// of a developer names a key the server does not read. The file is
// gitignored, so the check only runs where it exists.
func TestLocalConfigCarriesOnlyRealKeys(t *testing.T) {
	if _, err := os.Stat(localConfigPath); err != nil {
		t.Skipf("%s does not exist", localConfigPath)
	}
	documented := make(map[string]bool)
	for _, key := range ConfigKeys() {
		documented[strings.ToLower(key.Path)] = true
	}
	for key := range templateKeys(t, localConfigPath) {
		if !documented[key] {
			t.Errorf("%s sets %q, which is no configuration key", localConfigPath, key)
		}
	}
}

// TestConfigReferenceIsUpToDate fails when the generated reference differs
// from the file on disk. Run task configdoc to bring them together.
func TestConfigReferenceIsUpToDate(t *testing.T) {
	generated := generateReference(t)
	existing, err := os.ReadFile(referencePath)
	if err != nil {
		t.Fatalf("reading %s failed: %v", referencePath, err)
	}
	if string(existing) != generated {
		t.Errorf("%s is out of date — run task configdoc", referencePath)
	}
}

// TestConfigReferenceCarriesNoSecret fails when a credential of this
// repository reaches the operator documentation, or when the reference prints
// the value a template sets for a secret key.
func TestConfigReferenceCarriesNoSecret(t *testing.T) {
	published := []string{
		util.RetiredTokenPassword,
		util.RetiredTwoFactorEncryptionKey,
		util.PublishedMariaDBUserPassword,
		util.PublishedMariaDBRootPassword,
	}
	documents, err := filepath.Glob(filepath.Join(docsDirectoryPath, "*.md"))
	if err != nil {
		t.Fatalf("listing the documentation failed: %v", err)
	}
	for _, document := range documents {
		content, readErr := os.ReadFile(document)
		if readErr != nil {
			t.Fatalf("reading %s failed: %v", document, readErr)
		}
		for _, credential := range published {
			if strings.Contains(string(content), credential) {
				t.Errorf("%s carries a published credential", document)
			}
		}
	}

	// The README names the compose file's database password defaults on
	// purpose: the operator has to recognise them to replace them. The two
	// retired placeholders are named nowhere — a copied configuration file may
	// still carry them, and the server refuses to start.
	readme, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("reading %s failed: %v", readmePath, err)
	}
	for _, credential := range []string{util.RetiredTokenPassword, util.RetiredTwoFactorEncryptionKey} {
		if strings.Contains(string(readme), credential) {
			t.Errorf("%s carries a published credential", readmePath)
		}
	}

	templates := templateValues(t)
	for _, key := range ConfigKeys() {
		if !key.Secret {
			continue
		}
		switch cell := referenceTemplateCell(t, &key, templates); cell {
		case "a placeholder — replace it", "empty", "not set":
		default:
			t.Errorf("the reference prints the value of the secret %s: %s", key.Path, cell)
		}
	}
}

// referenceTemplateCell returns the cell of the reference row that holds the
// value the shipped templates set. It is the only cell a template value can
// reach, so asserting on it is what keeps a secret out of the document —
// searching the whole row would also match the default cell and pass even
// when the template cell leaks.
func referenceTemplateCell(t *testing.T, key *ConfigKey, templates map[string]string) string {
	t.Helper()
	row := renderConfigKeyRow(key, templates)
	cells := strings.SplitN(row, "|", 7)
	if len(cells) < 6 {
		t.Fatalf("the reference row of %s has %d cells: %s", key.Path, len(cells), row)
	}
	return strings.TrimSpace(cells[5])
}

// TestConfigTemplatesShipTheRejectedPlaceholders fails when a template ships a
// placeholder the server no longer refuses. The startup check compares the
// value against a constant in util, so a template that drifts away from it
// hands the operator a publicly known key that the server accepts: the
// two-factor encryption key would decrypt every enrolled second factor.
func TestConfigTemplatesShipTheRejectedPlaceholders(t *testing.T) {
	for _, template := range []struct {
		name string
		path string
	}{
		{"SQLite", sqliteTemplatePath},
		{"MariaDB", mariadbTemplatePath},
	} {
		t.Run(template.name, func(t *testing.T) {
			values := templateKeys(t, template.path)
			twoFactor := values["server.twofactor.encryptionkey"]
			if twoFactor != util.RetiredTwoFactorEncryptionKey {
				t.Errorf("%s ships %q as server.twoFactor.encryptionKey, want the placeholder the server refuses (%q)",
					template.path, twoFactor, util.RetiredTwoFactorEncryptionKey)
			}
			if token := values["server.authentication.tokenpassword"]; token == util.RetiredTokenPassword {
				t.Errorf("%s ships the retired token password", template.path)
			}
		})
	}
}

// TestDocumentedEnvNamesReachTheStruct fails when the environment variable the
// reference names does not reach the configuration struct. viper only honours
// an environment variable for a key it knows, so a default has to be
// registered under the key path of the struct.
func TestDocumentedEnvNamesReachTheStruct(t *testing.T) {
	for _, key := range ConfigKeys() {
		if key.Type == "list" {
			continue
		}
		t.Run(key.Path, func(t *testing.T) {
			probe := "probe-" + strings.ReplaceAll(key.Path, ".", "-")
			if key.Type == "int" {
				probe = "42"
			}
			if key.Type == "bool" {
				probe = "true"
			}
			t.Setenv(key.Env, probe)

			parser := viper.New()
			parser.SetEnvPrefix("PROVIANT")
			parser.AutomaticEnv()
			parser.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
			for path, value := range Defaults() {
				parser.SetDefault(path, value)
			}
			var config ProviantConfiguration
			if err := parser.Unmarshal(&config); err != nil {
				t.Fatalf("unmarshalling the configuration failed: %v", err)
			}

			got, ok := configValueAt(&config, key.Path)
			if !ok {
				t.Fatalf("the configuration struct has no field for %s", key.Path)
			}
			if renderConfigValue(got) != renderConfigValue(probe) {
				t.Errorf("%s = %v, want %v", key.Env, got, probe)
			}
		})
	}
}

// generateReference renders the configuration reference the way the generator
// does.
func generateReference(t *testing.T) string {
	t.Helper()
	return RenderReference(ConfigKeys(), templateValues(t))
}

// templateValues returns the values the shipped configuration templates set.
func templateValues(t *testing.T) map[string]string {
	t.Helper()
	files := make([]TemplateFile, 0, 2)
	for _, template := range []struct {
		name string
		path string
	}{
		{"SQLite", sqliteTemplatePath},
		{"MariaDB", mariadbTemplatePath},
	} {
		content, err := os.ReadFile(template.path)
		if err != nil {
			t.Fatalf("reading %s failed: %v", template.path, err)
		}
		files = append(files, TemplateFile{Name: template.name, Content: content})
	}
	values, err := TemplateValues(files)
	if err != nil {
		t.Fatalf("reading the configuration templates failed: %v", err)
	}
	return values
}

// templateKeys returns the lower-cased key paths a configuration file sets.
func templateKeys(t *testing.T, path string) map[string]string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}
		}
		t.Fatalf("reading %s failed: %v", path, err)
	}
	values, err := ParseConfigValues(content)
	if err != nil {
		t.Fatalf("parsing %s failed: %v", path, err)
	}
	return values
}

// configValueAt returns the value a configuration struct holds at a key path.
func configValueAt(config *ProviantConfiguration, path string) (any, bool) {
	current := reflect.ValueOf(config).Elem()
	for segment := range strings.SplitSeq(path, ".") {
		field, ok := configFieldByKey(current, segment)
		if !ok {
			return nil, false
		}
		current = field
	}
	return current.Interface(), true
}

// configFieldByKey returns the struct field a configuration key is stored in.
func configFieldByKey(structValue reflect.Value, key string) (reflect.Value, bool) {
	structType := structValue.Type()
	for i := range structType.NumField() {
		field := structType.Field(i)
		name, _, _ := strings.Cut(field.Tag.Get("mapstructure"), ",")
		if name == "" {
			name = strings.ToLower(field.Name[:1]) + field.Name[1:]
		}
		if strings.EqualFold(name, key) {
			return structValue.Field(i), true
		}
	}
	return reflect.Value{}, false
}

// TestConfigEnvNameFollowsTheViperRule pins the environment variable naming
// rule: the key path with the dots replaced by underscores, upper-cased and
// prefixed.
func TestConfigEnvNameFollowsTheViperRule(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"server.port", "PROVIANT_SERVER_PORT"},
		{"server.authentication.tokenPassword", "PROVIANT_SERVER_AUTHENTICATION_TOKENPASSWORD"},
		{"server.authentication.max_login_attempts", "PROVIANT_SERVER_AUTHENTICATION_MAX_LOGIN_ATTEMPTS"},
		{"server.securityHeaders.csrf_token_max_age", "PROVIANT_SERVER_SECURITYHEADERS_CSRF_TOKEN_MAX_AGE"},
		{"openfoodfacts.dumpUrl", "PROVIANT_OPENFOODFACTS_DUMPURL"},
		{"recipe_api.api_key", "PROVIANT_RECIPE_API_API_KEY"},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			if got := configEnvName(test.path); got != test.want {
				t.Errorf("configEnvName(%q) = %q, want %q", test.path, got, test.want)
			}
		})
	}
}

// TestReferenceSectionsCoverEveryKey fails when a configuration key lands in
// no block of the reference, which would hide it from the reader.
func TestReferenceSectionsCoverEveryKey(t *testing.T) {
	grouped := groupConfigKeys(ConfigKeys())
	for _, key := range ConfigKeys() {
		section := key.Path
		if separator := strings.LastIndex(key.Path, "."); separator >= 0 {
			section = key.Path[:separator]
		}
		if _, ok := grouped[section]; !ok {
			t.Errorf("%s belongs to no block of the reference", key.Path)
		}
	}
	if slices.Contains(referenceSections, "") {
		t.Error("the reference names an empty block")
	}
}

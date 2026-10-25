package templates

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"github.com/Isotop7/proviant/i18n"
	"github.com/Isotop7/proviant/models/configuration"
	"github.com/Isotop7/proviant/models/configuration/static"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/util"
)

//go:embed all:web "notification"
var TemplateFiles embed.FS

var (
	criticalThresholdDays = 3
	soonThresholdDays     = 7
)

// SetExpiryThresholds updates the package-level thresholds used by expiry* functions.
// Must be called before NewTemplateCache so the baked-in funcs use the correct values.
func SetExpiryThresholds(critical, soon int) {
	criticalThresholdDays = critical
	soonThresholdDays = soon
}

func humanDateTime(lang string, t time.Time) string {
	return t.Format(i18n.T(lang, "date.format.dateTime"))
}

func humanDateTimeFromSQL(lang string, t gorm.DeletedAt) string {
	return t.Time.Format(i18n.T(lang, "date.format.dateTime"))
}

func humanDate(lang string, t time.Time) string {
	return t.Format(i18n.T(lang, "date.format.date"))
}

func inputDate(t time.Time) string {
	return t.Format(util.DefaultDateFormatParseStr)
}

func today() string {
	return time.Now().Format(util.DefaultDateFormatParseStr)
}

func now() string {
	return time.Now().Format("2006-01-02 15:04")
}

func hasPassed(t time.Time) bool {
	return t.Before(time.Now())
}

func expiryStatusClass(t time.Time) string {
	if t.IsZero() {
		return "neutral"
	}
	now := time.Now()
	if t.Before(now) {
		return "expired"
	}
	if t.Before(now.Add(time.Duration(criticalThresholdDays) * 24 * time.Hour)) {
		return "critical"
	}
	if t.Before(now.Add(time.Duration(soonThresholdDays) * 24 * time.Hour)) {
		return "soon"
	}
	return "fresh"
}

func expiryStatusIcon(t time.Time) string {
	switch expiryStatusClass(t) {
	case "expired":
		return "x-circle-fill"
	case "critical":
		return "exclamation-circle-fill"
	case "soon":
		return "clock-fill"
	case "fresh":
		return "check-circle-fill"
	default:
		return "dash-circle-fill"
	}
}

func expiryStatusLabel(lang string, t time.Time) string {
	switch expiryStatusClass(t) {
	case "expired":
		return i18n.T(lang, "status.expired")
	case "critical":
		return i18n.T(lang, "status.critical")
	case "soon":
		return i18n.T(lang, "status.soon")
	case "fresh":
		return i18n.T(lang, "status.fresh")
	default:
		return i18n.T(lang, "status.noDate")
	}
}

func queryWith(params url.Values, key, val string) template.URL {
	p := url.Values{}
	for k, v := range params {
		p[k] = v
	}
	if val == "" {
		p.Del(key)
	} else {
		p.Set(key, val)
	}
	return template.URL(p.Encode())
}

func stringSlice(vals ...string) []string {
	return vals
}

func expiryUrgencyText(lang string, t time.Time) string {
	if t.IsZero() {
		return ""
	}
	now := time.Now()
	if t.Before(now) {
		days := int(now.Sub(t).Hours() / 24)
		if days == 0 {
			return i18n.T(lang, "status.expiredToday")
		}
		return i18n.T(lang, "status.expiredDaysAgo", days)
	}
	days := int(t.Sub(now).Hours()/24) + 1
	if days == 1 {
		return i18n.T(lang, "status.expiresTomorrow")
	}
	if days <= 7 {
		return i18n.T(lang, "status.expiresInDays", days)
	}
	return ""
}

// expiryDays returns the signed number of days until expiry (negative = expired).
func expiryDays(t time.Time) int {
	if t.IsZero() {
		return 0
	}
	now := time.Now()
	if t.Before(now) {
		return -int(now.Sub(t).Hours()/24) - 1 // -1 means "as of yesterday"
	}
	return int(t.Sub(now).Hours()/24) + 1
}

func badgifyCategories(categories string, limit int) template.HTML {
	var output strings.Builder
	elements := strings.Split(categories, ",")
	for idx, elem := range elements {
		contents := strings.Split(strings.TrimSpace(elem), ":")
		if idx == limit {
			break
		}
		if len(contents) == 2 {
			lang := contents[0]
			definition := contents[1]
			output.WriteString("<span class=\"badge bg-dark me-3\">")
			output.WriteString(lang)
			output.WriteString("</span>")
			output.WriteString(definition)
			output.WriteString("<br>")
		} else {
			output.WriteString(elem)
			output.WriteString("<br>")
		}
	}
	return template.HTML(output.String())
}

func splitString(source string) template.HTML {
	var output strings.Builder
	elements := strings.Split(source, ",")
	for _, elem := range elements {
		output.WriteString(elem)
		output.WriteString("<br>")
	}
	return template.HTML(output.String())
}

var flagMap = map[string]string{
	"ad": "🇦🇩", "ae": "🇦🇪", "af": "🇦🇫", "ag": "🇦🇬", "ai": "🇦🇮", "al": "🇦🇱", "am": "🇦🇲", "ao": "🇦🇴", "aq": "🇦🇶", "ar": "🇦🇷",
	"as": "🇦🇸", "at": "🇦🇹", "au": "🇦🇺", "aw": "🇦🇼", "ax": "🇦🇽", "az": "🇦🇿", "ba": "🇧🇦", "bb": "🇧🇧", "bd": "🇧🇩", "be": "🇧🇪",
	"bf": "🇧🇫", "bg": "🇧🇬", "bh": "🇧🇭", "bi": "🇧🇮", "bj": "🇧🇯", "bl": "🇧🇱", "bm": "🇧🇲", "bn": "🇧🇳", "bo": "🇧🇴", "bq": "🇧🇶",
	"br": "🇧🇷", "bs": "🇧🇸", "bt": "🇧🇹", "bv": "🇧🇻", "bw": "🇧🇼", "by": "🇧🇾", "bz": "🇧🇿", "ca": "🇨🇦", "cc": "🇨🇨", "cd": "🇨🇩",
	"cf": "🇨🇫", "cg": "🇨🇬", "ch": "🇨🇭", "ci": "🇨🇮", "ck": "🇨🇰", "cl": "🇨🇱", "cm": "🇨🇲", "cn": "🇨🇳", "co": "🇨🇴", "cr": "🇨🇷",
	"cu": "🇨🇺", "cv": "🇨🇻", "cw": "🇨🇼", "cx": "🇨🇽", "cy": "🇨🇾", "cz": "🇨🇿", "de": "🇩🇪", "dj": "🇩🇯", "dk": "🇩🇰", "dm": "🇩🇲",
	"do": "🇩🇴", "dz": "🇩🇿", "ec": "🇪🇨", "ee": "🇪🇪", "eg": "🇪🇬", "eh": "🇪🇭", "er": "🇪🇷", "es": "🇪🇸", "et": "🇪🇹", "fi": "🇫🇮",
	"fj": "🇫🇯", "fk": "🇫🇰", "fm": "🇫🇲", "fo": "🇫", "fr": "🇫🇷", "ga": "🇬🇦", "gb": "🇬🇧", "gd": "🇬🇩", "ge": "🇬🇪", "gf": "🇬🇫",
	"gg": "🇬🇬", "gh": "🇬🇭", "gi": "🇬🇮", "gl": "🇬🇱", "gm": "🇬🇲", "gn": "🇬🇳", "gp": "🇬🇵", "gq": "🇬🇶", "gr": "🇬🇷", "gs": "🇬🇸",
	"gt": "🇬🇹", "gu": "🇬🇺", "gw": "🇬🇼", "gy": "🇬🇾", "hk": "🇭🇰", "hm": "🇭🇲", "hn": "🇭🇳", "hr": "🇭🇷", "ht": "🇭🇹", "hu": "🇭🇺",
	"id": "🇮🇩", "ie": "🇮🇪", "il": "🇮🇱", "im": "🇮🇲", "in": "🇮🇳", "io": "🇮🇴", "iq": "🇮🇶", "ir": "🇮🇷", "is": "🇮🇸", "it": "🇮🇹",
	"je": "🇯🇪", "jm": "🇯🇲", "jo": "🇯🇴", "jp": "🇯🇵", "ke": "🇰🇪", "kg": "🇰🇬", "kh": "🇰🇭", "ki": "🇰🇮", "km": "🇰🇲", "kn": "🇰🇳",
	"kp": "🇰🇵", "kr": "🇰🇷", "kw": "🇰🇼", "ky": "🇰🇾", "kz": "🇰🇿", "la": "🇱🇦", "lb": "🇱🇧", "lc": "🇱🇨", "li": "🇱🇮", "lk": "🇱🇰",
	"lr": "🇱🇷", "ls": "🇱🇸", "lt": "🇱🇹", "lu": "🇱🇺", "lv": "🇱🇻", "ly": "🇱🇾", "ma": "🇲🇦", "mc": "🇲🇨", "md": "🇲🇩", "me": "🇲🇪",
	"mf": "🇲🇫", "mg": "🇲🇬", "mh": "🇲🇭", "mk": "🇲🇰", "ml": "🇲🇱", "mm": "🇲🇲", "mn": "🇲🇳", "mo": "🇲🇴", "mp": "🇲🇵", "mq": "🇲🇶",
	"mr": "🇲🇷", "ms": "🇲🇸", "mt": "🇲🇹", "mu": "🇲🇺", "mv": "🇲🇻", "mw": "🇲🇼", "mx": "🇲🇽", "my": "🇲🇾", "mz": "🇲🇿", "na": "🇳🇦",
	"nc": "🇳🇨", "ne": "🇳🇪", "nf": "🇳🇫", "ng": "🇳🇬", "ni": "🇳🇮", "nl": "🇳🇱", "no": "🇳", "np": "🇳🇵", "nr": "🇳🇷", "nu": "🇳🇺",
	"nz": "🇳🇿", "om": "🇴🇲", "pa": "🇵🇦", "pe": "🇵🇪", "pf": "🇵🇫", "pg": "🇵🇬", "ph": "🇵🇭", "pk": "🇵🇰", "pl": "🇵🇱", "pm": "🇵🇲",
	"pn": "🇵🇳", "pr": "🇵🇷", "ps": "🇵🇸", "pt": "🇵🇹", "pw": "🇵🇼", "py": "🇵🇾", "qa": "🇶🇦", "re": "🇷🇪", "ro": "🇷🇴", "rs": "🇷🇸",
	"ru": "🇷", "rw": "🇷🇼", "sa": "🇸🇦", "sb": "🇸🇧", "sc": "🇸🇨", "sd": "🇸🇩", "se": "🇸🇪", "sg": "🇸🇬", "sh": "🇸🇭", "si": "🇸🇮",
	"sj": "🇸🇯", "sk": "🇸🇰", "sl": "🇸🇱", "sm": "🇸🇲", "sn": "🇸🇳", "so": "🇸🇴", "sr": "🇸🇷", "ss": "🇸🇸", "st": "🇸🇹", "sv": "🇸🇻",
	"sx": "🇸🇽", "sy": "🇸🇾", "sz": "🇸🇿", "tc": "🇹🇨", "td": "🇹🇩", "tf": "🇹🇫", "tg": "🇹🇬", "th": "🇹🇭", "tj": "🇹🇯", "tk": "🇹🇰",
	"tl": "🇹🇱", "tm": "🇹🇲", "tn": "🇹🇳", "to": "🇹", "tr": "🇹🇷", "tt": "🇹🇹", "tv": "🇹🇻", "tw": "🇹🇼", "tz": "🇹🇿", "ua": "🇺🇦",
	"ug": "🇺🇬", "um": "🇺🇲", "us": "🇺🇸", "uy": "🇺🇾", "uz": "🇺🇿", "va": "🇻🇦", "vc": "🇻🇨", "ve": "🇻🇪", "vg": "🇻🇬", "vi": "🇻🇮",
	"vn": "🇻🇳", "vu": "🇻🇺", "wf": "🇼🇫", "ws": "🇼🇸", "ye": "🇾🇪", "yt": "🇾🇹", "za": "🇿🇦", "zm": "🇿🇲", "zw": "🇿🇼",
	"en":       "🏴󠁧󠁢󠁥󠁮󠁧󠁿",
	"wales":    "🏴󠁧󠁢󠁷󠁬󠁳󠁿",
	"scotland": "🏴󠁧󠁢󠁳󠁣󠁴󠁿",
}

func emojifyFlag(source string) string {
	var output strings.Builder
	sourceParts := strings.Split(source, ":")
	// If string cannot be split into two parts, return the original string
	if len(sourceParts) < 2 {
		return source
	}
	// Get the flag from the hash map
	flag := flagMap[strings.ToLower(sourceParts[0])]
	// If no flag was found in hash map, use the language code as a fallback
	if flag == "" {
		flag = sourceParts[0]
	}
	// Build the output string
	output.WriteString(flag)
	output.WriteString(":")
	output.WriteString(sourceParts[1])
	output.WriteString("<br>")
	return output.String()
}

func derefUint(p *uint) uint {
	if p == nil {
		return 0
	}
	return *p
}

// effectiveExpireAt returns the effective expiry date for a product
// (earlier of printed ExpireAt and OpenedAt + DaysAfterOpening days).
// Used by templates to drive status badges and urgency text so the UI
// reflects the earlier of the two dates.
func effectiveExpireAt(p *dbModel.Product) time.Time {
	return p.EffectiveExpireAt()
}

// QuantityLabel renders a pack count the way every quantity surface does: the
// count and the localized pack noun ("2 Stk."). It carries the language
// because only the label needs it, and it lives here so the web handler and
// the templates share one composition of the display rule.
func QuantityLabel(lang string, quantity int) string {
	return fmt.Sprintf("%d %s", quantity, i18n.T(lang, "common.qty"))
}

func flagReplace(source string) template.HTML {
	var output strings.Builder
	for elements := range strings.SplitSeq(source, ",") {
		output.WriteString(emojifyFlag(elements))
	}
	return template.HTML(output.String())
}

func templateFunctions(lang string) template.FuncMap {
	functions := template.FuncMap{
		"humanDate":            func(t time.Time) string { return humanDate(lang, t) },
		"humanDateTime":        func(t time.Time) string { return humanDateTime(lang, t) },
		"humanDateTimeFromSQL": func(t gorm.DeletedAt) string { return humanDateTimeFromSQL(lang, t) },
		"inputDate":            inputDate,
		"now":                  now,
		"today":                today,
		"hasPassed":            hasPassed,
		"expiryStatusClass":    expiryStatusClass,
		"expiryStatusIcon":     expiryStatusIcon,
		"expiryStatusLabel":    func(t time.Time) string { return expiryStatusLabel(lang, t) },
		"expiryUrgencyText":    func(t time.Time) string { return expiryUrgencyText(lang, t) },
		"expiryDays":           expiryDays,
		"queryWith":            queryWith,
		"stringSlice":          stringSlice,
		"badgifyCategories":    badgifyCategories,
		"splitString":          splitString,
		"flagReplace":          flagReplace,
		"derefUint":            derefUint,
		"effectiveExpireAt":    effectiveExpireAt,
		"quantityLabel": func(quantity int) string {
			return QuantityLabel(lang, quantity)
		},
	}
	for name, fn := range i18n.FuncMap(lang) {
		functions[name] = fn
	}
	return functions
}

// templateCacheKey names the per-language template set for a page.
func templateCacheKey(lang, page string) string {
	return lang + "/" + page
}

// lookupTemplate finds the page template set for lang with fallback to the
// default language and finally to an unqualified key (hand-built test caches).
func lookupTemplate(cache map[string]*template.Template, lang, page string) (*template.Template, bool) {
	if ts, ok := cache[templateCacheKey(lang, page)]; ok {
		return ts, true
	}
	if ts, ok := cache[templateCacheKey(i18n.DefaultLanguage, page)]; ok {
		return ts, true
	}
	ts, ok := cache[page]
	return ts, ok
}

func NewTemplateCache() (map[string]*template.Template, error) {
	cache := map[string]*template.Template{}

	pages, err := fs.Glob(TemplateFiles, "web/pages/*.tmpl")
	if err != nil {
		return nil, err
	}

	for _, lang := range i18n.Languages() {
		for _, page := range pages {
			name := filepath.Base(page)

			patterns := []string{
				"web/layout/base.tmpl",
				"web/layout/baseAuth.tmpl",
				"web/partials/*.tmpl",
				page,
			}

			ts, err := template.New(name).Funcs(templateFunctions(lang)).ParseFS(TemplateFiles, patterns...)
			if err != nil {
				return nil, err
			}

			cache[templateCacheKey(lang, name)] = ts
		}
	}

	// Return the map.
	return cache, nil
}

// requestLanguage resolves the UI language for a render: an explicit Lang in
// the page data wins, then the language resolved by middleware, then the
// Accept-Language header.
func requestLanguage(ctx *gin.Context, data map[string]any) string {
	var fromData, fromContext string
	if lang, ok := data["Lang"].(string); ok {
		fromData = lang
	}
	if lang, ok := ctx.Get(util.ContextKeyLanguage); ok {
		if langString, ok := lang.(string); ok {
			fromContext = langString
		}
	}
	acceptLanguage := ""
	if ctx.Request != nil {
		acceptLanguage = ctx.GetHeader("Accept-Language")
	}
	return i18n.ResolveLanguage(fromData, fromContext, acceptLanguage)
}

// injectI18nData fills the language, frontend catalog and translated page
// title into the render data.
func injectI18nData(ctx *gin.Context, data map[string]any) string {
	lang := requestLanguage(ctx, data)
	data["Lang"] = lang
	data["I18nJSON"] = template.JS(i18n.JSON(lang))
	// The outbox client splits its batches at the configured
	// sync body cap, so the value the endpoint enforces
	// reaches it with every page.
	maxSyncBodyBytes := util.ReceiptBulkMaxBodyBytes
	if cfgVal, ok := ctx.Get(util.ContextKeyProviantConfig); ok {
		if proviantConfig, isConfig := cfgVal.(*configuration.ProviantConfiguration); isConfig && proviantConfig != nil && proviantConfig.OfflineSync.MaxBatchBytes > 0 {
			maxSyncBodyBytes = proviantConfig.OfflineSync.MaxBatchBytes
		}
	}
	data["OfflineSyncMaxBodyBytes"] = maxSyncBodyBytes
	// The bounds a product may hold reach the client with every page, so the
	// receipt review clamps to the same values the server enforces.
	data["MaxProductQuantity"] = util.MaxProductQuantity
	data["MaxPackSizeLength"] = util.MaxPackSizeLength
	// The window in which the server still deduplicates a replayed stock
	// mutation. The outbox learns it so it can refuse to replay a queued
	// change the server would no longer recognize.
	data["MutationClaimTTLSeconds"] = int(util.ProductMutationClaimTTL.Seconds())
	// The page title and the nav key are two different things. The edit
	// page keeps "Products" as its nav key so the sidebar stays lit, while
	// its title is "Edit Product" — PageTitle carries the title, Title the
	// nav key.
	pageTitle, _ := data["PageTitle"].(string)
	if pageTitle == "" {
		pageTitle, _ = data["Title"].(string)
	}
	if pageTitle != "" {
		key := "pageTitle." + strings.ReplaceAll(pageTitle, " ", "")
		if i18n.Has(lang, key) {
			data["TitleText"] = i18n.T(lang, key)
		} else {
			data["TitleText"] = pageTitle
		}
	}
	return lang
}

func Render(ctx *gin.Context, tc map[string]*template.Template, status int, base, page string, data map[string]any) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

	writer := ctx.Writer
	lang := injectI18nData(ctx, data)
	ts, ok := lookupTemplate(tc, lang, page)
	if !ok {
		mapErr := errors.New("error getting template")
		logger.Error().Msg(mapErr.Error())
		RenderError(ctx, tc, http.StatusInternalServerError, mapErr.Error())
		return
	}

	// Inject CSP nonce if available and not already set
	if _, exists := data["CSPNonce"]; !exists {
		if nonce, ok := ctx.Get(util.ContextKeyCSPNonce); ok {
			data["CSPNonce"] = nonce
		}
	}

	// Inject Version for navbar link
	data["Version"] = static.Version

	// Write parsed template to temporary buffer
	buf := new(bytes.Buffer)

	// Check for errors
	err := ts.ExecuteTemplate(buf, base, data)
	if err != nil {
		logger.Error().Msg(err.Error())
		RenderError(ctx, tc, http.StatusInternalServerError, err.Error())
		return
	}

	// On success, set header and serve template
	writer.WriteHeader(status)
	_, writeErr := buf.WriteTo(writer)
	if writeErr != nil {
		logger.Error().Msg(writeErr.Error())
		RenderError(ctx, tc, http.StatusInternalServerError, writeErr.Error())
		return
	}
}

func RenderError(ctx *gin.Context, tc map[string]*template.Template, code int, message string) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

	writer := ctx.Writer
	data := map[string]any{
		"Code":    code,
		"Message": message,
	}
	lang := injectI18nData(ctx, data)
	data["Title"] = fmt.Sprintf("Error %d", code)
	data["TitleText"] = i18n.T(lang, "error.title", code)

	ts, ok := lookupTemplate(tc, lang, "error.tmpl")
	if !ok {
		mapErr := errors.New("error getting error template")
		logger.Error().Msg(mapErr.Error())
		abortErr := ctx.AbortWithError(http.StatusInternalServerError, mapErr)
		if abortErr != nil {
			logger.Error().Msg(abortErr.Error())
		}
		return
	}

	// Create temporary buffer for content
	buf := new(bytes.Buffer)

	// Inject CSP nonce if available
	if nonce, ok := ctx.Get(util.ContextKeyCSPNonce); ok {
		data["CSPNonce"] = nonce
	}

	// Check for errors
	tmplErr := ts.Execute(buf, data)
	if tmplErr != nil {
		logger.Error().Msg(tmplErr.Error())
		abortErr := ctx.AbortWithError(http.StatusInternalServerError, tmplErr)
		if abortErr != nil {
			logger.Error().Msg(abortErr.Error())
		}
		return
	}

	// On success, set header and serve template
	writer.WriteHeader(code)
	_, writeErr := buf.WriteTo(writer)
	if writeErr != nil {
		logger.Error().Msg(writeErr.Error())
		abortErr := ctx.AbortWithError(http.StatusInternalServerError, writeErr)
		if abortErr != nil {
			logger.Error().Msg(abortErr.Error())
		}
		return
	}
}

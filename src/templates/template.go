package templates

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

//go:embed "web" "notification"
var TemplateFiles embed.FS

func humanDateTime(t time.Time) string {
	return t.Format("02.01.2006, 15:04")
}

func humanDateTimeFromSQL(t gorm.DeletedAt) string {
	return t.Time.Format("02.01.2006, 15:04")
}

func humanDate(t time.Time) string {
	return t.Format("02.01.2006")
}

func inputDate(t time.Time) string {
	return t.Format("2006-01-02")
}

func today() string {
	return time.Now().Format("2006-01-02")
}

func hasPassed(t time.Time) bool {
	return t.Before(time.Now())
}

func expiryBadgeClass(t time.Time) string {
	if t.IsZero() {
		return "bg-secondary"
	}
	now := time.Now()
	if t.Before(now) {
		return "bg-danger"
	}
	if t.Before(now.Add(3 * 24 * time.Hour)) {
		return "bg-danger"
	}
	if t.Before(now.Add(7 * 24 * time.Hour)) {
		return "bg-warning"
	}
	return "bg-success"
}

func expiryStatusClass(t time.Time) string {
	if t.IsZero() {
		return "nodate"
	}
	now := time.Now()
	if t.Before(now) {
		return "expired"
	}
	if t.Before(now.Add(3 * 24 * time.Hour)) {
		return "critical"
	}
	if t.Before(now.Add(7 * 24 * time.Hour)) {
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

func expiryStatusLabel(t time.Time) string {
	switch expiryStatusClass(t) {
	case "expired":
		return "Expired"
	case "critical":
		return "Critical"
	case "soon":
		return "Expiring soon"
	case "fresh":
		return "Fresh"
	default:
		return "No date"
	}
}

func expiryTextClass(t time.Time) string {
	if t.IsZero() {
		return "text-secondary"
	}
	now := time.Now()
	if t.Before(now.Add(3 * 24 * time.Hour)) {
		return "text-danger"
	}
	return "text-warning"
}

func expiryUrgencyText(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	now := time.Now()
	if t.Before(now) {
		days := int(now.Sub(t).Hours() / 24)
		if days == 0 {
			return "Expired today"
		}
		return fmt.Sprintf("Expired %d day(s) ago", days)
	}
	days := int(t.Sub(now).Hours()/24) + 1
	if days == 1 {
		return "Expires tomorrow"
	}
	if days <= 7 {
		return fmt.Sprintf("Expires in %d days", days)
	}
	return ""
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

func flagReplace(source string) template.HTML {
	var output strings.Builder
	for elements := range strings.SplitSeq(source, ",") {
		output.WriteString(emojifyFlag(elements))
	}
	return template.HTML(output.String())
}

var customTemplateFunctions = template.FuncMap{
	"humanDate":            humanDate,
	"humanDateTime":        humanDateTime,
	"humanDateTimeFromSQL": humanDateTimeFromSQL,
	"inputDate":            inputDate,
	"today":                today,
	"hasPassed":            hasPassed,
	"expiryBadgeClass":     expiryBadgeClass,
	"expiryStatusClass":    expiryStatusClass,
	"expiryStatusIcon":     expiryStatusIcon,
	"expiryStatusLabel":    expiryStatusLabel,
	"expiryUrgencyText":    expiryUrgencyText,
	"expiryTextClass":      expiryTextClass,
	"badgifyCategories":    badgifyCategories,
	"splitString":          splitString,
	"flagReplace":          flagReplace,
	"derefUint":            derefUint,
}

func NewTemplateCache() (map[string]*template.Template, error) {
	cache := map[string]*template.Template{}

	pages, err := fs.Glob(TemplateFiles, "web/pages/*.tmpl")
	if err != nil {
		return nil, err
	}

	for _, page := range pages {
		name := filepath.Base(page)

		patterns := []string{
			"web/layout/base.tmpl",
			"web/layout/baseAuth.tmpl",
			"web/partials/*.tmpl",
			page,
		}

		ts, err := template.New(name).Funcs(customTemplateFunctions).ParseFS(TemplateFiles, patterns...)
		if err != nil {
			return nil, err
		}

		cache[name] = ts
	}

	// Return the map.
	return cache, nil
}

func Render(ctx *gin.Context, tc map[string]*template.Template, status int, base, page string, data map[string]any) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	writer := ctx.Writer
	ts, ok := tc[page]
	if !ok {
		mapErr := errors.New("error getting template")
		logger.Error().Msg(mapErr.Error())
		RenderError(ctx, tc, http.StatusInternalServerError, mapErr.Error())
		return
	}

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
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	writer := ctx.Writer
	ts, ok := tc["error.tmpl"]
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

	// Create data map
	data := map[string]any{
		"Title":   fmt.Sprintf("Error %d", code),
		"Code":    code,
		"Message": message,
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

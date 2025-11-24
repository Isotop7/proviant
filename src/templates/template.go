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

func badgifyCategories(categories string, limit int) template.HTML {
	output := ""
	elements := strings.Split(categories, ",")
	for idx, elem := range elements {
		contents := strings.Split(strings.TrimSpace(elem), ":")
		if idx == limit {
			break
		}
		if len(contents) == 2 {
			lang := contents[0]
			definition := contents[1]
			output += fmt.Sprintf("<span class=\"badge bg-dark me-3\">%s</span>%s</br>", lang, definition)
		} else {
			output += fmt.Sprintf("%s</br>", elem)
		}
	}
	return template.HTML(output)
}

func splitString(source string) template.HTML {
	output := ""
	elements := strings.Split(source, ",")
	for _, elem := range elements {
		output += fmt.Sprintf("%s</br>", elem)
	}
	return template.HTML(output)
}

var flagMap = map[string]string{
	"en": "🏴󠁧󠁢󠁥󠁮󠁧󠁿",
	"de": "🇩🇪",
	"fr": "🇫🇷",
	"es": "🇪🇸",
	"it": "🇮🇹",
	"pt": "🇵🇹",
	"ja": "🇯🇵",
	"zh": "🇨🇳",
	"ko": "🇰🇷",
	"nl": "🇳🇱",
}

func emojifyFlag(source string) string {
	sourceParts := strings.Split(source, ":")
	flag := flagMap[strings.ToLower(sourceParts[0])]
	// If no flag was found in hash map, use the language code as a fallback
	if flag == "" {
		flag = sourceParts[0]
	}
	return fmt.Sprintf("%s:%s</br>", flag, sourceParts[1])
}

func flagReplace(source string) template.HTML {
	output := ""
	for elements := range strings.SplitSeq(source, ",") {
		output += emojifyFlag(elements)
	}
	return template.HTML(output)
}

var customTemplateFunctions = template.FuncMap{
	"humanDate":            humanDate,
	"humanDateTime":        humanDateTime,
	"humanDateTimeFromSQL": humanDateTimeFromSQL,
	"inputDate":            inputDate,
	"today":                today,
	"hasPassed":            hasPassed,
	"badgifyCategories":    badgifyCategories,
	"splitString":          splitString,
	"flagReplace":          flagReplace,
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

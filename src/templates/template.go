package templates

import (
	"bytes"
	"embed"
	"errors"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

//go:embed "web"
var TemplateFiles embed.FS

func humanDate(t time.Time) string {
	return t.Format("02.01.2006 15:04")
}

func hasPassed(t time.Time) bool {
	return t.Before(time.Now())
}

var customTemplateFunctions = template.FuncMap{
	"humanDate": humanDate,
	"hasPassed": hasPassed,
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

func Render(ctx *gin.Context, tc map[string]*template.Template, status int, base string, page string, data map[string]any) {
	// Get zerolog instance from context
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	writer := ctx.Writer
	ts, ok := tc[page]
	if !ok {
		mapErr := errors.New("error getting template")
		logger.Error().Msg(mapErr.Error())
		// TODO: Render error site
		abortErr := ctx.AbortWithError(http.StatusInternalServerError, mapErr)
		if abortErr != nil {
			logger.Error().Msg(abortErr.Error())
		}
		return
	}

	// Write parsed template to temporary buffer
	buf := new(bytes.Buffer)

	// Check for errors
	err := ts.ExecuteTemplate(buf, base, data)
	if err != nil {
		logger.Error().Msg(err.Error())
		// TODO: Render error site
		abortErr := ctx.AbortWithError(http.StatusInternalServerError, err)
		if abortErr != nil {
			logger.Error().Msg(abortErr.Error())
		}
		return
	}

	// On success, set header and serve template
	writer.WriteHeader(status)
	_, writeErr := buf.WriteTo(writer)
	if writeErr != nil {
		logger.Error().Msg(writeErr.Error())
		// TODO: Render error site
		abortErr := ctx.AbortWithError(http.StatusInternalServerError, writeErr)
		if abortErr != nil {
			logger.Error().Msg(abortErr.Error())
		}
		return
	}
}

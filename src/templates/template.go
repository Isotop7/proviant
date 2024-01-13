package templates

import (
	"bytes"
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

//go:embed "web"
var TemplateFiles embed.FS

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

		ts, err := template.New(name).ParseFS(TemplateFiles, patterns...)
		if err != nil {
			return nil, err
		}

		cache[name] = ts
	}

	// Return the map.
	return cache, nil
}

func Render(ctx *gin.Context, tc map[string]*template.Template, status int, base string, page string) {
	writer := ctx.Writer
	ts, ok := tc[page]
	if !ok {
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// Write parsed template to temporary buffer
	buf := new(bytes.Buffer)

	// Check for errors
	err := ts.ExecuteTemplate(buf, base, nil)
	if err != nil {
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	// On success, set header and serve template
	writer.WriteHeader(status)
	_, writeErr := buf.WriteTo(writer)
	if writeErr != nil {
		ctx.AbortWithStatus(http.StatusInternalServerError)
		return
	}
}

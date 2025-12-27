package web

import (
	"html/template"
	"testing"
)

func TestFrontend_Struct(t *testing.T) {
	t.Run("can create Frontend", func(t *testing.T) {
		templateCache := make(map[string]*template.Template)
		frontend := Frontend{
			TemplateCache: templateCache,
		}

		if frontend.TemplateCache == nil {
			t.Errorf("TemplateCache = %v, want non-nil", frontend.TemplateCache)
		}
	})
}

func TestFrontend_WithEmptyCache(t *testing.T) {
	t.Run("can create Frontend with empty cache", func(t *testing.T) {
		frontend := Frontend{
			TemplateCache: make(map[string]*template.Template),
		}

		if len(frontend.TemplateCache) != 0 {
			t.Errorf("TemplateCache length = %v, want 0", len(frontend.TemplateCache))
		}
	})
}

func TestFrontend_WithPopulatedCache(t *testing.T) {
	t.Run("can create Frontend with populated cache", func(t *testing.T) {
		templateCache := make(map[string]*template.Template)
		templateCache["test"] = template.Must(template.New("test").Parse("test content"))

		frontend := Frontend{
			TemplateCache: templateCache,
		}

		if len(frontend.TemplateCache) != 1 {
			t.Errorf("TemplateCache length = %v, want 1", len(frontend.TemplateCache))
		}

		if _, exists := frontend.TemplateCache["test"]; !exists {
			t.Errorf("TemplateCache missing 'test' template")
		}
	})
}

func TestFrontend_WithMultipleTemplates(t *testing.T) {
	t.Run("can create Frontend with multiple templates", func(t *testing.T) {
		templateCache := make(map[string]*template.Template)
		templateCache["template1"] = template.Must(template.New("template1").Parse("content1"))
		templateCache["template2"] = template.Must(template.New("template2").Parse("content2"))
		templateCache["template3"] = template.Must(template.New("template3").Parse("content3"))

		frontend := Frontend{
			TemplateCache: templateCache,
		}

		if len(frontend.TemplateCache) != 3 {
			t.Errorf("TemplateCache length = %v, want 3", len(frontend.TemplateCache))
		}

		if _, exists := frontend.TemplateCache["template1"]; !exists {
			t.Errorf("TemplateCache missing 'template1' template")
		}
		if _, exists := frontend.TemplateCache["template2"]; !exists {
			t.Errorf("TemplateCache missing 'template2' template")
		}
		if _, exists := frontend.TemplateCache["template3"]; !exists {
			t.Errorf("TemplateCache missing 'template3' template")
		}
	})
}

func TestFrontend_NilCache(t *testing.T) {
	t.Run("can create Frontend with nil cache", func(t *testing.T) {
		frontend := Frontend{
			TemplateCache: nil,
		}

		if frontend.TemplateCache != nil {
			t.Errorf("TemplateCache = %v, want nil", frontend.TemplateCache)
		}
	})
}

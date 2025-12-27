package webparts

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTileStruct(t *testing.T) {
	t.Run("Tile struct has correct fields", func(t *testing.T) {
		tile := Tile{
			Title:  "Test Title",
			Hero:   "Test Hero",
			Body:   "Test Body content",
			Footer: "Test Footer",
		}

		assert.Equal(t, "Test Title", tile.Title)
		assert.Equal(t, "Test Hero", tile.Hero)
		assert.Equal(t, "Test Body content", tile.Body)
		assert.Equal(t, "Test Footer", tile.Footer)
	})
}

func TestTileWithEmptyFields(t *testing.T) {
	t.Run("Tile can have empty fields", func(t *testing.T) {
		tile := Tile{}

		assert.Empty(t, tile.Title)
		assert.Empty(t, tile.Hero)
		assert.Empty(t, tile.Body)
		assert.Empty(t, tile.Footer)
	})
}

func TestTileWithPartialFields(t *testing.T) {
	tests := []struct {
		name    string
		tile    Tile
		field   string
		wantVal string
	}{
		{
			name: "only title",
			tile: Tile{
				Title: "Only Title",
			},
			field:   "Title",
			wantVal: "Only Title",
		},
		{
			name: "only hero",
			tile: Tile{
				Hero: "42",
			},
			field:   "Hero",
			wantVal: "42",
		},
		{
			name: "title and body",
			tile: Tile{
				Title: "Title",
				Body:  "Body text",
			},
			field:   "Body",
			wantVal: "Body text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch tt.field {
			case "Title":
				assert.Equal(t, tt.wantVal, tt.tile.Title)
			case "Hero":
				assert.Equal(t, tt.wantVal, tt.tile.Hero)
			case "Body":
				assert.Equal(t, tt.wantVal, tt.tile.Body)
			}
		})
	}
}

func TestTileWithSpecialCharacters(t *testing.T) {
	t.Run("Tile can contain special characters", func(t *testing.T) {
		tile := Tile{
			Title:  "Title with émojis 🎉",
			Hero:   "Hero with spëcial char$",
			Body:   "Body with <html> tags & symbols",
			Footer: "Footer with \"quotes\" and 'apostrophes'",
		}

		assert.Contains(t, tile.Title, "🎉")
		assert.Contains(t, tile.Hero, "ë")
		assert.Contains(t, tile.Body, "<html>")
		assert.Contains(t, tile.Footer, "\"")
		assert.Contains(t, tile.Footer, "'")
	})
}

func TestTileWithLongContent(t *testing.T) {
	t.Run("Tile can handle long content", func(t *testing.T) {
		longBody := strings.Repeat("A", 1000)
		tile := Tile{
			Title:  "Title",
			Hero:   "Hero",
			Body:   longBody,
			Footer: "Footer",
		}

		assert.Equal(t, tile.Title, "Title")
		assert.Equal(t, tile.Hero, "Hero")
		assert.Len(t, tile.Body, 1000)
		assert.Equal(t, tile.Footer, "Footer")
	})
}

package assets

import (
	"testing"
)

func TestAssetFiles(t *testing.T) {
	t.Run("AssetFiles is defined", func(t *testing.T) {
		// Test that AssetFiles is of type embed.FS
		var _ = AssetFiles
	})
}

func TestAssetFilesRead(t *testing.T) {
	t.Run("Can read from AssetFiles", func(t *testing.T) {
		// Test that we can read directory entries from the embedded filesystem
		entries, err := AssetFiles.ReadDir(".")
		if err != nil {
			t.Errorf("Failed to read directory: %v", err)
		}

		// Should have at least some embedded directories
		if len(entries) == 0 {
			t.Errorf("Expected at least one embedded directory, got %d", len(entries))
		}

		// Check if expected directories exist
		found := map[string]bool{}
		for _, entry := range entries {
			found[entry.Name()] = true
		}

		for _, dir := range []string{"css", "js", "icons", "fonts"} {
			if !found[dir] {
				t.Errorf("Expected to find '%s' directory in embedded files", dir)
			}
		}
	})
}

func TestAssetFilesOpen(t *testing.T) {
	t.Run("Can open files from AssetFiles", func(t *testing.T) {
		// Test opening a specific file
		file, err := AssetFiles.Open("css/auth.css")
		if err != nil {
			t.Errorf("Failed to open auth.css: %v", err)
			return
		}

		// Check if file can be read
		stat, err := file.Stat()
		if err != nil {
			t.Errorf("Failed to get file stats: %v", err)
			return
		}

		if stat.Size() <= 0 {
			t.Errorf("Expected file size > 0, got %d", stat.Size())
		}
	})
}

func TestAssetFilesNonExistent(t *testing.T) {
	t.Run("Handles non-existent files gracefully", func(t *testing.T) {
		// Test opening non-existent file
		_, err := AssetFiles.Open("non-existent.css")
		if err == nil {
			t.Error("Expected error when opening non-existent file")
		}
	})
}

package controllers

import (
	"strings"
	"testing"

	apiModel "github.com/Isotop7/proviant/models/api"
)

// TestWasteInsight verifies the three states of the insight: the over-buying
// insight, the fallback naming the most common reason and the empty state for
// a household without enough data.
func TestWasteInsight(t *testing.T) {
	t.Run("over-buying names the category with its count and share", func(t *testing.T) {
		overbuy := apiModel.WasteOverbuyStat{
			CategoryKey:   "dairy",
			DisplayName:   "Dairy",
			Count:         3,
			CategoryWaste: 5,
			Share:         60,
		}

		sentence, available := WasteInsight("en", overbuy, nil, 9)

		if !available {
			t.Error("insight = unavailable, want the over-buying insight")
		}
		for _, want := range []string{"Dairy", "3", "5", "60%"} {
			if !strings.Contains(sentence, want) {
				t.Errorf("insight %q does not contain %q", sentence, want)
			}
		}
	})

	t.Run("without over-buying the most common reason is named", func(t *testing.T) {
		causes := []apiModel.WasteCauseStat{
			{Cause: "spoiled", Count: 4, Share: 57.1},
			{Cause: "", Count: 3, Share: 42.9},
		}

		sentence, available := WasteInsight("en", apiModel.WasteOverbuyStat{}, causes, 7)

		if !available {
			t.Error("insight = unavailable, want the fallback insight")
		}
		if !strings.Contains(sentence, "spoiled") && !strings.Contains(sentence, "It spoiled") {
			t.Errorf("insight %q does not name the most common reason", sentence)
		}
		if strings.Contains(sentence, "No reason") {
			t.Errorf("insight %q names the unattributed waste as a reason", sentence)
		}
	})

	t.Run("a single event is no pattern", func(t *testing.T) {
		causes := []apiModel.WasteCauseStat{{Cause: "spoiled", Count: 1, Share: 100}}

		sentence, available := WasteInsight("en", apiModel.WasteOverbuyStat{}, causes, 1)

		if available {
			t.Errorf("insight = %q, want the empty state", sentence)
		}
		if !strings.Contains(sentence, "Not enough") {
			t.Errorf("insight %q is not the empty state", sentence)
		}
	})

	t.Run("without any waste the empty state answers", func(t *testing.T) {
		sentence, available := WasteInsight("en", apiModel.WasteOverbuyStat{}, nil, 0)

		if available {
			t.Errorf("insight = %q, want the empty state", sentence)
		}
		if sentence == "" {
			t.Error("insight is empty, want the empty-state sentence")
		}
	})

	t.Run("the sentence follows the language", func(t *testing.T) {
		overbuy := apiModel.WasteOverbuyStat{DisplayName: "Milchprodukte", Count: 3, CategoryWaste: 5, Share: 60}

		german, _ := WasteInsight("de", overbuy, nil, 9)
		english, _ := WasteInsight("en", overbuy, nil, 9)

		if german == english {
			t.Errorf("the German and the English insight are the same: %q", german)
		}
		if !strings.Contains(german, "Milchprodukte") {
			t.Errorf("German insight %q does not name the category", german)
		}
	})
}

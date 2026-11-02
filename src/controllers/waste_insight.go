package controllers

import (
	"math"

	"github.com/Isotop7/proviant/i18n"
	apiModel "github.com/Isotop7/proviant/models/api"
)

// WasteInsightMinEvents is the number of waste events a cause needs before the
// insight names it: one event is an anecdote, not a pattern.
const WasteInsightMinEvents = 2

// WasteInsight returns the one-sentence insight about a household's waste and
// whether there is enough data to state one. The over-buying insight comes
// first: the category the household buys too much of is the one that changes
// its behaviour. Without it the insight names the reason most of the waste
// has, and without enough data even for that it states the empty state.
func WasteInsight(lang string, overbuy apiModel.WasteOverbuyStat, causes []apiModel.WasteCauseStat, totalWasted int) (string, bool) {
	if overbuy.Count >= WasteInsightMinEvents {
		return i18n.T(lang, "waste.insight.overbought",
			overbuy.DisplayName,
			overbuy.Count,
			overbuy.CategoryWaste,
			int(math.Round(overbuy.Share)),
		), true
	}
	if cause, count, ok := dominantWasteCause(causes); ok {
		return i18n.T(lang, "waste.insight.dominantCause",
			i18n.T(lang, "waste.causePhrase."+cause),
			count,
			totalWasted,
		), true
	}
	return i18n.T(lang, "waste.insight.insufficientData"), false
}

// dominantWasteCause returns the reason most of the waste has. The
// unattributed waste never wins: a waste without a reason is not a reason.
func dominantWasteCause(causes []apiModel.WasteCauseStat) (string, int, bool) {
	bestCause := ""
	bestCount := 0
	for _, cause := range causes {
		if cause.Cause == "" || cause.Count < WasteInsightMinEvents {
			continue
		}
		if cause.Count <= bestCount {
			continue
		}
		bestCause = cause.Cause
		bestCount = cause.Count
	}
	if bestCause == "" {
		return "", 0, false
	}
	return bestCause, bestCount, true
}

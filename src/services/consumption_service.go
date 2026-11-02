package services

import (
	"fmt"
	"math"
	"time"

	"github.com/Isotop7/proviant/controllers/database"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/util"

	"github.com/rs/zerolog"
)

type ConsumptionService struct {
	repos  *database.RepositoryContainer
	logger *zerolog.Logger
}

func NewConsumptionService(repos *database.RepositoryContainer, logger *zerolog.Logger) *ConsumptionService {
	return &ConsumptionService{
		repos:  repos,
		logger: logger,
	}
}

// ConsumptionRate is the consumption-rate estimate for one product, derived
// from the household's archived consumed samples. Display is a pre-formatted
// sentence for the API/UI; TemplateMessage is the same sentence for the
// server-rendered product detail alert.
type ConsumptionRate struct {
	PerWeek         float64
	SampleCount     int
	HasEstimate     bool
	DaysCovered     int
	LastConsumed    *time.Time
	Display         string
	TemplateMessage string
}

// RestockSuggestion is the suggested quantity to add to the shopping list
// for a product, plus the source of the suggestion and human-readable copy
// for the API and template.
type RestockSuggestion struct {
	ProductName     string
	SuggestedQty    int
	Source          string
	WeeklyRate      float64
	SampleCount     int
	HasEstimate     bool
	Display         string
	TemplateMessage string
	PerWeekDisplay  string
}

// ComputeConsumptionRate estimates the weekly consumption rate for a product
// (matched by barcode when present, otherwise by product name) based on the
// household's consumed samples in the last 90 days. Requires at least 2
// samples spread over at least 7 days to produce a stable "per week" rate.
func (s *ConsumptionService) ComputeConsumptionRate(householdID, userID uint, barcode, name string) (ConsumptionRate, error) {
	since := time.Now().AddDate(0, 0, -util.ConsumptionHistoryWindowDays)
	rate := ConsumptionRate{}

	products, err := s.repos.Products.GetConsumedSamples(householdID, userID, barcode, name, since)
	if err != nil {
		return rate, err
	}

	rate.SampleCount = len(products)
	if rate.SampleCount < util.ConsumptionMinSamples {
		return rate, nil
	}

	totalQuantity := aggregateSamples(products)

	first := products[0].DeletedAt.Time
	last := products[rate.SampleCount-1].DeletedAt.Time
	// Round to the nearest day so a 7-day span with sub-hour skew (or a
	// DST transition) does not get truncated to 6 and silently disable
	// the rate.
	spanDays := max(int(math.Round(last.Sub(first).Hours()/24)), util.ConsumptionSpanFloorDays)
	rate.DaysCovered = spanDays

	// Require a meaningful observation window before labelling the result
	// "per week". Two same-day samples would otherwise inflate the rate
	// by roughly 7x.
	if spanDays < util.ConsumptionMinSpanDays {
		return rate, nil
	}

	weeks := float64(spanDays) / 7.0
	rate.PerWeek = totalQuantity / weeks
	rate.HasEstimate = true
	lastCopy := last
	rate.LastConsumed = &lastCopy
	rate.Display = formatRateDisplay(rate.PerWeek)
	rate.TemplateMessage = formatRateTemplateMessage(rate.PerWeek, rate.SampleCount)

	return rate, nil
}

// ComputeRestockSuggestionFromProduct returns a restock suggestion for an
// already-loaded product. If a stable consumption rate is available it is
// preferred (ceil(PerWeek)). Otherwise, when the current amount is below
// the configured minimum stock, the deficit is suggested. When the minimum
// is already met (or not configured) no suggestion is produced.
func (s *ConsumptionService) ComputeRestockSuggestionFromProduct(householdID, userID uint, product *dbModel.Product) RestockSuggestion {
	rate, rateErr := s.ComputeConsumptionRate(householdID, userID, product.Barcode, product.ProductName)
	if rateErr != nil {
		s.logger.Warn().Msgf("ComputeRestockSuggestion: rate calc failed: %s", rateErr)
	}

	suggestion := RestockSuggestion{
		ProductName: product.ProductName,
		WeeklyRate:  rate.PerWeek,
		SampleCount: rate.SampleCount,
	}

	if rate.HasEstimate && rate.PerWeek > 0 {
		suggestion.SuggestedQty = int(math.Ceil(rate.PerWeek))
		suggestion.Source = util.ConsumptionSourceRate
		suggestion.HasEstimate = true
		suggestion.Display = formatSuggestionDisplay(suggestion.SuggestedQty)
		suggestion.TemplateMessage = rate.TemplateMessage
		suggestion.PerWeekDisplay = rate.Display
		return suggestion
	}

	// Min-stock fallback: only meaningful when the current amount is below
	// the configured minimum. When the minimum is met (or not configured)
	// we return "none" so the UI does not open a misleading modal.
	if product.MinStockAmount > 0 {
		deficit := product.MinStockAmount - product.Quantity
		if deficit > 0 {
			suggestion.SuggestedQty = deficit
			suggestion.Source = util.ConsumptionSourceMinStock
			suggestion.Display = formatSuggestionDisplay(suggestion.SuggestedQty)
			suggestion.TemplateMessage = formatMinStockTemplateMessage(suggestion.SuggestedQty, product.MinStockAmount)
			return suggestion
		}
	}

	suggestion.SuggestedQty = 0
	suggestion.Source = util.ConsumptionSourceNone
	return suggestion
}

// aggregateSamples sums the pack counts across all samples. A sample with no
// count is treated as one pack to keep the denominator meaningful.
func aggregateSamples(products []dbModel.Product) float64 {
	var total float64
	for i := range products {
		quantity := products[i].Quantity
		if quantity <= 0 {
			quantity = 1
		}
		total += float64(quantity)
	}
	return total
}

// humanizeQuantity rounds to 1 decimal and strips trailing zeros so that
// integral values render as "2" rather than "2.0".
func humanizeQuantity(value float64) string {
	rounded := math.Round(value*10) / 10
	if rounded == math.Trunc(rounded) {
		return fmt.Sprintf("%d", int64(rounded))
	}
	return fmt.Sprintf("%.1f", rounded)
}

func formatRateDisplay(perWeek float64) string {
	return fmt.Sprintf("~%s per week", humanizeQuantity(perWeek))
}

func formatRateTemplateMessage(perWeek float64, sampleCount int) string {
	return fmt.Sprintf("You use ~%s per week (based on the last %d consumptions).", humanizeQuantity(perWeek), sampleCount)
}

func formatSuggestionDisplay(qty int) string {
	if qty <= 0 {
		return ""
	}
	return fmt.Sprintf("Suggested: %d", qty)
}

func formatMinStockTemplateMessage(suggestedQty int, minStock int) string {
	return fmt.Sprintf("Suggested restock: %d to meet your minimum stock of %d.", suggestedQty, minStock)
}

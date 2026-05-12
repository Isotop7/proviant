package v1

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"codeberg.org/isotop7/proviant/api"
	dbRepo "codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
)

const (
	CalendarExpireDays = 30
	CalendarProdID     = "-//Proviant//ProductExpiry//EN"
	mimeTypeCalendar   = "text/calendar; charset=utf-8"
)

func escapeICalText(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, ";", "\\;")
	s = strings.ReplaceAll(s, ",", "\\,")
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

func formatICALDate(t time.Time) string {
	return t.Format("20060102")
}

func formatICALTimestamp(t time.Time) string {
	return t.UTC().Format("20060102T150405Z")
}

func generateVEVENT(product *database.Product) string {
	uid := strings.TrimSpace(strconv.FormatUint(uint64(product.ID), 10)) + "@proviant"
	dtstamp := formatICALTimestamp(time.Now())
	dtstart := formatICALDate(product.ExpireAt)
	summary := escapeICalText(product.ProductName)
	description := escapeICalText(formatProductDescription(product))

	var sb strings.Builder
	sb.WriteString("BEGIN:VEVENT\r\n")
	sb.WriteString("UID:" + uid + "\r\n")
	sb.WriteString("DTSTAMP:" + dtstamp + "\r\n")
	sb.WriteString("DTSTART;VALUE=DATE:" + dtstart + "\r\n")
	sb.WriteString("SUMMARY:" + summary + "\r\n")
	sb.WriteString("DESCRIPTION:" + description + "\r\n")
	sb.WriteString("BEGIN:VALARM\r\n")
	sb.WriteString("TRIGGER:-P1D\r\n")
	sb.WriteString("ACTION:DISPLAY\r\n")
	sb.WriteString("DESCRIPTION:Expires tomorrow: " + summary + "\r\n")
	sb.WriteString("END:VALARM\r\n")
	sb.WriteString("END:VEVENT\r\n")
	return sb.String()
}

func formatProductDescription(product *database.Product) string {
	desc := product.ProductName
	if product.Amount > 0 {
		desc = desc + " - " + strconv.Itoa(product.Amount)
		if product.Unit != "" {
			desc = desc + " " + product.Unit
		}
	}
	if product.StorageLocation != nil {
		desc = desc + " @ " + product.StorageLocation.Name
	}
	return desc
}

// ExportICalendar returns an iCalendar feed of products expiring in the next 30 days
// @Summary      Export iCalendar feed
// @Description  Returns an iCalendar (RFC 5545) feed with VEVENTs for products expiring in the next 30 days. Use the token query parameter for authentication.
// @Tags         calendar
// @Produce      text/calendar
// @Param        token  query  string  true  "Calendar token for authentication"
// @Success      200   {string} string  "iCalendar feed"
// @Failure      401   {object} api.APIResponse
// @Failure      500   {object} api.APIResponse
// @Router       /api/v1/calendar/export.ics [get]
func ExportICalendar(ctx *gin.Context, appCtx *AppContext) {
	token := ctx.Query("token")
	if token == "" {
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, api.APIResponse{Message: "Calendar token required"})
		return
	}

	calendarTokenRepo := dbRepo.NewCalendarTokenRepository(appCtx.DB)
	ct, err := calendarTokenRepo.GetByToken(token)
	if err != nil {
		appCtx.Logger.Debug().Msgf("Invalid calendar token: %s", err)
		ctx.AbortWithStatusJSON(http.StatusUnauthorized, api.APIResponse{Message: "Invalid calendar token"})
		return
	}

	productRepo := dbRepo.NewProductRepository(appCtx.DB)
	products, err := productRepo.GetExpiringInDays(ct.UserID, CalendarExpireDays)
	if err != nil {
		appCtx.Logger.Error().Msgf("GetExpiringInDays: %s", err)
		ctx.AbortWithStatusJSON(http.StatusInternalServerError, api.APIResponse{Message: "Error fetching products"})
		return
	}

	var sb strings.Builder
	sb.WriteString("BEGIN:VCALENDAR\r\n")
	sb.WriteString("VERSION:2.0\r\n")
	sb.WriteString("PRODID:-//Proviant//ProductExpiry//EN\r\n")
	sb.WriteString("CALSCALE:GREGORIAN\r\n")
	sb.WriteString("METHOD:PUBLISH\r\n")

	for i := range products {
		sb.WriteString(generateVEVENT(&products[i]))
	}

	sb.WriteString("END:VCALENDAR\r\n")

	ctx.Header(util.RequestHeaderContentType, mimeTypeCalendar)
	ctx.Header(util.RequestHeaderContentDisposition, "inline")
	ctx.Data(http.StatusOK, mimeTypeCalendar, []byte(sb.String()))
}

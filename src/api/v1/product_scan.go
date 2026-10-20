package v1

import (
	"context"
	"errors"
	"image"

	// Import for image decoding
	_ "image/jpeg"
	// Import for image decoding
	_ "image/png"
	"net/http"

	"image/draw"

	"github.com/Isotop7/proviant/api"
	"github.com/Isotop7/proviant/controllers"
	"github.com/Isotop7/proviant/controllers/database"
	apperrors "github.com/Isotop7/proviant/errors"
	apiModel "github.com/Isotop7/proviant/models/api"
	"github.com/Isotop7/proviant/models/configuration"
	"github.com/Isotop7/proviant/models/configuration/static"
	dbModel "github.com/Isotop7/proviant/models/database"
	"github.com/Isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/oned"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// ScanProduct returns a barcode based on an image
// @Summary      	Scan product
// @Description  	Returns the barcode of a product in an uploaded image
// @Tags         	product
// @Accept			json
// @Produce      	json
// @Success      	200  {object}  database.ProductDTOBarcode
// @Failure      	400  {object}  api.APIResponse
// @Failure      	500  {object}  api.APIResponse
// @Router       	/api/v1/products/scan [post]
func ScanProduct(ctx *gin.Context, appCtx *AppContext) {
	logger := appCtx.Logger

	// Create variables
	var decodedBarcode string

	// Create decoding context
	decodingTimeout := static.BarcodeDecodingTimeout
	decodingContext, cancel := context.WithTimeout(context.Background(), decodingTimeout)
	defer cancel()
	// Create communication channel for go routine
	decodingProcessChannel := make(chan bool)

	// Decode image go routine
	go func() {
		// Read file from form or signal error
		file, formErr := ctx.FormFile("image")
		if formErr != nil {
			logger.Error().Msgf("Error reading image from body: %s", formErr.Error())
			api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrNoBarcodeFoundInImage)
			decodingProcessChannel <- false
			return
		}

		proviantConfig, _ := ctx.MustGet(util.ContextKeyProviantConfig).(*configuration.ProviantConfiguration)
		if file.Size > int64(proviantConfig.Server.MaxUploadSizeMB)*1024*1024 {
			api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrFileTooLarge)
			decodingProcessChannel <- false
			return
		}

		// Open file from form or signal error
		src, openErr := file.Open()
		if openErr != nil {
			logger.Error().Msgf("Error opening file: %s", openErr.Error())
			api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrNoBarcodeFoundInImage)
			decodingProcessChannel <- false
			return
		}

		// Decode file from form as image or signal error
		img, format, decodeErr := image.Decode(src)
		if decodeErr != nil {
			logger.Error().Msgf("Error decoding image as type %s: %s", format, decodeErr.Error())
			api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrNoBarcodeFoundInImage)
			decodingProcessChannel <- false
			return
		} else {
			logger.Info().Msgf("Decoded image of type '%s' from body", format)
		}

		// Convert image to gray scale image
		convertedImage := image.NewGray(img.Bounds())
		draw.Draw(convertedImage, convertedImage.Bounds(), img, img.Bounds().Min, draw.Src)

		// Convert gray scaled image to binary bitmap or signal error
		bmp, bmpErr := gozxing.NewBinaryBitmapFromImage(convertedImage)
		if bmpErr != nil {
			logger.Error().Msgf("Error converting image to bitmap: %s", bmpErr.Error())
			api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrNoBarcodeFoundInImage)
			decodingProcessChannel <- false
			return
		}

		// Create EAN13 scanner and hints
		scanner := oned.NewEAN13Reader()
		hints := map[gozxing.DecodeHintType]any{
			gozxing.DecodeHintType_TRY_HARDER:             true,
			gozxing.DecodeHintType_ALLOWED_EAN_EXTENSIONS: true,
			gozxing.DecodeHintType_ALSO_INVERTED:          true,
		}

		// Decode image and try to find barcode
		code, scanErr := scanner.Decode(bmp, hints)
		if scanErr != nil {
			logger.Error().Msgf("Error decoding image when finding barcode: %s", scanErr.Error())
			api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrNoBarcodeFoundInImage)
			return
		} else {
			// If barcode is found, return it
			decodedBarcode = code.GetText()
		}
		// Signal success
		decodingProcessChannel <- true
	}()

	// Wait for success or timeout
	select {
	case <-decodingContext.Done():
		// Timeout was reached, error is returned
		api.RespondError(ctx, http.StatusInternalServerError, apperrors.ErrBarcodeDecodeTimeoutExceeded)
		return
	case success := <-decodingProcessChannel:
		// Timeout was not reached and channel signaled success on decoding barcode
		if success {
			ctx.JSON(http.StatusOK, dbModel.ProductDTOBarcode{Barcode: decodedBarcode})
			return
		}
	}

	// Return if timeout was not reached but channel did not signal success
	api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrNoBarcodeFoundInImage)
}

// GetOpenFoodFactsData returns product data from OpenFoodFacts for a given barcode.
// It consults the locally imported bulk dump first, then the database cache when
// cacheEnabled is true, and finally proxies the live OpenFoodFacts API.
//
// @Summary      Return OpenFoodFacts product data
// @Description  Resolves a barcode from the imported dump, the database cache, or the proxy
// @Tags         product
// @Produce      json
// @Param        barcode  path  string  true  "Barcode"
// @Success      200  {object}  apiModel.OpenFoodFactsCacheResponse
// @Failure      400  {object}  api.APIResponse
// @Failure      500  {object}  api.APIResponse
// @Failure      502  {object}  api.APIResponse
// @Router       /api/v1/products/openfoodfacts/{barcode} [get]
func GetOpenFoodFactsData(ctx *gin.Context, appCtx *AppContext) {
	barcode := ctx.Param("barcode")
	if barcode == "" {
		api.RespondError(ctx, http.StatusBadRequest, errors.New("barcode missing"))
		return
	}

	offacntrl, offaOk := ctx.MustGet("offacntrl").(*controllers.OpenFoodFactsAPIController)
	if !offaOk {
		appCtx.Logger.Error().Msg("Failed to get OpenFoodFacts controller from context")
		api.RespondError(ctx, http.StatusInternalServerError, errors.New("failed to get controller from context"))
		return
	}

	productRepo := database.NewProductRepository(appCtx.DB)

	if product, found := offacntrl.LookupOffline(barcode); found {
		appCtx.Logger.Info().Msgf("Offline dump hit for barcode '%s'", barcode)
		entry := dbModel.OpenFoodFactsCache{
			Barcode:     product.Barcode,
			ProductName: product.ProductName,
			Categories:  product.Categories,
			Countries:   product.Countries,
			ImageURL:    product.ImageURL,
			PackSize:    product.PackSize,
			CO2KgPerKg:  product.CO2KgPerKg,
			StorageHint: product.StorageHint,
		}
		ctx.JSON(http.StatusOK, apiModel.NewOpenFoodFactsCacheResponse(&entry))
		return
	}

	if offacntrl.Configuration.CacheEnabled {
		cached, cacheErr := productRepo.GetOpenFoodFactsCacheByBarcode(barcode)
		if cacheErr == nil {
			appCtx.Logger.Info().Msgf("Cache hit for barcode '%s'", barcode)
			ctx.JSON(http.StatusOK, apiModel.NewOpenFoodFactsCacheResponse(&cached))
			return
		}
		if cacheErr != gorm.ErrRecordNotFound {
			appCtx.Logger.Warn().Msgf("Cache lookup error for barcode '%s': %s", barcode, cacheErr)
		}
	}

	product, apiErr := offacntrl.GetLiveDataset(barcode)
	if apiErr != nil {
		appCtx.Logger.Error().Msgf("OpenFoodFacts API error for barcode '%s': %s", barcode, apiErr)
		api.RespondError(ctx, http.StatusBadGateway, errors.New("error fetching product data from OpenFoodFacts"))
		return
	}

	entry := dbModel.OpenFoodFactsCache{
		Barcode:     product.Barcode,
		ProductName: product.ProductName,
		Categories:  product.Categories,
		Countries:   product.Countries,
		ImageURL:    product.ImageURL,
		PackSize:    product.PackSize,
		CO2KgPerKg:  product.CO2KgPerKg,
		StorageHint: product.StorageHint,
	}

	if offacntrl.Configuration.CacheEnabled {
		storeCacheEntry(productRepo, offacntrl, appCtx.Logger, barcode, &entry)
	}

	ctx.JSON(http.StatusOK, apiModel.NewOpenFoodFactsCacheResponse(&entry))
}

func storeCacheEntry(productRepo *database.ProductRepository, offacntrl *controllers.OpenFoodFactsAPIController, logger *zerolog.Logger, barcode string, entry *dbModel.OpenFoodFactsCache) {
	if storeErr := productRepo.CreateOpenFoodFactsCache(entry); storeErr != nil {
		logger.Warn().Msgf("Failed to store cache entry for barcode '%s': %s", barcode, storeErr)
		return
	}
	if !offacntrl.Configuration.ImageCacheEnabled || entry.ImageURL == "" {
		return
	}
	localPath, imgErr := offacntrl.DownloadImage(entry.ImageURL, barcode)
	if imgErr != nil {
		logger.Warn().Msgf("Failed to cache image for barcode '%s': %s", barcode, imgErr)
		return
	}
	entry.ImageURL = localPath
	if updateErr := productRepo.UpdateOpenFoodFactsCacheImageURL(barcode, localPath); updateErr != nil {
		logger.Warn().Msgf("Failed to update cached image URL for barcode '%s': %s", barcode, updateErr)
	}
}

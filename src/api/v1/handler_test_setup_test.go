package v1

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
	"gorm.io/gorm"
)

// handlerTestEnv bundles the common fixtures for DB-backed handler tests:
// an admin user owning a household, plus a gin context wired with repos,
// config and JWT claims.
type handlerTestEnv struct {
	Ctx       *gin.Context
	W         *httptest.ResponseRecorder
	AppCtx    *AppContext
	DB        *gorm.DB
	User      *authentication.User
	Household *dbModel.Household
}

func setupHandlerTest(t *testing.T) *handlerTestEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := testutil.SetupTestDB(t)
	user := testutil.CreateTestUser(db, 0)
	household := testutil.CreateTestHousehold(db, user.ID)
	user.HouseholdID = household.ID
	user.Role = authentication.RoleAdmin
	if err := db.Save(user).Error; err != nil {
		t.Fatalf("save user: %v", err)
	}

	ctx, w := testutil.SetupGinContext(db)
	testutil.MockJWTClaims(ctx, user.ID)
	ctx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(db, nil))
	ctx.Set(util.ContextKeyProviantConfig, &configuration.ProviantConfiguration{})
	// AdminResetUserPassword MustGets this key; a typed nil skips the mail send.
	ctx.Set(util.ContextKeyNotificationController, (*controllers.NotificationController)(nil))
	appCtx := SetupTestAppContext(ctx, user.ID)
	return &handlerTestEnv{Ctx: ctx, W: w, AppCtx: appCtx, DB: db, User: user, Household: household}
}

// addMember creates a non-admin user inside the env's household.
func (e *handlerTestEnv) addMember(t *testing.T, username string) *authentication.User {
	t.Helper()
	member := testutil.CreateTestUser(e.DB, e.Household.ID)
	member.Username = username
	member.Role = authentication.RoleMember
	if err := e.DB.Save(member).Error; err != nil {
		t.Fatalf("save member: %v", err)
	}
	return member
}

// useMember switches the env's context + appCtx to the given user.
func (e *handlerTestEnv) useMember(member *authentication.User) {
	testutil.MockJWTClaims(e.Ctx, member.ID)
	e.AppCtx.UserID = member.ID
}

// freshCtx returns a new gin context + recorder on the same DB, for tests that
// invoke a handler more than once (a recorder only keeps the first status code).
func (e *handlerTestEnv) freshCtx(userID uint) (*gin.Context, *httptest.ResponseRecorder) {
	ctx, w := testutil.SetupGinContext(e.DB)
	testutil.MockJWTClaims(ctx, userID)
	ctx.Set(util.ContextKeyRepos, database.NewRepositoryContainer(e.DB, nil))
	ctx.Set(util.ContextKeyProviantConfig, &configuration.ProviantConfiguration{})
	ctx.Set(util.ContextKeyNotificationController, (*controllers.NotificationController)(nil))
	return ctx, w
}

// multipartImageRequest builds a multipart POST request carrying one file field.
func multipartImageRequest(t *testing.T, fieldName, filename string, content []byte) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/upload", &buf)
	req.Header.Set(util.RequestHeaderContentType, writer.FormDataContentType())
	return req
}

// pngWithText renders text onto a white PNG so OCR tests have a decodable
// image. A large TrueType face (not the 7x13 bitmap font) keeps tesseract's
// output reliable enough to assert on the parsed date.
func pngWithText(t *testing.T, text string) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 640, 200))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	parsed, err := opentype.Parse(goregular.TTF)
	if err != nil {
		t.Fatalf("parse font: %v", err)
	}
	face, err := opentype.NewFace(parsed, &opentype.FaceOptions{
		Size:    48,
		DPI:     72,
		Hinting: font.HintingFull,
	})
	if err != nil {
		t.Fatalf("build font face: %v", err)
	}
	defer face.Close()
	d := &font.Drawer{
		Dst:  img,
		Src:  image.Black,
		Face: face,
		Dot:  fixed.P(20, 120),
	}
	d.DrawString(text)
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// blankPNG returns a valid PNG without any barcode or text.
func blankPNG(t *testing.T) []byte {
	t.Helper()
	return pngWithText(t, "")
}

// itoa formats a uint path param value.
func itoa(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}

// bytesReaderString builds a request body reader from a raw string.
func bytesReaderString(s string) *bytes.Reader {
	return bytes.NewReader([]byte(s))
}

// newRawJSONRequest builds a POST request with a raw JSON body.
func newRawJSONRequest(body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/x", bytesReaderString(body))
	req.Header.Set(util.RequestHeaderContentType, "application/json")
	return req
}

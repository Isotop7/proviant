package v1

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/testutil"
	repomocks "codeberg.org/isotop7/proviant/testutil/mocks"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func openProductRequest(t *testing.T, body any) (*gin.Context, *repomocks.MockRepositoryContainer, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	m := repomocks.NewMockRepositoryContainer()
	ctx, w := repomocks.SetupGinContextWithMocks(m)
	testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
	ctx.Request = &http.Request{Header: make(http.Header)}
	ctx.Params = []gin.Param{{Key: "id", Value: "1"}}
	if body != nil {
		buf, _ := json.Marshal(body)
		ctx.Request.Body = io.NopCloser(bytes.NewBuffer(buf))
		ctx.Request.Header.Set("Content-Type", "application/json")
		ctx.Request.ContentLength = int64(len(buf))
	}
	return ctx, m, w
}

func TestOpenProduct(t *testing.T) {
	t.Run("first mark returns 200 and sets OpenedAt", func(t *testing.T) {
		ctx, m, w := openProductRequest(t, nil)
		m.Products.Product = dbModel.Product{
			Model:       gorm.Model{ID: 1},
			ProductName: "Jam",
			Barcode:     "1234567890123",
		}
		appCtx := newTestAppContext(m, 1)
		OpenProduct(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if m.Products.Product.OpenedAt == nil {
			t.Fatalf("OpenedAt not set on product after /open call")
		}
		if time.Since(*m.Products.Product.OpenedAt) > 5*time.Second {
			t.Errorf("OpenedAt = %v, want approximately now", *m.Products.Product.OpenedAt)
		}
	})

	t.Run("second mark without force returns 409 with previous OpenedAt", func(t *testing.T) {
		ctx, m, w := openProductRequest(t, nil)
		previous := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
		m.Products.Product = dbModel.Product{
			Model:       gorm.Model{ID: 1},
			ProductName: "Jam",
			Barcode:     "1234567890123",
			OpenedAt:    &previous,
		}
		appCtx := newTestAppContext(m, 1)
		OpenProduct(ctx, appCtx)

		if w.Code != http.StatusConflict {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusConflict)
		}
		var resp OpenProductResponse
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("response not JSON: %v", err)
		}
		if !resp.OpenedAt.Equal(previous) {
			t.Errorf("OpenedAt = %v, want %v", resp.OpenedAt, previous)
		}
		if m.Products.Product.OpenedAt == nil || !m.Products.Product.OpenedAt.Equal(previous) {
			t.Errorf("stored OpenedAt changed on 409 path: got %v, want %v", m.Products.Product.OpenedAt, previous)
		}
	})

	t.Run("force=true overwrites OpenedAt", func(t *testing.T) {
		ctx, m, w := openProductRequest(t, OpenProductRequest{Force: true})
		previous := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
		m.Products.Product = dbModel.Product{
			Model:       gorm.Model{ID: 1},
			ProductName: "Jam",
			Barcode:     "1234567890123",
			OpenedAt:    &previous,
		}
		appCtx := newTestAppContext(m, 1)
		OpenProduct(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
		}
		if m.Products.Product.OpenedAt == nil {
			t.Fatalf("OpenedAt not set on product after force /open call")
		}
		if !m.Products.Product.OpenedAt.After(previous) {
			t.Errorf("OpenedAt = %v, want > previous %v", *m.Products.Product.OpenedAt, previous)
		}
	})

	t.Run("repo error returns 500", func(t *testing.T) {
		ctx, m, w := openProductRequest(t, nil)
		m.Products.Err = gorm.ErrInvalidData
		appCtx := newTestAppContext(m, 1)
		OpenProduct(ctx, appCtx)

		if w.Code != http.StatusInternalServerError {
			t.Fatalf("status = %d, want %d", w.Code, http.StatusInternalServerError)
		}
	})

	t.Run("chunked body with force=true overwrites OpenedAt", func(t *testing.T) {
		// Simulate a chunked / Content-Length-less request: ContentLength = -1
		// but a valid JSON body is present. The body-parsing logic must not
		// gate on ContentLength and must honour force=true.
		gin.SetMode(gin.TestMode)
		m := repomocks.NewMockRepositoryContainer()
		ctx, w := repomocks.SetupGinContextWithMocks(m)
		testutil.MockJWTClaimsWithKey(ctx, 1, testutil.TokenIdentityKey)
		buf, _ := json.Marshal(OpenProductRequest{Force: true})
		ctx.Request = &http.Request{
			Header:        make(http.Header),
			Body:          io.NopCloser(bytes.NewBuffer(buf)),
			ContentLength: -1,
		}
		ctx.Request.Header.Set("Content-Type", "application/json")
		ctx.Params = []gin.Param{{Key: "id", Value: "1"}}
		previous := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
		m.Products.Product = dbModel.Product{
			Model:       gorm.Model{ID: 1},
			ProductName: "Jam",
			Barcode:     "1234567890123",
			OpenedAt:    &previous,
		}
		appCtx := newTestAppContext(m, 1)
		OpenProduct(ctx, appCtx)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d (force override must work on chunked body)", w.Code, http.StatusOK)
		}
		if m.Products.Product.OpenedAt == nil || !m.Products.Product.OpenedAt.After(previous) {
			t.Errorf("OpenedAt = %v, want > previous %v", m.Products.Product.OpenedAt, previous)
		}
	})
}

func TestEffectiveExpireAt(t *testing.T) {
	now := time.Now()
	zero := time.Time{}
	d := func(days int) *int { return &days }

	cases := []struct {
		name    string
		product dbModel.Product
		want    time.Time
	}{
		{
			name:    "no fields set returns zero",
			product: dbModel.Product{},
			want:    zero,
		},
		{
			name:    "only printed date returns printed",
			product: dbModel.Product{ExpireAt: now.Add(24 * time.Hour)},
			want:    now.Add(24 * time.Hour),
		},
		{
			name: "opened with days returns opened+days when earlier",
			product: dbModel.Product{
				ExpireAt:         now.Add(10 * 24 * time.Hour),
				OpenedAt:         &now,
				DaysAfterOpening: d(2),
			},
			want: now.Add(2 * 24 * time.Hour),
		},
		{
			name: "opened later than printed returns printed",
			product: dbModel.Product{
				ExpireAt:         now.Add(1 * 24 * time.Hour),
				OpenedAt:         &now,
				DaysAfterOpening: d(10),
			},
			want: now.Add(1 * 24 * time.Hour),
		},
		{
			name: "opened without days returns printed",
			product: dbModel.Product{
				ExpireAt: now.Add(5 * 24 * time.Hour),
				OpenedAt: &now,
			},
			want: now.Add(5 * 24 * time.Hour),
		},
		{
			name: "days without opened returns printed",
			product: dbModel.Product{
				ExpireAt:         now.Add(5 * 24 * time.Hour),
				DaysAfterOpening: d(10),
			},
			want: now.Add(5 * 24 * time.Hour),
		},
		{
			name: "zero days is ignored (treat as no days)",
			product: dbModel.Product{
				ExpireAt:         now.Add(5 * 24 * time.Hour),
				OpenedAt:         &now,
				DaysAfterOpening: d(0),
			},
			want: now.Add(5 * 24 * time.Hour),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := tc.product
			got := p.EffectiveExpireAt()
			if !got.Equal(tc.want) {
				t.Errorf("EffectiveExpireAt = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestTriggerForExpireAt(t *testing.T) {
	now := time.Now()
	d := func(days int) *int { return &days }

	p := dbModel.Product{ExpireAt: now.Add(10 * 24 * time.Hour)}
	if got := p.TriggerForExpireAt(); got != dbModel.ExpiryTriggerPrinted {
		t.Errorf("printed-only: trigger = %v, want %v", got, dbModel.ExpiryTriggerPrinted)
	}

	p = dbModel.Product{
		ExpireAt:         now.Add(10 * 24 * time.Hour),
		OpenedAt:         &now,
		DaysAfterOpening: d(2),
	}
	if got := p.TriggerForExpireAt(); got != dbModel.ExpiryTriggerOpened {
		t.Errorf("opened earlier: trigger = %v, want %v", got, dbModel.ExpiryTriggerOpened)
	}

	p = dbModel.Product{
		ExpireAt:         now.Add(1 * 24 * time.Hour),
		OpenedAt:         &now,
		DaysAfterOpening: d(10),
	}
	if got := p.TriggerForExpireAt(); got != dbModel.ExpiryTriggerPrinted {
		t.Errorf("printed earlier: trigger = %v, want %v", got, dbModel.ExpiryTriggerPrinted)
	}

	// Equal dates => printed wins (strictly earlier is required for opened).
	p = dbModel.Product{
		ExpireAt:         now.Add(5 * 24 * time.Hour),
		OpenedAt:         &now,
		DaysAfterOpening: d(5),
	}
	if got := p.TriggerForExpireAt(); got != dbModel.ExpiryTriggerPrinted {
		t.Errorf("equal dates: trigger = %v, want %v", got, dbModel.ExpiryTriggerPrinted)
	}

	// nil receiver
	var nilP *dbModel.Product
	if got := nilP.TriggerForExpireAt(); got != dbModel.ExpiryTriggerPrinted {
		t.Errorf("nil product: trigger = %v, want %v", got, dbModel.ExpiryTriggerPrinted)
	}
}

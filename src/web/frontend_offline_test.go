package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	repomocks "github.com/Isotop7/proviant/testutil/mocks"
)

// The offline page is the fallback the service worker serves from its cache
// when a navigation cannot reach the network. It must render without a
// session, carry the app stylesheet and follow the requested language.
func TestOfflineHandler(t *testing.T) {
	env := newHandlerEnv(t)

	t.Run("renders the offline page in the default language", func(t *testing.T) {
		w := env.run(0, "/offline", nil, env.frontend.Offline)
		requireStatus(t, w, http.StatusOK)
		requireContains(t, w, "You&#39;re offline", "Try again", "/assets/css/main.css")
	})

	t.Run("renders the offline page in the requested language", func(t *testing.T) {
		ctx, w := repomocks.SetupGinContextWithDB(env.db)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/offline", nil)
		ctx.Request.Header.Set("Accept-Language", "de-DE,de;q=0.9")
		env.frontend.Offline(ctx)
		requireStatus(t, w, http.StatusOK)
		requireContains(t, w, "Sie sind offline", "Erneut versuchen")
	})
}

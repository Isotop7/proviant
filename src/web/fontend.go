package web

import (
	"html/template"
	"net/http"

	"github.com/gin-gonic/gin"
	"gitlab.com/Isotop7/expiro/templates"
)

type Frontend struct {
	TemplateCache map[string]*template.Template
}

func (frontend *Frontend) Root(ctx *gin.Context) {
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "home.tmpl")
}

func (frontend *Frontend) Auth(ctx *gin.Context) {
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "baseAuth", "auth.tmpl")
}

func (frontend *Frontend) AuthLogin(ctx *gin.Context) {
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "authLogin.tmpl")
}

func (frontend *Frontend) AuthRegister(ctx *gin.Context) {
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "authRegister.tmpl")
}

func (frontend *Frontend) User(ctx *gin.Context) {
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "user.tmpl")
}

func (frontend *Frontend) UserSettings(ctx *gin.Context) {
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "userSettings.tmpl")
}

func (frontend *Frontend) Products(ctx *gin.Context) {
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "products.tmpl")
}

func (frontend *Frontend) ProductsCreate(ctx *gin.Context) {
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "productsCreate.tmpl")
}

func (frontend *Frontend) ProductsScan(ctx *gin.Context) {
	templates.Render(ctx, frontend.TemplateCache, http.StatusOK, "base", "productsScan.tmpl")
}

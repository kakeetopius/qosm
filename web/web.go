// Package web contains backend logic for http server.
package web

import (
	"embed"
	"io/fs"
	"net/http"

	"github.com/gin-gonic/gin"
)

//go:embed templates
var tmplFs embed.FS

//go:embed static
var staticFS embed.FS

func Run() error {
	router := gin.Default()

	embedFiles(router)

	app := ServerCtx{}
	addRoutes(router, &app)

	router.Run()
	return nil
}

func addRoutes(router *gin.Engine, app *ServerCtx) {
	router.Use(ErrorHandlerHTML())

	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "hello from qosm")
	})

	router.GET("/login", app.login)
	router.POST("/login", app.loginPost)
	router.GET("/dashboard", app.dashboard)
	router.GET("/rules", app.rules)
	router.GET("/analytics", app.analytics)
	router.GET("/logs", app.logs)
	router.GET("/settings", app.settings)
}

func embedFiles(router *gin.Engine) error {
	tmplSubFS, err := fs.Sub(tmplFs, "templates")
	if err != nil {
		return err
	}
	router.LoadHTMLFS(http.FS(tmplSubFS), "**/*.tmpl")

	staticSubFS, err := fs.Sub(staticFS, "static")
	if err != nil {
		return err
	}
	router.StaticFS("/static", http.FS(staticSubFS))

	return nil
}

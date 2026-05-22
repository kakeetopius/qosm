// Package web contains backend logic for http server.
package web

import (
	"embed"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"

	"github.com/gin-contrib/multitemplate"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/kakeetopius/qosm/internal/core/tc"
	"github.com/kakeetopius/qosm/web/db"
	"github.com/kakeetopius/qosm/web/routes"
	_ "modernc.org/sqlite"
)

//go:embed static
var staticFS embed.FS

type ServerOptions struct {
	Port            int
	DBPath          string
	SessionsEncKey  string
	SessionsHashKey string
}

func Run() error {
	router := gin.Default()

	renderer, err := createRenderer()
	if err != nil {
		return err
	}
	router.HTMLRender = renderer

	setUpSessionMgmt(router)
	addStaticRoutes(router)

	dbConn, err := db.Connect()
	if err != nil {
		return err
	}
	err = db.SetUp(dbConn)
	if err != nil {
		return err
	}

	htbCtx, err := setUpHTBContext()
	if err != nil {
		return err
	}
	settings, err := db.LoadSettings(dbConn)
	if err != nil {
		return err
	}

	ifaces, err := initNetInterface()
	if err != nil {
		return err
	}

	app := routes.ServerCtx{
		Logger:   slog.New(slog.NewJSONHandler(os.Stdout, nil)),
		DB:       dbConn,
		Ifaces:   ifaces,
		HTBCtx:   htbCtx,
		Settings: settings,
	}

	addRoutes(router, &app)

	router.Run()
	return nil
}

func setUpSessionMgmt(router *gin.Engine) {
	store := cookie.NewStore([]byte("cookie-key"))

	router.Use(sessions.Sessions("qosm-session", store))
}

func addRoutes(router *gin.Engine, app *routes.ServerCtx) {
	router.Use(ErrorHandlerHTML())

	router.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "hello from qosm")
	})

	auth := router.Group("/")
	auth.GET("/login", app.LoginPage)
	auth.POST("/login", app.LoginPost)

	admin := router.Group("/", AuthRequired(app), ErrorHandlerToast(app))
	admin.GET("/dashboard", app.DashboardPage)
	admin.GET("/rules", app.RulesPage)
	admin.GET("/analytics", app.AnalyticsPage)
	admin.GET("/logs", app.LogsPage)

	admin.GET("/settings", app.SettingsPage)
	admin.POST("/settings/system/save", app.PostSystemSettings)
	admin.POST("/settings/interface/save", app.PostInterfaceSettings)
	admin.POST("settings/dns/save", app.PostDNSSettings)
	admin.POST("settings/security/save", app.PostSecuritySettings)

	admin.POST("/rules/create", app.PostRules)

	admin.GET("/logout", app.Logout)
	admin.GET("/", app.DashboardPage)
}

func addStaticRoutes(router *gin.Engine) error {
	staticSubFS, err := fs.Sub(staticFS, "static/js")
	if err != nil {
		return err
	}
	router.StaticFS("/static/js", http.FS(staticSubFS))

	staticSubFS, err = fs.Sub(staticFS, "static/css")
	if err != nil {
		return err
	}
	router.StaticFS("/static/css", http.FS(staticSubFS))

	staticSubFS, err = fs.Sub(staticFS, "static/pictures")
	if err != nil {
		return err
	}
	router.StaticFS("/static/pictures", http.FS(staticSubFS))

	return nil
}

func createRenderer() (multitemplate.Renderer, error) {
	tmplSubFS, err := fs.Sub(staticFS, "static/templates")
	if err != nil {
		return nil, err
	}

	commonTemplates := []string{"partials/meta.tmpl", "partials/sidebar.tmpl", "partials/topbar.tmpl"}
	pages := []string{"dashboard", "rules", "analytics", "logs", "settings"}

	r := multitemplate.NewRenderer()

	for _, page := range pages {
		files := append([]string{"layout/base.tmpl", "pages/" + page + ".tmpl"}, commonTemplates...)
		r.AddFromFS(page, tmplSubFS, files...)
	}

	r.AddFromFS("login", tmplSubFS, "pages/login.tmpl", "partials/meta.tmpl", "partials/fail.tmpl")
	r.AddFromFS("fail", tmplSubFS, "partials/fail.tmpl")
	r.AddFromFS("toast_success", tmplSubFS, "partials/toast_success.tmpl")
	r.AddFromFS("toast_error", tmplSubFS, "partials/toast_error.tmpl")
	return r, nil
}

func setUpHTBContext() (*tc.HTBCtx, error) {
	htbCtx, err := tc.NewHTBCtx()
	if err != nil {
		return nil, err
	}

	err = htbCtx.InitHTBFilter(true)
	if err != nil {
		return nil, err
	}

	return htbCtx, nil
}

func initNetInterface() (map[string]routes.Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	netIfaces := make(map[string]routes.Interface, len(ifaces))
	for _, iface := range ifaces {
		enabled, err := tc.HasHTBQdisc(&iface)
		if err != nil {
			return nil, err
		}

		netIfaces[iface.Name] = routes.Interface{
			Interface: iface,
			Enabled:   enabled,
		}
	}

	return netIfaces, nil
}

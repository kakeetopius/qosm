package server

import (
	"database/sql"
	"embed"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kakeetopius/qosm/internal/db"
	"github.com/kakeetopius/qosm/internal/qos"
)

type ServerError struct {
	StatusCode int
	Err        error
}

func (e ServerError) Error() string {
	return e.Err.Error()
}

type Server struct {
	DB         *sql.DB
	QoSManager *qos.QoSManager
	Settings   *db.Settings
	Logger     *slog.Logger
}

func (app *Server) InitQoSManager(opts qos.Options) error {
	qosManager, err := qos.NewManager(opts)
	if err != nil {
		return err
	}

	err = qosManager.InitQoSClassifier(true)
	if err != nil {
		return err
	}
	app.QoSManager = qosManager

	err = app.QoSManager.InitSavedRules()
	if err != nil {
		return err
	}

	err = app.QoSManager.InitSavedInterfaceSettings()
	if err != nil {
		return err
	}

	return nil
}

func (app *Server) AddRoutes(router *gin.Engine) {
	router.Use(ErrorHandlerHTML())

	// login
	auth := router.Group("/")
	auth.GET("/login", app.LoginPage)
	auth.POST("/login", app.Login)

	// pages
	admin := router.Group("/", AuthRequired(app), ErrorHandlerToast(app))
	admin.GET("/dashboard", app.DashboardPage)
	admin.GET("/rules", app.RulesPage)
	admin.GET("/analytics", app.AnalyticsPage)
	admin.GET("/logs", app.LogsPage)

	// settingss
	admin.GET("/settings", app.SettingsPage)
	admin.POST("/settings/interfaces/:ifaceName", app.PostInterfaceSettings)
	admin.GET("/settings/interfaces/:ifaceName", app.GetInterfaceSettingsPopUp)
	admin.POST("/settings/dns/save", app.PostDNSSettings)
	admin.POST("/settings/security/save", app.PostSecuritySettings)

	// analytics
	admin.GET("/analytics/refresh", app.AnalyticsRefresh)

	// rules
	admin.POST("/rules/create", app.PostHostRules)
	admin.DELETE("/rules/:type/:id", app.DeleteHostRule)

	// logs
	admin.GET("/logs/filter", app.LogsFilter)
	admin.DELETE("/logs/delete", app.LogsDelete)

	// logout
	admin.GET("/logout", app.Logout)
	admin.GET("/", app.DashboardPage)
}

func (app *Server) AddStaticRoutes(router *gin.Engine, staticFS *embed.FS) error {
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

func (app *Server) CleanUp() {
	app.QoSManager.Close()
	app.DB.Close()
}

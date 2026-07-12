package server

import (
	"fmt"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/kakeetopius/qosm/internal/core/pam"
	"github.com/kakeetopius/qosm/internal/db"
)

func (app *Server) Login(ctx *gin.Context) {
	username := ctx.PostForm("username")
	password := ctx.PostForm("password")

	db.AddLog(app.DB, db.Log{
		EventType:   "INFO",
		Description: "login attempt for " + username,
	})

	if err := pam.AuthenticateUser(username, password); err != nil {
		db.AddErrorLog(app.DB, err, "")

		ctx.Error(ServerError{
			StatusCode: http.StatusOK,
			Err:        fmt.Errorf(" Invalid username or password"),
		})

		return
	}

	db.AddLog(app.DB, db.Log{
		EventType:   "INFO",
		Description: "login successfull for " + username,
	})
	session := sessions.Default(ctx)
	session.Options(sessions.Options{
		MaxAge:   app.Settings.SessionTimeout * 60,
		HttpOnly: true, // Prevent JavaScript access
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	})

	session.Set("username", username)
	session.Set("role", "administrator")
	session.Save()
	ctx.Header("HX-Redirect", "/dashboard")
	ctx.Status(http.StatusOK)
}

func (app *Server) LoginPage(c *gin.Context) {
	c.HTML(http.StatusOK, "login", gin.H{
		"Title": "Login - QoS Manager",
	})
}

func (app *Server) Logout(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("username").(string)

	session.Clear()
	session.Save()

	db.AddLog(app.DB, db.Log{
		EventType:   "INFO",
		Description: "logout for user " + username,
	})
	c.Redirect(http.StatusFound, "/login")
}

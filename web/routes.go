package web

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (app *ServerCtx) loginPost(ctx *gin.Context) {
	username := ctx.PostForm("username")
	password := ctx.PostForm("password")

	if username != "admin" || password != "1234" {
		ctx.Error(ServerError{
			StatusCode: http.StatusOK,
			Err:        errors.New(" Invalid username or password"),
		})

		return
	}

	ctx.Header("HX-Redirect", "/dashboard")
	ctx.Status(http.StatusOK)
}

func (app *ServerCtx) login(c *gin.Context) {
	c.HTML(http.StatusOK, "login", gin.H{
		"Title": "Login - QoS Manager",
	})
}

func (app *ServerCtx) dashboard(c *gin.Context) {
	c.HTML(http.StatusOK, "dashboard", gin.H{
		"Path": c.Request.URL.Path,
	})
}

func (app *ServerCtx) rules(c *gin.Context) {
	c.HTML(http.StatusOK, "rules", gin.H{
		"Path": c.Request.URL.Path,
	})
}

func (app *ServerCtx) analytics(c *gin.Context) {
	c.HTML(http.StatusOK, "analytics", gin.H{
		"Path": c.Request.URL.Path,
	})
}

func (app *ServerCtx) logs(c *gin.Context) {
	c.HTML(http.StatusOK, "logs", gin.H{
		"Path": c.Request.URL.Path,
	})
}

func (app *ServerCtx) settings(c *gin.Context) {
	c.HTML(http.StatusOK, "settings", gin.H{
		"Path": c.Request.URL.Path,
	})
}

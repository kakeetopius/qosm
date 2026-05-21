package web

import (
	"errors"
	"net/http"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"github.com/kakeetopius/qosm/web/routes"
)

func ErrorHandlerJSON() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Next()

		if len(ctx.Errors) == 0 {
			return
		}

		err := ctx.Errors.Last().Err

		var serverError routes.ServerError
		if errors.As(err, &serverError) {
			ctx.JSON(serverError.StatusCode, gin.H{
				"success": false,
				"message": serverError.Error(),
			})
		}
	}
}

func ErrorHandlerHTML() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Next()

		if len(ctx.Errors) == 0 {
			return
		}

		err := ctx.Errors.Last().Err

		var serverError routes.ServerError
		if errors.As(err, &serverError) {
			ctx.HTML(serverError.StatusCode, "fail", gin.H{
				"Error": serverError.Error(),
			})
		}
	}
}

func ErrorHandlerToast(app *routes.ServerCtx) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.Next()

		if len(ctx.Errors) == 0 {
			return
		}

		err := ctx.Errors.Last().Err

		app.Logger.Error("server_error", "Error", err.Error())

		ctx.HTML(http.StatusOK, "toast_error", gin.H{
			"Message": "Failed to apply settings: " + err.Error(),
		})
	}
}

func AuthRequired(app *routes.ServerCtx) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		session := sessions.Default(ctx)

		username := session.Get("username")

		if username == nil {
			if ctx.GetHeader("HX-Request") == "true" {
				ctx.Header("HX-Redirect", "/login")
				ctx.AbortWithStatus(http.StatusOK)
				return
			}
			ctx.Redirect(http.StatusFound, "/login")
			ctx.Abort()
			return
		}

		ctx.Next()
	}
}

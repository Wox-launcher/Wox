package main

import (
	"context"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"net/http"
	"time"
	"wox/plugin"
	"wox/util"
)

type ApiResponse struct {
	Status  int
	Data    any
	Message string
}

func NewApiResponseSuccessWithoutData() ApiResponse {
	return ApiResponse{
		Status:  http.StatusOK,
		Data:    nil,
		Message: "",
	}
}

func NewApiResponseSuccess(data any) ApiResponse {
	return ApiResponse{
		Status:  http.StatusOK,
		Data:    data,
		Message: "",
	}
}

func NewApiResponseError(msg string) ApiResponse {
	return ApiResponse{
		Status:  http.StatusInternalServerError,
		Data:    nil,
		Message: msg,
	}
}

func ServeAndWait(ctx context.Context, port int) {
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.MaxMultipartMemory = 100 << 20
	router.Use(gin.RecoveryWithWriter(util.GetLogger().GetWriter()))
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{},
		AllowMethods:     []string{"PUT", "PATCH", "POST", "GET"},
		AllowHeaders:     []string{"Origin", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		AllowOriginFunc: func(origin string) bool {
			return true
		},
		MaxAge: 12 * time.Hour,
	}))

	router.GET("/query", func(c *gin.Context) {
		token := c.Query("token")
		if token == "" {
			c.JSON(http.StatusOK, NewApiResponseError("token parameter is required"))
			return
		}

		results := plugin.GetPluginManager().Query(util.NewTraceContext(), plugin.NewQuery(token))
		c.JSON(http.StatusOK, NewApiResponseSuccess(results))
	})

	util.GetLogger().Info(ctx, fmt.Sprintf("rest ServeAndWait at：http://localhost:%d", port))
	err := router.Run(fmt.Sprintf("localhost:%d", port))
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to start rest ServeAndWait: %s", err.Error()))
	}
}

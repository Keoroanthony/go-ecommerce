package main

import (
    "os"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"

	"github.com/Keoroanthony/go-ecommerce/internal/auth"
	"github.com/Keoroanthony/go-ecommerce/internal/db"
	"github.com/Keoroanthony/go-ecommerce/internal/handlers"
)

func main() {

    db.Init()
    auth.Init()

    r := gin.Default()

    // ── session store ──
	store := cookie.NewStore([]byte(getEnv("SESSION_SECRET", "change-me")))
	r.Use(sessions.Sessions("gosess", store))

    // ── public endpoints ──
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	r.GET("/auth/login", auth.Login)
	r.GET("/auth/callback", auth.Callback)

    // ── protected API ──
    api := r.Group("/api")
    api.Use(auth.RequireAuth())
    {
        api.POST("/categories", handlers.CreateCategory)
        api.POST("/products", handlers.CreateProduct)
        api.GET("/products/average", handlers.GetAveragePrice)
        api.POST("/orders", handlers.CreateOrder)
    }

    r.Run(":8080")
}


func getEnv(key, fallback string) string {

	if value, exists := os.LookupEnv(key); exists {
		return value
	}

	return fallback
}
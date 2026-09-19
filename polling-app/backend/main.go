package main

import (
	"log"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"pollingapp/config"
	"pollingapp/db"
	"pollingapp/handlers"
	"pollingapp/middleware"
	"pollingapp/ws"
)

func main() {
	cfg := config.Load()

	mongo := db.NewMongo(cfg.MongoURI, cfg.MongoDBName)
	redisClient := db.NewRedis(cfg.RedisAddr, cfg.RedisPass)
	hub := ws.NewHub(redisClient)

	authHandler := &handlers.AuthHandler{DB: mongo.DB, Cfg: cfg}
	pollHandler := &handlers.PollHandler{DB: mongo.DB, Redis: redisClient, Hub: hub, Cfg: cfg}

	r := gin.Default()

	corsCfg := cors.DefaultConfig()
	corsCfg.AllowOrigins = []string{cfg.AllowOrigin}
	corsCfg.AllowHeaders = []string{"Origin", "Content-Type", "Authorization"}
	corsCfg.AllowCredentials = true
	r.Use(cors.New(corsCfg))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	api := r.Group("/api")
	{
		api.POST("/signup", authHandler.Signup)
		api.POST("/login", authHandler.Login)

		api.GET("/polls/:id", pollHandler.GetPoll)
		api.GET("/polls/:id/results", pollHandler.GetResults)
		api.POST("/polls/:id/vote", pollHandler.Vote)
		api.GET("/polls/:id/watch", pollHandler.WatchResults) // websocket upgrade

		authed := api.Group("/")
		authed.Use(middleware.RequireAuth(cfg.JWTSecret))
		{
			authed.POST("/polls", pollHandler.CreatePoll)
			authed.GET("/my-polls", pollHandler.ListMyPolls)
			authed.POST("/polls/:id/close", pollHandler.ClosePoll)
		}
	}

	log.Printf("listening on :%s", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

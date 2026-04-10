package main

import (
	"flag"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rowlys/panjang-umur-backend/internal/config"
	"github.com/rowlys/panjang-umur-backend/internal/database"
	"github.com/rowlys/panjang-umur-backend/internal/handlers"
	"github.com/rowlys/panjang-umur-backend/internal/middlewares"
)

func main() {
	seederFlag := flag.Bool("seed", false, "Seed the database with initial data")
	flag.Parse()

	config.LoadConfig()
	database.Connect()
	
	if *seederFlag {
		database.Seed()
		return
	}

	router := gin.Default()

	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	api := router.Group("/api")
	{
		api.POST("/register", handlers.Register)
		api.POST("/login", handlers.Login)

		protected := api.Group("/")
		protected.Use(middlewares.RequireAuth) 
		{
			protected.GET("/challenges", handlers.GetAllChallenges)
			protected.GET("/challenges/:id", handlers.GetChallengeByID)
			protected.GET("/users/group/:groupId/challenges", handlers.GetMyGroupChallenges)
			protected.GET("/users/group/:groupId/created-challenges", handlers.GetMyGroupCreatedChallenges)

			protected.POST("/challenges/:groupId/create", handlers.CreateChallenge)

			protected.PATCH("/challenges/:challengeId/submit", handlers.SubmitChallenge)
			protected.PATCH("/challenges/:challengeId/approve", handlers.ApproveChallenge)
		}
	}

	port := config.GetEnv("PORT", "8080")
	log.Println("Starting server on :" + port)

	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}

}
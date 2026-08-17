// @title Panjang Umur API
// @version 1.0
// @description A health and longevity challenge API
// @BasePath /api
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Use "Bearer <token>"
// @host localhost:8080
package main

import (
	"flag"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/rowlys/panjang-umur-backend/internal/config"
	"github.com/rowlys/panjang-umur-backend/internal/database"
	"github.com/rowlys/panjang-umur-backend/internal/domain/challenge"
	"github.com/rowlys/panjang-umur-backend/internal/domain/chat"
	"github.com/rowlys/panjang-umur-backend/internal/domain/friendship"
	"github.com/rowlys/panjang-umur-backend/internal/domain/reward"
	"github.com/rowlys/panjang-umur-backend/internal/domain/transaction"
	"github.com/rowlys/panjang-umur-backend/internal/domain/user"
	"github.com/rowlys/panjang-umur-backend/internal/pkg/image_storage"
	"github.com/rowlys/panjang-umur-backend/internal/middlewares"

	_ "github.com/rowlys/panjang-umur-backend/docs"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
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

	// External services
	imageStorageService := image_storage.NewService()

	// Internal services and handlers

	userRepo := user.NewRepository(database.DB)
	userService := user.NewService(userRepo)
	userHandler := user.NewHandler(userService)

	transactionRepo := transaction.NewRepository(database.DB)
	transactionService := transaction.NewService(transactionRepo)
	transactionHandler := transaction.NewHandler(transactionService)

	friendshipRepo := friendship.NewRepository(database.DB)
	friendshipService := friendship.NewService(friendshipRepo)
	friendshipHandler := friendship.NewHandler(friendshipService)
	
	challengeRepo := challenge.NewRepository(database.DB)
	challengeService := challenge.NewService(challengeRepo, friendshipService, transactionService, imageStorageService)
	challengeHandler := challenge.NewHandler(challengeService)
	
	rewardRepo := reward.NewRepository(database.DB)
	rewardService := reward.NewService(rewardRepo, userService, friendshipService, transactionService)
	rewardHandler := reward.NewHandler(rewardService)
	
	chatHub := chat.NewHub()
	chatRepo := chat.NewRepository(database.DB)
	chatService := chat.NewService(chatRepo, friendshipService, chatHub)
	chatHandler := chat.NewHandler(chatService, chatHub)
	
	router := gin.Default()

	router.Use(cors.New(cors.Config{
		AllowOriginFunc: func(origin string) bool {
			return strings.HasPrefix(origin, "http://localhost:") ||
				strings.HasPrefix(origin, "http://127.0.0.1:")
		},
		AllowMethods:     []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	router.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})

	api := router.Group("/api")
	{
		public := api.Group("/auth")
		{
			userHandler.RegisterAuthRoutes(public)
		}

		wsGroup := api.Group("/chat")
		wsGroup.Use(middlewares.RequireAuthWS(config.GetEnv("JWT_SECRET", "")))
		wsGroup.GET("/ws", chatHandler.ServeWS)

		protected := api.Group("/")
		protected.Use(middlewares.RequireAuth(config.GetEnv("JWT_SECRET", "")))
		{
			userHandler.RegisterProtectedRoutes(protected.Group("/users"))
			challengeHandler.RegisterRoutes(protected.Group("/challenges"))
			friendshipHandler.RegisterRoutes(protected.Group("/friends"))
			transactionHandler.RegisterRoutes(protected.Group("/transactions"))
			rewardHandler.RegisterRoutes(protected.Group("/rewards"))
			chatHandler.RegisterRoutes(protected.Group("/chat"))
		}
	}

	port := config.GetEnv("PORT", "8080")
	log.Println("Starting server on :" + port)

	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to run server: %v", err)
	}

}

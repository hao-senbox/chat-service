package main

import (
	"chat-service/config"
	"chat-service/internal/api"
	"chat-service/internal/repository"
	"chat-service/internal/service"
	"chat-service/internal/socket"
	"chat-service/pkg/consul"
	"chat-service/pkg/zap"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Initialize configuration
	cfg := config.LoadConfig()

	// Initialize logger
	logger, err := zap.New(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}

	consulConn := consul.NewConsulConn(logger, cfg)
	consulClient := consulConn.Connect()
	defer consulConn.Deregister()

	// Connect to MongoDB
	mongoClient, err := connectToMongoDB(cfg.MongoURI)
	if err != nil {
		logger.Fatalf("Failed to connect to MongoDB: %v", err)
	}

	defer func() {
		if err := mongoClient.Disconnect(context.Background()); err != nil {
			logger.Fatal(err)
		}
	}()

	groupCollection := mongoClient.Database(cfg.MongoDB).Collection("group")
	groupMemberCollection := mongoClient.Database(cfg.MongoDB).Collection("group_member")
	messagesCollection := mongoClient.Database(cfg.MongoDB).Collection("messages")
	userOnlineCollection := mongoClient.Database(cfg.MongoDB).Collection("user_online")
	messagesReactCollection := mongoClient.Database(cfg.MongoDB).Collection("messages_react")
	messagesReadCollection := mongoClient.Database(cfg.MongoDB).Collection("messages_read")
	messagesVoteCollection := mongoClient.Database(cfg.MongoDB).Collection("messages_vote")
	messagesReadRepository := repository.NewReadMessageRepository(messagesReadCollection)
	if err := messagesReadRepository.EnsureIndexes(context.Background()); err != nil {
		log.Fatalf("Failed to create indexes: %v", err)
	}
	userOnlineRepository := repository.NewUserOnlineRepository(userOnlineCollection)
	userService := service.NewUserService(consulClient, userOnlineRepository)
	messagesReactRepository := repository.NewMessageReactRepository(messagesReactCollection)
	messagesRepository := repository.NewChatRepository(messagesCollection, groupMemberCollection)
	groupRepository := repository.NewGroupRepository(groupCollection, groupMemberCollection, nil)
	groupMemberRepository := repository.NewGroupMemberRepository(groupMemberCollection, groupRepository)
	groupService := service.NewGroupService(groupRepository, groupMemberRepository, messagesRepository, userService, nil, messagesReactRepository)
	messageService := service.NewChatService(consulClient ,messagesRepository, messagesReadRepository, groupService, userService, messagesReactRepository)
	
	messageVoteRepository := repository.NewVoteRepository(messagesVoteCollection)
	voteService := service.NewVoteService(messageVoteRepository)
	

	groupService.SetMessageService(messageService)
	groupRepository.SetGroupMemberRepo(groupMemberRepository)

	hub := socket.NewHub(messageService, userService, userOnlineRepository, groupService, voteService)
	go hub.Run()
	// Set up router with Gin
	router := gin.Default()
	api.RegisterSocketRouters(router, hub, messageService)
	api.RegisterGroupRouters(router, groupService)
	api.RegisterChatRouters(router, messageService)

	// router.LoadHTMLGlob("web/templates/*")
	// router.GET("/", func(c *gin.Context) {
	// 	c.HTML(http.StatusOK ,"home.html", gin.H{
	// 	})
	// })

	// router.GET("/chat/:group_id", func(c *gin.Context) {
	// 	groupID := c.Param("group_id")
		
	// 	// Pass these values to the template
	// 	c.HTML(http.StatusOK, "chat.html", gin.H{
	// 		"groupID": groupID,
	// 		"title": "Group Chat",
	// 	})	
	// })


	// Initialize HTTP server
	server := &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: router,	
	}

	// Run server in a separate goroutine
	go func() {
		logger.Infof("Server running on port %s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Error starting server: %v", err)
		}
	}()

	// Set up graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Fatalf("Error shutting down server: %v", err)
	}
	logger.Info("Server stopped")
}

func connectToMongoDB(uri string) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		log.Println("Failed to connect to MongoDB")
		return nil, err
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		log.Println("Failed to ping to MongoDB")
		return nil, err
	}

	log.Println("Successfully connected to MongoDB")
	return client, nil
}


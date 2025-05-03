package api

import (
	"chat-service/internal/service"
	"chat-service/internal/socket"

	"github.com/gin-gonic/gin"
)

func RegisterSocketRouters(r *gin.Engine, hub *socket.Hub) {
	r.GET("/ws/:user_id/:group_id", socket.ServeWsGin(hub))
}

func RegisterChatRouters(r *gin.Engine, chatService service.ChatService, userService service.UserService) {

	handlers := NewChatService(chatService, userService)

	chatGroup := r.Group("/api/v1/chat")
	{
		chatGroup.GET("/:group_id", handlers.GetGroupMessages)
		chatGroup.GET("/check/:user_id/:group_id", handlers.IsUserInGroup)
	}
}

func RegisterGroupRouters(r *gin.Engine, groupService service.GroupService) {
	
	handlers := NewGroupService(groupService)

	groupGroup := r.Group("/api/v1/group")
	{
		//Group 
		groupGroup.GET("", handlers.GetAllGroups)
		groupGroup.POST("", handlers.CreateGroup)
		

		//Group User
		groupGroup.POST("/user", handlers.CreateGroupUser)
		groupGroup.GET("/user/:user_id", handlers.GetUserGroups)
	}
}
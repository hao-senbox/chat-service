package api

import (
	"chat-service/internal/service"
	"chat-service/internal/socket"
	"github.com/gin-gonic/gin"
)

func RegisterSocketRouters(r *gin.Engine, hub *socket.Hub, chatService service.ChatService) {
	r.GET("/ws/:user_id/:group_id", UserInGroupMiddleware(chatService), socket.ServeWsGin(hub))
}

func RegisterChatRouters(r *gin.Engine, chatService service.ChatService) {

	handlers := NewChatService(chatService)

	chatGroup := r.Group("/api/v1/chat")
	{
		chatGroup.GET("/:group_id", handlers.GetGroupMessages)
		chatGroup.GET("/check/:user_id/:group_id", handlers.IsUserInGroup)
		chatGroup.GET("/dowload/:group_id", handlers.DownloadGroupMessages)
	}
}

func RegisterGroupRouters(r *gin.Engine, groupService service.GroupService) {
	
	handlers := NewGroupService(groupService)

	groupGroup := r.Group("/api/v1/group")
	{
		//Group 
		groupGroup.GET("", handlers.GetAllGroups)
		groupGroup.GET("/:group_id", handlers.GetGroupDetail)
		groupGroup.POST("", handlers.CreateGroup)
		groupGroup.PUT("/:group_id", handlers.UpdateGroup)
		groupGroup.DELETE("/:group_id", handlers.DeleteGroup)
		

		//Group User
		groupGroup.POST("/user", handlers.AddUserToGroup)
		groupGroup.GET("/user/:user_id", handlers.GetUserGroups)
		groupGroup.DELETE("/user/:group_id", handlers.RemoveUserFromGroup)
	}
}
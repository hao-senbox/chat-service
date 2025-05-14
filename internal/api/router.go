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
		//Chat
		chatGroup.GET("/:group_id", handlers.GetGroupMessages)
		chatGroup.GET("/check/:user_id/:group_id", handlers.IsUserInGroup)
		chatGroup.GET("/download/:group_id", handlers.DownloadGroupMessages)
		chatGroup.GET("/information/user/:user_id", handlers.GetUserInformation)
	}
}

func RegisterGroupRouters(r *gin.Engine, groupService service.GroupService) {
	
	handlers := NewGroupService(groupService)

	groupGroup := r.Group("/api/v1/group")
	{
		//Group 
		groupGroup.GET("", handlers.GetAllGroups)
		groupGroup.GET("/:group_id", handlers.GetGroupDetail)
		groupGroup.GET("/count/keywod", handlers.CountKeywordAllGroups)
		groupGroup.POST("", handlers.CreateGroup)
		groupGroup.POST("/generate/qrcode", handlers.GenerateGroupQrCode)
		groupGroup.PUT("/:group_id", handlers.UpdateGroup)
		groupGroup.DELETE("/:group_id", handlers.DeleteGroup)
		

		//Group User
		groupGroup.POST("/user", handlers.AddUserToGroup)
		groupGroup.POST("join_group/by_qrcode", handlers.JoinGroupByQrCode)
		groupGroup.GET("/user/:user_id", handlers.GetUserGroups)
		groupGroup.DELETE("/user/:group_id", handlers.RemoveUserFromGroup)
	}
}
package api

import (
	"chat-service/internal/models"
	"chat-service/internal/service"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

type ChatHandlers struct {
	chatService service.ChatService
	userService service.UserService
}

func NewChatService(chatService service.ChatService, userService service.UserService) *ChatHandlers {
	return &ChatHandlers{
		chatService: chatService,
		userService: userService,
	}
}

func (h *ChatHandlers) GetGroupMessages(c *gin.Context) {
	groupID := c.Param("group_id")

	if groupID == "" {
		SendError(c, http.StatusBadRequest, fmt.Errorf("group ID be not empty"), models.ErrInvalidRequest)
		return
	}

	messages, err := h.chatService.GetGroupMessages(c, groupID)
	if err != nil {
		SendError(c, http.StatusInternalServerError, err, models.ErrInvalidOperation)
		return
	}

	var enrichedMessages []models.MessageWithUser

	userCache := make(map[string]*models.UserInfor)

	for _, msg := range messages {
		var userInfo *models.UserInfor
		if cached, ok := userCache[msg.SenderID]; ok {
			userInfo = cached
		} else {
			fmt.Printf("Fetching user info for %s\n", msg.SenderID)
			fetchedInfo, err := h.userService.GetUserInfor(msg.SenderID)
			if err != nil {
				fmt.Printf("Failed to get user info for %s: %v\n", msg.SenderID, err)
				fetchedInfo = &service.UserInfor{} 
			}
			userInfo = &models.UserInfor{
				UserID:   fetchedInfo.UserID,
				UserName: fetchedInfo.UserName,
				Avartar:  fetchedInfo.Avartar,
			}
			userCache[msg.SenderID] = userInfo
		}

		enrichedMessages = append(enrichedMessages, models.MessageWithUser{
			SenderID:    msg.SenderID,
			Content:     msg.Content,
			CreatedAt:   msg.CreatedAt,
			SenderInfor: userInfo,
		})
	}

	SendSuccess(c, http.StatusOK, "Get group messages successfully", enrichedMessages)
}

func (h *ChatHandlers) IsUserInGroup(c *gin.Context) {

	userID := c.Param("user_id")
	groupID := c.Param("group_id")

	if userID == "" || groupID == "" {
		SendError(c, http.StatusBadRequest, fmt.Errorf("user ID or group ID be not empty"), models.ErrInvalidRequest)
		return
	}

	isInGroup, err := h.chatService.IsUserInGroup(c, userID, groupID)
	if err != nil {
		SendError(c, http.StatusInternalServerError, err, models.ErrInvalidOperation)
		return
	}

	SendSuccess(c, http.StatusOK, "Check user in group successfully", isInGroup)
}

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
}

func NewChatService(chatService service.ChatService) *ChatHandlers {
	return &ChatHandlers{
		chatService: chatService,
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

	SendSuccess(c, http.StatusOK, "Get group messages successfully", messages)
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

func (h *ChatHandlers) DownloadGroupMessages(c *gin.Context) {

	groupID := c.Param("group_id")

	if groupID == "" {
		SendError(c, http.StatusBadRequest, fmt.Errorf("group ID be not empty"), models.ErrInvalidRequest)
		return
	}

	err := h.chatService.DownloadGroupMessages(c, groupID)
	if err != nil {
		SendError(c, http.StatusInternalServerError, err, models.ErrInvalidOperation)
		return
	}
}

func (h *ChatHandlers) GetUserInformation(c *gin.Context) {

	userID := c.Param("user_id")

	if userID == "" {
		SendError(c, http.StatusBadRequest, fmt.Errorf("user ID be not empty"), models.ErrInvalidRequest)
		return
	}

	user, err := h.chatService.GetUserInformation(c, userID)
	if err != nil {
		SendError(c, http.StatusInternalServerError, err, models.ErrInvalidOperation)
		return
	}

	SendSuccess(c, http.StatusOK, "Get user information successfully", user)
}
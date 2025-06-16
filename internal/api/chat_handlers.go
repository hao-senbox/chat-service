package api

import (
	"chat-service/internal/models"
	"chat-service/internal/service"
	"chat-service/pkg/constants"
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"time"

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

	var req models.Panigination

	if err := c.ShouldBindQuery(&req); err != nil {
		SendError(c, http.StatusBadRequest, err, models.ErrInvalidRequest)
		return
	}

	if req.Page == 0 {
		req.Page = 1
	}

	if req.Limit == 0 {
		req.Limit = 30
	}

	groupID := c.Param("group_id")

	if groupID == "" {
		SendError(c, http.StatusBadRequest, fmt.Errorf("group ID be not empty"), models.ErrInvalidRequest)
		return
	}

	token, ok := c.Get(constants.Token)
	if !ok {
		SendError(c, http.StatusForbidden, errors.New("unauthorized"), models.ErrInvalidRequest)
		return
	}
	userID := c.Value("user_id").(string)
	if userID == "" {
		SendError(c, http.StatusBadRequest, fmt.Errorf("user ID be not empty"), models.ErrInvalidRequest)
		return
	}

	ctx := context.WithValue(c, constants.TokenKey, token)
	messages, totalItems, err := h.chatService.GetGroupMessages(ctx, groupID, &userID, nil, nil, &req)
	if err != nil {
		SendError(c, http.StatusInternalServerError, err, models.ErrInvalidOperation)
		return
	}

	totalPages := int(math.Ceil(float64(totalItems) / float64(req.Limit)))

	data := models.PaniginationResponse{
		TotalItems: totalItems,
		TotalPages: totalPages,
		Limit:      req.Limit,
		Page:       req.Page,
		Data:       messages,
	}

	SendSuccess(c, http.StatusOK, "Get group messages successfully", data)
}

func (h *ChatHandlers) IsUserInGroup(c *gin.Context) {

	groupID := c.Param("group_id")
	userID := c.Value("user_id").(string)
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

	var fromTime, toTime *time.Time

	fromStr := c.Query("from")
	if fromStr != "" {
		t, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			SendError(c, http.StatusBadRequest, fmt.Errorf("invalid from time"), models.ErrInvalidRequest)
			return
		}
		fromTime = &t
	}

	toStr := c.Query("to")
	if toStr != "" {
		t, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			SendError(c, http.StatusBadRequest, fmt.Errorf("invalid to time"), models.ErrInvalidRequest)
			return
		}
		toTime = &t
	}

	groupID := c.Param("group_id")

	if groupID == "" {
		SendError(c, http.StatusBadRequest, fmt.Errorf("group ID be not empty"), models.ErrInvalidRequest)
		return
	}

	err := h.chatService.DownloadGroupMessages(c, groupID, fromTime, toTime)
	if err != nil {
		SendError(c, http.StatusInternalServerError, err, models.ErrInvalidOperation)
		return
	}
}

func (h *ChatHandlers) GetUserInformation(c *gin.Context) {

	userID := c.Value("user_id").(string)

	token, ok := c.Get(constants.Token)
	if !ok {
		SendError(c, http.StatusForbidden, errors.New("unauthorized"), models.ErrInvalidRequest)
		return
	}

	ctx := context.WithValue(c, constants.TokenKey, token)
	
	if userID == "" {
		SendError(c, http.StatusBadRequest, fmt.Errorf("user ID be not empty"), models.ErrInvalidRequest)
		return
	}

	user, err := h.chatService.GetUserInformation(ctx, userID)
	if err != nil {
		SendError(c, http.StatusInternalServerError, err, models.ErrInvalidOperation)
		return
	}

	SendSuccess(c, http.StatusOK, "Get user information successfully", user)
}

func (h *ChatHandlers) GetReactMessages(c *gin.Context) {

	messageID := c.Param("message_id")
	groupID := c.Param("group_id")

	messages, err := h.chatService.GetMessageReacts(c, messageID, groupID)
	if err != nil {
		SendError(c, http.StatusInternalServerError, err, models.ErrInvalidOperation)
		return
	}

	SendSuccess(c, http.StatusOK, "Get react messages successfully", messages)
}

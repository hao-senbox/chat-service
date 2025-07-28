package api

import (
	"chat-service/internal/models"
	"chat-service/internal/service"
	"chat-service/pkg/constants"
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type EmergencyHandlers struct {
	emergencyService service.EmergencyService
}

func NewEmergencyService(emergencyService service.EmergencyService) *EmergencyHandlers {
	return &EmergencyHandlers{
		emergencyService: emergencyService,
	}
}

func (h *EmergencyHandlers) CreateEmergency(c *gin.Context) {

	token, ok := c.Get(constants.Token)
	if !ok {
		SendError(c, http.StatusForbidden, errors.New("unauthorized"), models.ErrInvalidRequest)
		return
	}

	ctx := context.WithValue(c, constants.TokenKey, token)
	var req models.EmergencyRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		SendError(c, http.StatusBadRequest, err, models.ErrInvalidRequest)
		return
	}

	if err := h.emergencyService.CreateEmergency(ctx, &req); err != nil {
		SendError(c, http.StatusInternalServerError, err, models.ErrInvalidOperation)
		return
	}

	SendSuccess(c, http.StatusOK, "Emergency created successfully", nil)

}

func (h *EmergencyHandlers) GetNotificationsUser(c *gin.Context) {

	userID := c.Value("user_id").(string)
	if userID == "" {
		SendError(c, http.StatusBadRequest, nil, models.ErrInvalidRequest)
		return
	}

	token, ok := c.Get(constants.Token)
	if !ok {
		SendError(c, http.StatusForbidden, errors.New("unauthorized"), models.ErrInvalidRequest)
		return
	}

	ctx := context.WithValue(c, constants.TokenKey, token)

	notifications, err := h.emergencyService.GetNotificationsUser(ctx, userID)
	if err != nil {
		SendError(c, http.StatusInternalServerError, err, models.ErrInvalidOperation)
		return
	}

	SendSuccess(c, http.StatusOK, "Get notifications successfully", notifications)

}

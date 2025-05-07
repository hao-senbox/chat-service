package api

import (
	"chat-service/internal/models"
	"chat-service/internal/service"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

func UserInGroupMiddleware(chatService service.ChatService) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.Param("user_id")
		groupID := c.Param("group_id")
		if userID == "" || groupID == "" {
			SendError(c, http.StatusBadRequest, fmt.Errorf("user ID or group ID be not empty"), models.ErrInvalidRequest)
			return
		}
		
		check, err := chatService.IsUserInGroup(c, userID, groupID)
		if err != nil {
			SendError(c, http.StatusInternalServerError, err, models.ErrInvalidOperation)
			return
		}

		if !check {
			SendError(c, http.StatusBadRequest, fmt.Errorf("user is not in group"), models.ErrInvalidRequest)
			return
		}
		c.Next()
	}
}
package api

import (
	"chat-service/internal/models"
	"chat-service/internal/service"
	"chat-service/pkg/constants"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
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

func Secured() gin.HandlerFunc {
	return func(context *gin.Context) {
		authorizationHeader := context.GetHeader("Authorization")

		if len(authorizationHeader) == 0 {
			context.AbortWithStatus(http.StatusForbidden)
			return
		}
		
		if !strings.HasPrefix(authorizationHeader, "Bearer ") {
			context.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		tokenString := strings.Split(authorizationHeader, " ")[1]

		token, _, _ := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})

		if claims, ok := token.Claims.(jwt.MapClaims); ok {
			if userId, ok := claims[constants.UserID].(string); ok {
				context.Set(constants.UserID, userId)
			}
		}

		context.Set(constants.Token, tokenString)
		context.Next()
	}
}

func WebsocketSecured() gin.HandlerFunc {
	return func(c *gin.Context) {
		
		token := c.Query(constants.Token)

		if len(token) == 0 {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}

		t, _, _ := new(jwt.Parser).ParseUnverified(token, jwt.MapClaims{})

		if claims, ok := t.Claims.(jwt.MapClaims); ok {
			if userId, ok := claims[constants.UserID].(string); ok {
				c.Set(constants.UserID, userId)
			} else {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
		}

		c.Set(constants.Token, token)
		c.Next()
	}
}
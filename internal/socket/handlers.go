package socket

import (
	"chat-service/pkg/constants"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func ServeWsGin(hub *Hub) gin.HandlerFunc {
	return func(c *gin.Context) {
		groupID := c.Param("group_id")

		if groupID == "" {
			log.Printf("Invalid user ID or group ID")
			c.JSON(http.StatusBadRequest, gin.H{"error": "Missing user_id or group_id"})
			return
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("Error upgrading to WebSocket: %v", err)
			return
		}

		userID, ok := c.Get(constants.UserID)
		if !ok {
			log.Printf("User ID not found in context")
			return
		}

		token, ok := c.Get(constants.Token)
		if !ok {
			log.Printf("User ID not found in context")
			return
		}

		client := &Client{
			hub:     hub,
			conn:    conn,
			send:    make(chan []byte, 512),
			userID:  userID.(string),
			groupID: groupID,
			token:   token.(string),
		}

		hub.registerChannel <- client

		// Bắt đầu goroutines cho client
		go client.writePump()
		go client.readPump()

		log.Printf("WebSocket connection established for user %s in group %s", userID, groupID)
	}
}

package socket

import (
    "log"
    "net/http"
    "github.com/gin-gonic/gin"
)

func ServeWsGin(hub *Hub) gin.HandlerFunc {
    return func(c *gin.Context) {
        groupID := c.Param("group_id")
        userID := c.Param("user_id")
        
        if userID == "" || groupID == "" {
            log.Printf("Invalid user ID or group ID")
            c.Status(http.StatusBadRequest)
            return
        }
        
        // Ghi log khi có kết nối mới
        log.Printf("New WebSocket connection request from user %s to group %s", userID, groupID)
        
        conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
        if err != nil {
            log.Printf("Error upgrading to WebSocket: %v", err)
            return
        }
        
        client := &Client{
            hub: hub,
            conn: conn,
            send: make(chan []byte, 512),
            userID: userID,
            groupID: groupID,
        }
        
        // Đăng ký client với hub
        hub.register <- client
        
        // Bắt đầu goroutines cho client
        go client.writePump()
        go client.readPump()
        
        log.Printf("WebSocket connection established for user %s in group %s", userID, groupID)
    }
}


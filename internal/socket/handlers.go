package socket

import (
    "log"
    "net/http"
    "strings"
    "github.com/gin-gonic/gin"
    "github.com/golang-jwt/jwt/v5"
)

// Thêm function validate token
func validateToken(tokenString string) (string, bool) {
    if tokenString == "" {
        return "", false
    }
    
    token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
    if err != nil {
        log.Printf("Error parsing token: %v", err)
        return "", false
    }
    
    if claims, ok := token.Claims.(jwt.MapClaims); ok {
        if userID, ok := claims["user_id"].(string); ok {
            return userID, true
        }
    }
    
    return "", false
}

func ServeWsGin(hub *Hub) gin.HandlerFunc {
    return func(c *gin.Context) {
        groupID := c.Param("group_id")
        userID := c.Param("user_id")
        
        // Lấy token từ query parameter (vì WebSocket không support custom headers)
        token := c.Query("token")
        
        // Nếu không có token trong query, thử lấy từ Authorization header
        if token == "" {
            authHeader := c.GetHeader("Authorization")
            if strings.HasPrefix(authHeader, "Bearer ") {
                token = strings.TrimPrefix(authHeader, "Bearer ")
            }
        }
        
        if userID == "" || groupID == "" {
            log.Printf("Invalid user ID or group ID")
            c.JSON(http.StatusBadRequest, gin.H{"error": "Missing user_id or group_id"})
            return
        }
        
        if token == "" {
            log.Printf("Missing token")
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Missing authentication token"})
            return
        }
        
        // Validate token và lấy user ID từ token
        tokenUserID, valid := validateToken(token)
        if !valid {
            log.Printf("Invalid token")
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
            return
        }
        
        // Kiểm tra user ID từ token có khớp với user ID từ URL không
        if tokenUserID != userID {
            log.Printf("Token user ID %s doesn't match URL user ID %s", tokenUserID, userID)
            c.JSON(http.StatusForbidden, gin.H{"error": "Token user ID mismatch"})
            return
        }
        
        log.Printf("New WebSocket connection request from user %s to group %s", userID, groupID)
        
        conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
        if err != nil {
            log.Printf("Error upgrading to WebSocket: %v", err)
            return
        }
        
        client := &Client{
            hub:     hub,
            conn:    conn,
            send:    make(chan []byte, 512),
            userID:  userID,
            groupID: groupID,
            token:   token, // Lưu token vào client
        }
        
        // Đăng ký client với hub
        hub.register <- client
        
        // Bắt đầu goroutines cho client
        go client.writePump()
        go client.readPump()
        
        log.Printf("WebSocket connection established for user %s in group %s", userID, groupID)
    }
}
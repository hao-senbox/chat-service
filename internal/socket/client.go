package socket

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
    // Tăng thời gian chờ viết để tránh timeout
    writeWait = 15 * time.Second
    // Tăng thời gian chờ pong để cho phép kết nối kém ổn định
    pongWait = 120 * time.Second
    // Đảm bảo thời gian ping nhỏ hơn thời gian pong
    pingPeriod = (pongWait * 8) / 10
    // Tăng kích thước tin nhắn tối đa
    maxMessageSize = 4096
)

var (
    newLine = []byte{'\n'}
    space = []byte{' '}
)

var upgrader = websocket.Upgrader{
    ReadBufferSize: 4096,
    WriteBufferSize: 4096,
    // Thêm CheckOrigin để cho phép kết nối từ bất kỳ nguồn nào
    CheckOrigin: func(r *http.Request) bool {
        return true
    },
}

type MessageType struct {
    Type string `json:"type,omitempty"`
}

type Client struct {
    hub *Hub
    conn *websocket.Conn
    send chan []byte
    userID string
    groupID string
}

func (c *Client) readPump() {
    defer func() {
        log.Printf("Client %s disconnected from group %s", c.userID, c.groupID)
        c.hub.unregister <- c
        c.conn.Close()
    }()

    c.conn.SetReadLimit(maxMessageSize)
    c.conn.SetReadDeadline(time.Now().Add(pongWait))
    c.conn.SetPongHandler(func(string) error { 
        // Ghi log khi nhận được pong để debug
        // log.Printf("Received pong from client %s", c.userID)
        c.conn.SetReadDeadline(time.Now().Add(pongWait))
        return nil 
    })

    for {
        _, message, err := c.conn.ReadMessage()
        if err != nil {
            if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
                log.Printf("WebSocket error for client %s: %v", c.userID, err)
            } else {
                log.Printf("Expected close for client %s: %v", c.userID, err)
            }
            break
        }

        var msgType MessageType
        if err := json.Unmarshal(message, &msgType); err == nil && msgType.Type == "ping" {
            // log.Printf("Received ping from client %s", c.userID)
            
            // Trả lời pong ngay lập tức
            pongMessage, _ := json.Marshal(map[string]string{"type": "pong"})
            c.conn.SetWriteDeadline(time.Now().Add(writeWait))
            err := c.conn.WriteMessage(websocket.TextMessage, pongMessage)
            if err != nil {
                log.Printf("Error sending pong to client %s: %v", c.userID, err)
                return
            }
            
            // Đặt lại thời gian chờ đọc
            c.conn.SetReadDeadline(time.Now().Add(pongWait))
            continue
        }
        
        message = bytes.TrimSpace(bytes.Replace(message, newLine, space, -1))
        c.hub.broadcast <- message
    }
}

func (c *Client) writePump() {
    ticker := time.NewTicker(pingPeriod)

    defer func() {
        ticker.Stop()
        c.conn.Close()    
        log.Printf("writePump: Client %s disconnected from group %s", c.userID, c.groupID)
    }()

    for {
        select {
        case message, ok := <-c.send:
            c.conn.SetWriteDeadline(time.Now().Add(writeWait))
            if !ok {
                // Hub đã đóng kênh
                log.Printf("Hub closed channel for client %s", c.userID)
                c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
                return
            }

            w, err := c.conn.NextWriter(websocket.TextMessage)
            if err != nil {
                log.Printf("Error getting next writer for client %s: %v", c.userID, err)
                return
            }

            w.Write(message)
            
            // Gửi những tin nhắn còn lại trong hàng đợi
            n := len(c.send)
            for i := 0; i < n; i++ {
                w.Write(newLine)
                w.Write(<-c.send)    
            }

            if err := w.Close(); err != nil {
                log.Printf("Error closing writer for client %s: %v", c.userID, err)
                return
            }
            
        case <-ticker.C:
            c.conn.SetWriteDeadline(time.Now().Add(writeWait))
            if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
                log.Printf("Error sending ping to client %s: %v", c.userID, err)
                return
            }
            // log.Printf("Sent ping to client %s", c.userID)
        }
    }
}
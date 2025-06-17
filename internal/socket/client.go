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
	space   = []byte{' '}
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
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
	hub     *Hub
	conn    *websocket.Conn
	send    chan []byte
	userID  string
	groupID string
	token   string
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		_ = c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			log.Printf("Read error client %s: %v", c.userID, err)
			break
		}

		var msgType MessageType
		if err := json.Unmarshal(message, &msgType); err == nil && msgType.Type == "ping" {
			pongMessage, _ := json.Marshal(map[string]string{"type": "pong"})
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, pongMessage); err != nil {
				log.Printf("Pong send error: %v", err)
				return
			}
			continue
		}

		c.hub.broadcast <- bytes.TrimSpace(bytes.Replace(message, newLine, space, -1))
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)

			for i := 0; i < len(c.send); i++ {
				_, _ = w.Write(newLine)
				_, _ = w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

package socket

import (
	"chat-service/internal/models"
	"chat-service/internal/repository"
	"chat-service/internal/service"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Message struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	Token      string    `json:"token"`
	MessageID  string    `json:"message_id"`
	MessageIDs []string  `json:"message_ids"`
	IsEdit     bool      `json:"is_edit"`
	IsDelete   bool      `json:"is_delete"`
	GroupID    string    `json:"group_id"`
	ReaderID   string    `json:"reader_id"`
	SenderID   string    `json:"sender_id"`
	SenderInfo *UserInfo `json:"sender_infor,omitempty"`
	Content    string    `json:"content"`
	ReactType  string    `json:"react_type"`
	ContenType string    `json:"content_type"`
	ImageKey   string    `json:"image_key,omitempty"`
	Timestamp  string    `json:"created_at"`
}

type MessageChat struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	GroupID    string    `json:"group_id"`
	SenderID   string    `json:"sender_id"`
	SenderInfo *UserInfo `json:"sender_infor,omitempty"`
	Content    string    `json:"content"`
	ContenType string    `json:"content_type"`
	ImageKey   string    `json:"image_key,omitempty"`
	Timestamp  string    `json:"created_at"`
}

type EditMessage struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	IsEdit     bool      `json:"is_edit"`
	GroupID    string    `json:"group_id"`
	SenderID   string    `json:"sender_id"`
	SenderInfo *UserInfo `json:"sender_infor,omitempty"`
	Content    string    `json:"content"`
	Timestamp  string    `json:"created_at"`
}

type DeleteMessage struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	IsDelete   bool      `json:"is_delete"`
	GroupID    string    `json:"group_id"`
	SenderID   string    `json:"sender_id"`
	SenderInfo *UserInfo `json:"sender_infor,omitempty"`
	Content    string    `json:"content"`
	Timestamp  string    `json:"created_at"`
}

type ReactMessage struct {
	ID         string    `json:"id"`
	Type       string    `json:"type"`
	GroupID    string    `json:"group_id"`
	SenderID   string    `json:"sender_id"`
	SenderInfo *UserInfo `json:"sender_infor,omitempty"`
	ReactType  string    `json:"react_type"`
	Timestamp  string    `json:"created_at"`
}

// type ReadReceiptMessage struct {
// 	Type       string              `json:"type"`
// 	MessageID  string              `json:"message_id"`
// 	GroupID    string              `json:"group_id"`
// 	SenderID   string              `json:"sender_id"`
// 	SenderInfo *UserInfo           `json:"sender_infor,omitempty"`
// 	ReadAt     string              `json:"read_at"`
// 	ReadBy     []map[string]string `json:"read_by"`
// 	Timestamp  string              `json:"created_at"`
// }

type OnlineUsersUpdate struct {
	Type        string              `json:"type"`
	GroupID     string              `json:"group_id"`
	OnlineCount int                 `json:"online_count"`
	OnlineUsers []*models.UserInfor `json:"online_users"`
}

type UserInfo struct {
	UserID    string    `json:"user_id"`
	Username  string    `json:"user_name"`
	AvatarURL string    `json:"avatar_url"`
	LastFetch time.Time `json:"-"`
}

// type MessageRead struct {
// 	MessageID string            `json:"message_id"`
// 	GroupID   string            `json:"group_id"`
// 	ReadBy    map[string]string `json:"read_by"`
// }

type Hub struct {
	rooms            map[string]map[*Client]bool
	roomsMutex       sync.RWMutex
	onlineUsers      map[string]map[string]bool
	onlineUsersMutex sync.RWMutex
	broadcast        chan []byte
	register         chan *Client
	unregister       chan *Client
	messageService   service.ChatService
	userOnlineRepo   repository.UserOnlineRepository

	userCache      map[string]*UserInfo
	userCacheMutex sync.RWMutex
	userCacheTTL   time.Duration
	userService    service.UserService
}

func NewHub(messageService service.ChatService, userService service.UserService, userOnlineRepo repository.UserOnlineRepository) *Hub {
	return &Hub{
		broadcast:        make(chan []byte, 256),
		register:         make(chan *Client, 10),
		unregister:       make(chan *Client, 10),
		rooms:            make(map[string]map[*Client]bool),
		roomsMutex:       sync.RWMutex{},
		onlineUsers:      make(map[string]map[string]bool),
		onlineUsersMutex: sync.RWMutex{},
		messageService:   messageService,
		userOnlineRepo:   userOnlineRepo,

		userCache:      make(map[string]*UserInfo),
		userCacheMutex: sync.RWMutex{},
		userCacheTTL:   24 * time.Hour,
		userService:    userService,
	}
}

func (h *Hub) getUserInfo(userID string) (*UserInfo, error) {
	h.userCacheMutex.RLock()
	if userInfo, ok := h.userCache[userID]; ok {
		// Kiểm tra cache còn hiệu lực không
		if time.Since(userInfo.LastFetch) < h.userCacheTTL {
			h.userCacheMutex.RUnlock()
			return userInfo, nil
		}
	}
	h.userCacheMutex.RUnlock()

	// Cache không có hoặc hết hạn, gọi API
	userInfo, err := h.userService.GetUserInfor(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %v", err)
	}

	// Cập nhật cache
	h.userCacheMutex.Lock()
	h.userCache[userID] = &UserInfo{
		UserID:    userID,
		Username:  userInfo.UserName,
		AvatarURL: userInfo.Avartar,
		LastFetch: time.Now(),
	}
	h.userCacheMutex.Unlock()

	return h.userCache[userID], nil
}

func (h *Hub) broadcastOnlineUsersUpdate(groupID string) {
	h.onlineUsersMutex.RLock()
	defer h.onlineUsersMutex.RUnlock()

	var onlineCount int
	var onlineUsersList []*models.UserInfor

	if userMap, ok := h.onlineUsers[groupID]; ok {
		onlineCount = len(userMap)
		for userID := range userMap {
			userInfo, err := h.getUserInfo(userID)
			if err != nil {
				log.Printf("Failed to get user info for %s: %v\n", userID, err)
				continue
			}
			user := &models.UserInfor{
				UserID:   userInfo.UserID,
				UserName: userInfo.Username,
				Avartar:  userInfo.AvatarURL,
			}
			onlineUsersList = append(onlineUsersList, user)
		}
	}

	update := OnlineUsersUpdate{
		Type:        "online_update",
		GroupID:     groupID,
		OnlineCount: onlineCount,
		OnlineUsers: onlineUsersList,
	}

	updateBytes, err := json.Marshal(update)
	if err != nil {
		log.Printf("Error marshaling online users update: %v", err)
		return
	}

	h.roomsMutex.RLock()
	if clients, ok := h.rooms[groupID]; ok {
		for client := range clients {
			select {
			case client.send <- updateBytes:
			default:

			}
		}
	}
	h.roomsMutex.RUnlock()
}

// func (h *Hub) handleJoinGroup(client *Client) {

// 	messages, err := h.messageService.GetUnreadMessages(context.Background(), client.userID, client.groupID)
// 	if err != nil {
// 		log.Printf("Failed to get unread messages: %v", err)
// 		return
// 	}

// 	if len(messages) > 0 {

// 		unreadNotification := struct {
// 			Type             string               `json:"type"`
// 			Unread           int                  `json:"unread"`
// 			UnreadMessageIds []primitive.ObjectID `json:"unread_message_ids"`
// 			GroupID          string               `json:"group_id"`
// 		}{
// 			Type:             "unread_notification",
// 			Unread:           len(messages),
// 			UnreadMessageIds: messages,
// 			GroupID:          client.groupID,
// 		}

// 		notificationBytes, err := json.Marshal(unreadNotification)
// 		if err != nil {
// 			log.Printf("Failed to marshal unread notification: %v", err)
// 			return
// 		}

// 		client.send <- notificationBytes
// 	}
// }
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.roomsMutex.Lock()
			if _, ok := h.rooms[client.groupID]; !ok {
				h.rooms[client.groupID] = make(map[*Client]bool)
			}
			h.rooms[client.groupID][client] = true
			h.roomsMutex.Unlock()

			h.onlineUsersMutex.Lock()
			if _, ok := h.onlineUsers[client.groupID]; !ok {
				h.onlineUsers[client.groupID] = make(map[string]bool)
			}
			h.onlineUsers[client.groupID][client.userID] = true
			h.onlineUsersMutex.Unlock()

			log.Printf("Client %s connected to group %s", client.userID, client.groupID)

			// Prefetch user info để tránh trễ lần sau
			go func(uid string) {
				_, _ = h.getUserInfo(uid)
			}(client.userID)
			h.broadcastOnlineUsersUpdate(client.groupID)
		case client := <-h.unregister:
			h.roomsMutex.Lock()
			if clients, ok := h.rooms[client.groupID]; ok {
				if _, found := clients[client]; found {
					err := h.userOnlineRepo.SaveUserOnline(context.Background(), &models.UserOnline{
						UserID:     client.userID,
						LastOnline: time.Now(),
					})
					if err != nil {
						log.Printf("Error saving user online: %v", err)
					}
					delete(clients, client)
					close(client.send)
					log.Printf("Client %s removed from group %s", client.userID, client.groupID)
					if len(clients) == 0 {
						delete(h.rooms, client.groupID)
						log.Printf("Group %s removed as it's empty", client.groupID)
					}
				}
			}
			h.roomsMutex.Unlock()

			h.onlineUsersMutex.Lock()
			if userMap, ok := h.onlineUsers[client.groupID]; ok {
				delete(userMap, client.userID)
				if len(userMap) == 0 {
					delete(h.onlineUsers, client.groupID)
				}
			}
			h.onlineUsersMutex.Unlock()

			h.broadcastOnlineUsersUpdate(client.groupID)

		case message := <-h.broadcast:
			h.handleBroadcastMessage(message)
		}
	}
}

func (h *Hub) handleBroadcastMessage(message []byte) {
	var msg Message
	if err := json.Unmarshal(message, &msg); err != nil {
		log.Printf("Error parsing message: %v", err)
		return
	}

	userInfo, err := h.getUserInfo(msg.SenderID)
	if err == nil {
		msg.SenderInfo = userInfo
	} else {
		log.Printf("Failed to get user info for %s: %v\n", msg.SenderID, err)
	}

	switch msg.Type {
	case "message":
		h.saveAndBroadcastMessage(msg)
	case "edit-message":
		h.editAndBroadcastMessage(msg)
	case "delete-message":
		h.deleteAndBroadcastMessage(msg)
	case "react-message":
		h.reactAndBroadcastMessage(msg)
	// case "read-receipt":
	// 	h.handleReadReceipt(msg)
	// case "batch-read-receipt":
	// 	h.handleBatchReadReceipt(msg)
	}
}

func (h *Hub) saveAndBroadcastMessage(msg Message) {
	groupID, err := primitive.ObjectIDFromHex(msg.GroupID)
	if err != nil {
		log.Printf("Invalid group ID: %v", err)
		return
	}

	dbMsg := models.Message{
		GroupID:    groupID,
		SenderID:   msg.SenderID,
		Content:    msg.Content,
		ContenType: msg.ContenType,
		ImageKey:   msg.ImageKey,
		IsEdit:     false,
		IsDelete:   false,
		CreatedAt:  time.Now(),
	}

	messageChat := MessageChat{
		Type:       msg.Type,
		GroupID:    msg.GroupID,
		SenderID:   msg.SenderID,
		SenderInfo: msg.SenderInfo,
		Content:    msg.Content,
		ContenType: msg.ContenType,
		ImageKey:   msg.ImageKey,
		Timestamp:  msg.Timestamp,
	}

	go func() {
		id, err := h.messageService.SaveMessage(context.Background(), &dbMsg)
		if err != nil {
			log.Printf("Error saving message: %v", err)
			return
		}
		messageChat.ID = id.Hex()
		updatedMessage, _ := json.Marshal(messageChat)
		h.sendToGroup(msg.GroupID, updatedMessage)
	}()
}

func (h *Hub) editAndBroadcastMessage(msg Message) {

	dbMsg := models.EditMessage{
		ID:       msg.ID,
		Content:  msg.Content,
		UpdateAt: time.Now(),
	}

	editMessage := EditMessage{
		ID:         msg.ID,
		Type:       msg.Type,
		IsEdit:     true,
		GroupID:    msg.GroupID,
		SenderID:   msg.SenderID,
		SenderInfo: msg.SenderInfo,
		Content:    msg.Content,
		Timestamp:  msg.Timestamp,
	}

	go func() {
		if err := h.messageService.EditMessage(context.Background(), &dbMsg); err != nil {
			log.Printf("Error editing message: %v", err)
		}
		updatedMessage, _ := json.Marshal(editMessage)
		h.sendToGroup(msg.GroupID, updatedMessage)
	}()
}

func (h *Hub) deleteAndBroadcastMessage(msg Message) {
	id, err := primitive.ObjectIDFromHex(msg.ID)
	if err != nil {
		log.Printf("Error parsing message ID: %v", err)
		return
	}

	deleteMessage := DeleteMessage{
		ID:         msg.ID,
		Type:       msg.Type,
		IsDelete:   true,
		GroupID:    msg.GroupID,
		SenderID:   msg.SenderID,
		SenderInfo: msg.SenderInfo,
		Timestamp:  msg.Timestamp,
	}

	go func() {
		if err := h.messageService.DeleteMessage(context.Background(), id, msg.Token); err != nil {
			log.Printf("Error deleting message: %v", err)
		}
		updatedMessage, _ := json.Marshal(deleteMessage)
		h.sendToGroup(msg.GroupID, updatedMessage)
	}()
}

func (h *Hub) reactAndBroadcastMessage(msg Message) {

	messageReact := ReactMessage{
		ID:         msg.ID,
		Type:       msg.Type,
		GroupID:    msg.GroupID,
		SenderID:   msg.SenderID,
		SenderInfo: msg.SenderInfo,
		ReactType:  msg.ReactType,
		Timestamp:  msg.Timestamp,
	}
	
	go func() {
		if err := h.messageService.InsertMessageReact(context.Background(), msg.ID, msg.GroupID, msg.SenderID, msg.ReactType); err != nil {
			log.Printf("Error react message: %v", err)
		}
		updatedMessage, _ := json.Marshal(messageReact)
		h.sendToGroup(msg.GroupID, updatedMessage)
	}()

}

func (h *Hub) sendToGroup(groupID string, message []byte) {
	h.roomsMutex.RLock()
	defer h.roomsMutex.RUnlock()
	if clients, ok := h.rooms[groupID]; ok {
		for client := range clients {
			select {
			case client.send <- message:
			default:
				go func(c *Client) { h.unregister <- c }(client)
			}
		}
	}
}



// func (h *Hub) handleReadReceipt(msg Message) {
// 	// Kiểm tra kiểu tin nhắn
// 	if msg.Type != "read-receipt" {
// 		return
// 	}

// 	go func() {
// 		// Đánh dấu tin nhắn đã được đọc
// 		if err := h.messageService.MarkAsRead(context.Background(), msg.MessageID, msg.ReaderID, msg.SenderID, msg.GroupID); err != nil {
// 			log.Printf("Error marking message as read: %v", err)
// 			return
// 		}

// 		// Lấy thông tin của người đọc
// 		userInfor, err := h.userService.GetUserInfor(msg.SenderID)
// 		if err != nil {
// 			log.Printf("Error getting user info: %v", err)
// 			return
// 		}

// 		userInfo := &UserInfo{
// 			UserID:    userInfor.UserID,
// 			Username:  userInfor.UserName,
// 			AvatarURL: userInfor.Avartar,
// 		}

// 		// Lấy trạng thái đọc của tin nhắn một lần duy nhất
// 		readStatus, err := h.messageService.GetMessageReadStatus(context.Background(), []string{msg.MessageID}, msg.GroupID)
// 		if err != nil {
// 			log.Printf("Error getting message read status: %v", err)
// 			return
// 		}

// 		var readBy []map[string]string

// 		// Xử lý tất cả các thông tin "đã đọc" một lần
// 		if receipts, ok := readStatus[msg.MessageID]; ok {
// 			for _, receipt := range receipts {
// 				userReadInfor, err := h.getUserInfo(receipt.UserID)
// 				if err != nil {
// 					log.Printf("Error getting user info for ID %s: %v", receipt.UserID, err)
// 					continue // Bỏ qua người dùng này nhưng vẫn tiếp tục với người khác
// 				}

// 				readBy = append(readBy, map[string]string{
// 					"user_id":    receipt.UserID,
// 					"read_at":    receipt.ReadAt.Format(time.RFC3339),
// 					"username":   userReadInfor.Username,
// 					"avatar_url": userReadInfor.AvatarURL,
// 				})
// 			}
// 		}

// 		// Tạo một thông báo duy nhất chứa tất cả thông tin
// 		readReceipt := ReadReceiptMessage{
// 			Type:       msg.Type,
// 			MessageID:  msg.MessageID,
// 			GroupID:    msg.GroupID,
// 			SenderID:   msg.SenderID,
// 			SenderInfo: userInfo,
// 			ReadAt:     time.Now().Format(time.RFC3339),
// 			ReadBy:     readBy, // Bao gồm tất cả người đã đọc
// 			Timestamp:  time.Now().Format(time.RFC3339),
// 		}

// 		readReceiptBytes, err := json.Marshal(readReceipt)
// 		if err != nil {
// 			log.Printf("Error marshaling read receipt: %v", err)
// 			return
// 		}

// 		// Gửi một thông báo duy nhất cho cả nhóm
// 		h.sendToGroup(msg.GroupID, readReceiptBytes)
// 	}()
// }

// func (h *Hub) handleBatchReadReceipt(msg Message) {

// 	for _, message := range msg.MessageIDs {

// 		readReceipt := Message{
// 			Type:      "read-receipt",
// 			MessageID: message,
// 			GroupID:   msg.GroupID,
// 			ReaderID:  msg.SenderID,
// 			SenderID:  "",
// 		}

// 		h.handleReadReceipt(readReceipt)
// 	}

// }

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
	IsEdit     bool      `json:"is_edit"`
	IsDelete   bool      `json:"is_delete"`
	GroupID    string    `json:"group_id"`
	SenderID   string    `json:"sender_id"`
	SenderInfo *UserInfo `json:"sender_infor,omitempty"`
	Content    string    `json:"content"`
	ContenType string    `json:"content_type"`
	ImageKey   string    `json:"image_key,omitempty"`
	Timestamp  string    `json:"created_at"`
}

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
	}
	updatedMessage, err := json.Marshal(msg)
	if err == nil {
		message = updatedMessage
	}

	switch msg.Type {
	case "message":
		h.saveAndBroadcastMessage(msg, message)
	case "edit-message":
		h.editAndBroadcastMessage(msg, message)
	case "delete-message":
		h.deleteAndBroadcastMessage(msg, message)
	}
}

func (h *Hub) saveAndBroadcastMessage(msg Message, message []byte) {
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

	go func() {
		id, err := h.messageService.SaveMessage(context.Background(), &dbMsg)
		if err != nil {
			log.Printf("Error saving message: %v", err)
			return
		}
		msg.ID = id.Hex()
		updatedMessage, _ := json.Marshal(msg)
		h.sendToGroup(msg.GroupID, updatedMessage)
	}()
}

func (h *Hub) editAndBroadcastMessage(msg Message, message []byte) {
	dbMsg := models.EditMessage{
		ID:       msg.ID,
		Content:  msg.Content,
		UpdateAt: time.Now(),
	}

	go func() {
		if err := h.messageService.EditMessage(context.Background(), &dbMsg); err != nil {
			log.Printf("Error editing message: %v", err)
		}
		h.sendToGroup(msg.GroupID, message)
	}()
}

func (h *Hub) deleteAndBroadcastMessage(msg Message, message []byte) {
	id, err := primitive.ObjectIDFromHex(msg.ID)
	if err != nil {
		log.Printf("Error parsing message ID: %v", err)
		return
	}
	go func() {
		if err := h.messageService.DeleteMessage(context.Background(), id, msg.Token); err != nil {
			log.Printf("Error deleting message: %v", err)
		}
		h.sendToGroup(msg.GroupID, message)
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

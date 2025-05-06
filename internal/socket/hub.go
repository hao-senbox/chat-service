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
	GroupID    string    `json:"group_id"`
	SenderID   string    `json:"sender_id"`
	SenderInfo *UserInfo `json:"sender_infor,omitempty"`
	Content    string    `json:"content"`
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
	messageRepo      repository.ChatRepository

	userCache      map[string]*UserInfo
	userCacheMutex sync.RWMutex
	userCacheTTL   time.Duration
	userService    service.UserService
}

func NewHub(messageRepo repository.ChatRepository, userService service.UserService) *Hub {
	return &Hub{
		broadcast:        make(chan []byte, 256),
		register:         make(chan *Client, 10),
		unregister:       make(chan *Client, 10),
		rooms:            make(map[string]map[*Client]bool),
		roomsMutex:       sync.RWMutex{},
		onlineUsers:      make(map[string]map[string]bool),
		onlineUsersMutex: sync.RWMutex{},
		messageRepo:      messageRepo,

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
			// Đảm bảo room tồn tại
			if _, ok := h.rooms[client.groupID]; !ok {
				h.rooms[client.groupID] = make(map[*Client]bool)
			}
			// Thêm client vào room
			h.rooms[client.groupID][client] = true
			h.roomsMutex.Unlock()

			// Thêm vào map theo dõi người dùng online
			h.onlineUsersMutex.Lock()
			if _, ok := h.onlineUsers[client.groupID]; !ok {
				h.onlineUsers[client.groupID] = make(map[string]bool)
			}
			h.onlineUsers[client.groupID][client.userID] = true
			h.onlineUsersMutex.Unlock()

			log.Printf("Client %s connected to group %s", client.userID, client.groupID)

			go func() {
				_, err := h.getUserInfo(client.userID)
				if err != nil {
					log.Printf("Error fetching user info: %v", err)
				}
			}()

			// Gửi thông báo cập nhật số người online sau khi đăng ký thành công
			h.broadcastOnlineUsersUpdate(client.groupID)

		case client := <-h.unregister:
			h.roomsMutex.Lock()
			// Xóa client khỏi room
			if _, ok := h.rooms[client.groupID]; ok {
				if _, found := h.rooms[client.groupID][client]; found {
					delete(h.rooms[client.groupID], client)
					close(client.send)
					log.Printf("Client %s removed from group %s", client.userID, client.groupID)

					// Nếu room rỗng, xóa room
					if len(h.rooms[client.groupID]) == 0 {
						delete(h.rooms, client.groupID)
						log.Printf("Group %s removed as it's empty", client.groupID)
					}
				}
			}
			h.roomsMutex.Unlock()

			// Cập nhật map theo dõi người dùng online
			userOffline := false
			h.onlineUsersMutex.Lock()
			if userMap, ok := h.onlineUsers[client.groupID]; ok {
				// Kiểm tra xem người dùng còn kết nối nào khác trong group không
				delete(userMap, client.userID)
				userOffline = true

				// Nếu không còn người dùng nào, xóa map của group
				if len(userMap) == 0 {
					delete(h.onlineUsers, client.groupID)
				}
			}
			h.onlineUsersMutex.Unlock()

			// Nếu có người dùng offline, gửi thông báo cập nhật
			if userOffline {
				h.broadcastOnlineUsersUpdate(client.groupID)
			}

		case message := <-h.broadcast:
			// Parse message để lấy GroupID
			var msg Message
			if err := json.Unmarshal(message, &msg); err != nil {
				log.Printf("Error parsing message: %v", err)
				continue
			}

			userInfo, err := h.getUserInfo(msg.SenderID)
			if err != nil {
				log.Printf("Error getting sender info: %v", err)
				// Tiếp tục xử lý ngay cả khi không có thông tin người gửi
			} else {
				// Thêm thông tin người dùng vào message
				msg.SenderInfo = userInfo
				// Tạo message mới với thông tin người gửi
				updatedMessage, err := json.Marshal(msg)
				if err == nil {
					message = updatedMessage
				}
			}

			// Lưu message vào database
			groupID, err := primitive.ObjectIDFromHex(msg.GroupID)
			if err != nil {
				log.Printf("Invalid group ID: %v", err)
				continue
			}

			dbMsg := models.Message{
				GroupID:   groupID,
				SenderID:  msg.SenderID,
				Content:   msg.Content,
				CreatedAt: time.Now(),
			}

			msgToSave := dbMsg

			go func() {
				err := h.messageRepo.SaveMessage(context.Background(), &msgToSave)
				if err != nil {
					log.Printf("Error saving message: %v", err)
				}
			}()

			// Broadcast đến mọi người trong room
			h.roomsMutex.RLock()
			if clients, ok := h.rooms[msg.GroupID]; ok {
				for client := range clients {
					select {
					case client.send <- message:
					default:
						h.roomsMutex.RUnlock()
						h.unregister <- client
						h.roomsMutex.RLock()
					}
				}
			}
			h.roomsMutex.RUnlock()
		}
	}
}

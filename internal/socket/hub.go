package socket

import (
	"chat-service/internal/models"
	"chat-service/internal/repository"
	"chat-service/internal/service"
	"context"
	"encoding/json"
	"fmt"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"runtime"
	"sync"
	"time"
)

type Message struct {
	ID         string      `json:"id"`
	Type       string      `json:"type"`
	Token      string      `json:"token"`
	MessageID  string      `json:"message_id"`
	IsEdit     bool        `json:"is_edit"`
	IsDelete   bool        `json:"is_delete"`
	GroupID    string      `json:"group_id"`
	ReaderID   string      `json:"reader_id"`
	SenderID   string      `json:"sender_id"`
	SenderInfo *UserInfo   `json:"sender_infor,omitempty"`
	Content    string      `json:"content"`
	ReactType  string      `json:"react_type"`
	ContenType string      `json:"content_type"`
	ImageKey   string      `json:"image_key,omitempty"`
	VoteData   *VoteData   `json:"vote_data,omitempty"`
	VoteAction *VoteAction `json:"vote_action,omitempty"`
	Timestamp  string      `json:"created_at"`
}

type VoteData struct {
	Question string   `json:"question"`
	Options  []string `json:"options"`
	VoteType string   `json:"vote_type"`
	EndTime  string   `json:"end_time"`
	IsActive bool     `json:"is_active"`
}

type VoteAction struct {
	VoteID      string `json:"vote_id"`
	OptionIndex int    `json:"option_index"`
	Action      string `json:"action"`
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

// Worker pool for handling goroutines
type WorkerPool struct {
	workers  int
	taskChan chan func()
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

func NewWorkerPool(workers int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	wp := &WorkerPool{
		workers:  workers,
		taskChan: make(chan func(), 1000), // Buffered channel
		ctx:      ctx,
		cancel:   cancel,
	}

	// Start workers
	for i := 0; i < workers; i++ {
		wp.wg.Add(1)
		go wp.worker()
	}

	return wp
}

func (wp *WorkerPool) worker() {
	defer wp.wg.Done()
	for {
		select {
		case task := <-wp.taskChan:
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("Worker panic recovered: %v", r)
					}
				}()
				task()
			}()
		case <-wp.ctx.Done():
			return
		}
	}
}

func (wp *WorkerPool) Submit(task func()) {
	select {
	case wp.taskChan <- task:
	case <-wp.ctx.Done():
		return
	default:
		// Channel is full, handle gracefully
		log.Printf("Worker pool is busy, task dropped")
	}
}

func (wp *WorkerPool) Stop() {
	wp.cancel()
	wp.wg.Wait()
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
	groupService     service.GroupService
	userService      service.UserService
	voteService      service.VoteService

	// User cache with LRU-like behavior
	userCache      map[string]*UserInfo
	userCacheList  []string // For LRU tracking
	userCacheMutex sync.RWMutex
	userCacheTTL   time.Duration
	maxCacheSize   int

	// Worker pool for async operations
	workerPool *WorkerPool

	// Rate limiting
	rateLimiter map[string]*time.Ticker
	rateMutex   sync.RWMutex

	// Metrics
	metrics struct {
		activeConnections int64
		messagesProcessed int64
		errors            int64
		mutex             sync.RWMutex
	}

	// Graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc
}

func NewHub(messageService service.ChatService, 
	userService service.UserService,
	userOnlineRepo repository.UserOnlineRepository,
	groupService service.GroupService,
	voteService service.VoteService) *Hub {

	ctx, cancel := context.WithCancel(context.Background())

	// Calculate worker pool size based on CPU cores
	workerCount := runtime.NumCPU() * 2
	if workerCount < 4 {
		workerCount = 4
	}

	return &Hub{
		broadcast:      make(chan []byte, 1000), // Increased buffer
		register:       make(chan *Client, 100), // Increased buffer
		unregister:     make(chan *Client, 100), // Increased buffer
		rooms:          make(map[string]map[*Client]bool),
		onlineUsers:    make(map[string]map[string]bool),
		messageService: messageService,
		userOnlineRepo: userOnlineRepo,
		groupService:   groupService,
		userService:    userService,
		voteService:    voteService,

		userCache:     make(map[string]*UserInfo),
		userCacheList: make([]string, 0),
		userCacheTTL:  24 * time.Hour,
		maxCacheSize:  10000, // Limit cache size

		workerPool:  NewWorkerPool(workerCount),
		rateLimiter: make(map[string]*time.Ticker),

		ctx:    ctx,
		cancel: cancel,
	}
}

// Improved cache with LRU eviction
func (h *Hub) createContextWithToken(client *Client) context.Context {
	ctx := context.Background()
	return context.WithValue(ctx, "token", client.token)
}

// Updated getUserInfo method để nhận context
func (h *Hub) getUserInfoWithContext(ctx context.Context, userID string) (*UserInfo, error) {
	h.userCacheMutex.RLock()
	if userInfo, ok := h.userCache[userID]; ok {
		if time.Since(userInfo.LastFetch) < h.userCacheTTL {
			h.userCacheMutex.RUnlock()
			return userInfo, nil
		}
	}
	h.userCacheMutex.RUnlock()

	// Fetch from service với context chứa token
	userInfo, err := h.userService.GetUserInfor(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %v", err)
	}

	// Update cache with LRU eviction
	h.userCacheMutex.Lock()
	defer h.userCacheMutex.Unlock()

	// Remove from old position if exists
	for i, id := range h.userCacheList {
		if id == userID {
			h.userCacheList = append(h.userCacheList[:i], h.userCacheList[i+1:]...)
			break
		}
	}

	// Add to front
	h.userCacheList = append([]string{userID}, h.userCacheList...)

	// Evict if cache is too large
	if len(h.userCacheList) > h.maxCacheSize {
		// Remove oldest entries
		for i := h.maxCacheSize; i < len(h.userCacheList); i++ {
			delete(h.userCache, h.userCacheList[i])
		}
		h.userCacheList = h.userCacheList[:h.maxCacheSize]
	}

	h.userCache[userID] = &UserInfo{
		UserID:    userID,
		Username:  userInfo.UserName,
		AvatarURL: userInfo.Avartar,
		LastFetch: time.Now(),
	}

	return h.userCache[userID], nil
}

// Debounced online users update
func (h *Hub) broadcastOnlineUsersUpdate(groupID string, clientForToken *Client) {
	h.workerPool.Submit(func() {
		// Add small delay to batch updates
		time.Sleep(100 * time.Millisecond)

		h.onlineUsersMutex.RLock()
		defer h.onlineUsersMutex.RUnlock()

		var onlineCount int
		var onlineUsersList []*models.UserInfor

		if userMap, ok := h.onlineUsers[groupID]; ok {
			onlineCount = len(userMap)
			ctx := h.createContextWithToken(clientForToken)
			// Use worker pool to fetch user info concurrently
			userInfoChan := make(chan *models.UserInfor, onlineCount)
			var wg sync.WaitGroup

			for userID := range userMap {
				wg.Add(1)
				go func(uid string) {
					defer wg.Done()
					userInfo, err := h.getUserInfoWithContext(ctx, uid)
					if err != nil {
						log.Printf("Failed to get user info for %s: %v\n", uid, err)
						return
					}
					userInfoChan <- &models.UserInfor{
						UserID:   userInfo.UserID,
						UserName: userInfo.Username,
						Avartar:  userInfo.AvatarURL,
					}
				}(userID)
			}

			// Close channel when all goroutines finish
			go func() {
				wg.Wait()
				close(userInfoChan)
			}()

			// Collect results
			for user := range userInfoChan {
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

		h.sendToGroup(groupID, updateBytes)
	})
}

func (h *Hub) Run() {
	defer h.workerPool.Stop()

	// Cleanup ticker for rate limiters
	cleanupTicker := time.NewTicker(5 * time.Minute)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-h.ctx.Done():
			log.Println("Hub shutting down...")
			return

		case <-cleanupTicker.C:
			h.cleanupRateLimiters()

		case client := <-h.register:
			h.handleClientRegister(client)

		case client := <-h.unregister:
			h.handleClientUnregister(client)

		case message := <-h.broadcast:
			h.handleBroadcastMessage(message)
		}
	}
}

func (h *Hub) handleClientRegister(client *Client) {
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

	// Update metrics
	h.metrics.mutex.Lock()
	h.metrics.activeConnections++
	h.metrics.mutex.Unlock()

	log.Printf("Client %s connected to group %s", client.userID, client.groupID)

	// Preload user info asynchronously
	h.workerPool.Submit(func() {
		ctx := h.createContextWithToken(client)
		_, _ = h.getUserInfoWithContext(ctx, client.userID)
	})

	h.broadcastOnlineUsersUpdate(client.groupID, client)
}

func (h *Hub) handleClientUnregister(client *Client) {
	h.roomsMutex.Lock()
	if clients, ok := h.rooms[client.groupID]; ok {
		if _, found := clients[client]; found {
			// Save user online status asynchronously
			h.workerPool.Submit(func() {
				ctx := h.createContextWithToken(client)
				err := h.userOnlineRepo.SaveUserOnline(ctx, &models.UserOnline{
					UserID:     client.userID,
					LastOnline: time.Now(),
				})
				if err != nil {
					log.Printf("Error saving user online: %v", err)
				}
			})

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

	// Update metrics
	h.metrics.mutex.Lock()
	h.metrics.activeConnections--
	h.metrics.mutex.Unlock()

	var tokenClient *Client

	h.broadcastOnlineUsersUpdate(client.groupID, tokenClient)
}

func (h *Hub) handleBroadcastMessage(message []byte) {
	var msg Message
	if err := json.Unmarshal(message, &msg); err != nil {
		log.Printf("Error parsing message: %v", err)
		h.incrementErrorCount()
		return
	}

	// Rate limiting check
	if !h.checkRateLimit(msg.SenderID) {
		log.Printf("Rate limit exceeded for user %s", msg.SenderID)
		return
	}

	var senderClient *Client
	h.roomsMutex.RLock()
	if clients, ok := h.rooms[msg.GroupID]; ok {
		for client := range clients {
			if client.userID == msg.SenderID {
				senderClient = client
				break
			}
		}
	}
	h.roomsMutex.RUnlock()

	if senderClient == nil {
		log.Printf("Could not find sender client for user %s", msg.SenderID)
		return
	}

	// Get user info asynchronously
	h.workerPool.Submit(func() {
		ctx := h.createContextWithToken(senderClient)
		userInfo, err := h.getUserInfoWithContext(ctx, msg.SenderID)
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
		case "create-vote":
			h.createAndBroadcastVote(msg)
		default:
			log.Printf("Unknown message type: %s", msg.Type)
		}

		h.incrementProcessedCount()
	})
}

// Rate limiting
func (h *Hub) checkRateLimit(userID string) bool {
	h.rateMutex.Lock()
	defer h.rateMutex.Unlock()

	if _, exists := h.rateLimiter[userID]; !exists {
		// Allow 10 messages per second per user
		h.rateLimiter[userID] = time.NewTicker(100 * time.Millisecond)
		return true
	}

	select {
	case <-h.rateLimiter[userID].C:
		return true
	default:
		return false
	}
}

func (h *Hub) cleanupRateLimiters() {
	h.rateMutex.Lock()
	defer h.rateMutex.Unlock()

	// Clean up inactive rate limiters
	for userID, ticker := range h.rateLimiter {
		// Check if user is still online in any group
		userOnline := false
		h.onlineUsersMutex.RLock()
		for _, userMap := range h.onlineUsers {
			if userMap[userID] {
				userOnline = true
				break
			}
		}
		h.onlineUsersMutex.RUnlock()

		if !userOnline {
			ticker.Stop()
			delete(h.rateLimiter, userID)
		}
	}
}

// Message handlers with better error handling and context
func (h *Hub) saveAndBroadcastMessage(msg Message) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	groupID, err := primitive.ObjectIDFromHex(msg.GroupID)
	if err != nil {
		log.Printf("Invalid group ID: %v", err)
		h.incrementErrorCount()
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

	res := map[string]interface{}{
		"type":         msg.Type,
		"group_id":     msg.GroupID,
		"sender_id":    msg.SenderID,
		"sender_infor": msg.SenderInfo,
		"content":      msg.Content,
		"content_type": msg.ContenType,
		"image_key":    msg.ImageKey,
		"created_at":   msg.Timestamp,
	}

	id, err := h.messageService.SaveMessage(ctx, &dbMsg)
	if err != nil {
		log.Printf("Error saving message: %v", err)
		h.incrementErrorCount()
		return
	}

	res["id"] = id.Hex()
	updatedMessage, _ := json.Marshal(res)
	h.sendToGroup(msg.GroupID, updatedMessage)
}

// Similar improvements for other message handlers...
func (h *Hub) editAndBroadcastMessage(msg Message) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	dbMsg := models.EditMessage{
		ID:       msg.ID,
		Content:  msg.Content,
		UpdateAt: time.Now(),
	}

	res := map[string]interface{}{
		"id":           msg.ID,
		"type":         msg.Type,
		"group_id":     msg.GroupID,
		"is_edit":      true,
		"sender_id":    msg.SenderID,
		"sender_infor": msg.SenderInfo,
		"content":      msg.Content,
		"created_at":   msg.Timestamp,
	}

	if err := h.messageService.EditMessage(ctx, &dbMsg); err != nil {
		log.Printf("Error editing message: %v", err)
		h.incrementErrorCount()
		return
	}

	updatedMessage, _ := json.Marshal(res)
	h.sendToGroup(msg.GroupID, updatedMessage)
}

func (h *Hub) deleteAndBroadcastMessage(msg Message) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	id, err := primitive.ObjectIDFromHex(msg.ID)
	if err != nil {
		log.Printf("Error parsing message ID: %v", err)
		h.incrementErrorCount()
		return
	}

	res := map[string]interface{}{
		"id":           msg.ID,
		"type":         msg.Type,
		"is_delete":    true,
		"group_id":     msg.GroupID,
		"sender_id":    msg.SenderID,
		"sender_infor": msg.SenderInfo,
		"created_at":   msg.Timestamp,
	}

	if err := h.messageService.DeleteMessage(ctx, id, msg.Token); err != nil {
		log.Printf("Error deleting message: %v", err)
		h.incrementErrorCount()
		return
	}

	if err := h.messageService.DeleteMessageReacts(ctx, msg.ID, msg.GroupID); err != nil {
		log.Printf("Error deleting message reacts: %v", err)
		h.incrementErrorCount()
	}

	updatedMessage, _ := json.Marshal(res)
	h.sendToGroup(msg.GroupID, updatedMessage)
}

func (h *Hub) reactAndBroadcastMessage(msg Message) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err := h.messageService.InsertMessageReact(ctx, msg.ID, msg.GroupID, msg.SenderID, msg.ReactType)
	if err != nil {
		log.Printf("Error inserting message react: %v", err)
		h.incrementErrorCount()
		return
	}

	groupDetail, err := h.groupService.GetGroupDetail(ctx, msg.GroupID)
	if err != nil {
		log.Printf("Error getting group detail: %v", err)
		h.incrementErrorCount()
		return
	}

	reacts, err := h.messageService.GetMessageReacts(ctx, msg.ID, msg.GroupID)
	if err != nil {
		log.Printf("Error getting message reacts: %v", err)
		h.incrementErrorCount()
		return
	}

	var totalAllReacts int64 = 0
	reactedsUserIDs := make(map[string]bool)
	ctxToken := h.createContextWithToken(senderClient)
	// Process reacts concurrently
	var wg sync.WaitGroup
	for i := range reacts {
		totalAllReacts += reacts[i].TotalReact
		for j := range reacts[i].UserReact {
			reactedsUserIDs[reacts[i].UserReact[j].UserID] = true
			if reacts[i].UserReact[j].UserInfor == nil {
				wg.Add(1)
				go func(reactIdx, userIdx int, userID string) {
					defer wg.Done()
					user, err := h.userService.GetUserInfor(context.Background(), userID)
					if err != nil {
						log.Printf("Error getting user info: %v", err)
						return
					}
					reacts[reactIdx].UserReact[userIdx].UserInfor = &models.UserInfor{
						UserID:   user.UserID,
						UserName: user.UserName,
						Avartar:  user.Avartar,
					}ctx := h.createContextWithToken(senderClient)
				}(i, j, reacts[i].UserReact[j].UserID)
			}
		}
	}
	wg.Wait()

	var notReactedMembers []map[string]interface{}
	var ReactedMembers []map[string]interface{}

	for _, member := range groupDetail.Members {
		memberInfo := map[string]interface{}{
			"user_id":    member.GroupMember.UserID,
			"user_name":  member.GroupMember.UserInfor.UserName,
			"avatar_url": member.GroupMember.UserInfor.Avartar,
		}

		if reactedsUserIDs[member.GroupMember.UserID] {
			var userReactions []string
			for _, react := range reacts {
				for _, ur := range react.UserReact {
					if ur.UserID == member.GroupMember.UserID {
						userReactions = append(userReactions, react.React)
					}
				}
			}
			memberInfo["reacts"] = userReactions
			ReactedMembers = append(ReactedMembers, memberInfo)
		} else {
			notReactedMembers = append(notReactedMembers, memberInfo)
		}
	}

	res := map[string]interface{}{
		"type":                "react-message",
		"message_id":          msg.ID,
		"group_id":            msg.GroupID,
		"sender_id":           msg.SenderID,
		"sender_infor":        msg.SenderInfo,
		"react_type":          msg.ReactType,
		"reacts":              reacts,
		"total_all_reacts":    totalAllReacts,
		"reacted_members":     ReactedMembers,
		"not_reacted_members": notReactedMembers,
	}

	updatedMessage, _ := json.Marshal(res)
	h.sendToGroup(msg.GroupID, updatedMessage)
}

func (h *Hub) createAndBroadcastVote(msg Message) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	objectGroupID, err := primitive.ObjectIDFromHex(msg.GroupID)
	if err != nil {
		log.Printf("Error converting group ID to ObjectID: %v", err)
		h.incrementErrorCount()
		return
	}

	convertEndTIme, err := time.Parse(time.RFC3339, msg.VoteData.EndTime)
	if err != nil {
		log.Printf("Error parsing end time: %v", err)
		h.incrementErrorCount()
		return
	}

	dbVote := models.Vote{
		GroupID:   objectGroupID,
		CreatedBy: msg.SenderID,
		Question:  msg.VoteData.Question,
		Options:   []models.VoteOption{},
		VoteType:  msg.VoteData.VoteType,
		EndTime:   convertEndTIme,
		IsActive:  msg.VoteData.IsActive,
	}

	for _, optionText := range msg.VoteData.Options {
		dbVote.Options = append(dbVote.Options, models.VoteOption{
			Text:      optionText,
			VoteBy:    []string{},
			VoteCount: 0,
		})
	}

	idVote, err := h.voteService.InsertVote(ctx, &dbVote)
	if err != nil {
		log.Printf("Error inserting vote: %v", err)
		h.incrementErrorCount()
		return
	}

	dbMessage := models.Message{
		VoteID:     &idVote,
		GroupID:    objectGroupID,
		SenderID:   msg.SenderID,
		ContenType: "vote",
	}

	_, err = h.messageService.SaveMessage(ctx, &dbMessage)
	if err != nil {
		log.Printf("Error inserting message: %v", err)
		h.incrementErrorCount()
		return
	}

	res := map[string]interface{}{
		"type":         msg.Type,
		"group_id":     msg.GroupID,
		"sender_id":    msg.SenderID,
		"sender_infor": msg.SenderInfo,
		"vote":         msg.VoteData,
	}

	updatedMessage, _ := json.Marshal(res)
	h.sendToGroup(msg.GroupID, updatedMessage)

}

func (h *Hub) sendToGroup(groupID string, message []byte) {
	h.roomsMutex.RLock()
	defer h.roomsMutex.RUnlock()

	if clients, ok := h.rooms[groupID]; ok {
		deadClients := make([]*Client, 0)

		for client := range clients {
			select {
			case client.send <- message:
				// Message sent successfully
			default:
				// Client's send channel is blocked or closed
				deadClients = append(deadClients, client)
			}
		}

		// Clean up dead clients
		for _, client := range deadClients {
			go func(c *Client) {
				select {
				case h.unregister <- c:
				default:
					// Unregister channel is full, will be handled later
				}
			}(client)
		}
	}
}

// Metrics methods
func (h *Hub) incrementProcessedCount() {
	h.metrics.mutex.Lock()
	h.metrics.messagesProcessed++
	h.metrics.mutex.Unlock()
}

func (h *Hub) incrementErrorCount() {
	h.metrics.mutex.Lock()
	h.metrics.errors++
	h.metrics.mutex.Unlock()
}

func (h *Hub) GetMetrics() (int64, int64, int64) {
	h.metrics.mutex.RLock()
	defer h.metrics.mutex.RUnlock()
	return h.metrics.activeConnections, h.metrics.messagesProcessed, h.metrics.errors
}

// Graceful shutdown
func (h *Hub) Shutdown() {
	h.cancel()

	// Clean up rate limiters
	h.rateMutex.Lock()
	for _, ticker := range h.rateLimiter {
		ticker.Stop()
	}
	h.rateMutex.Unlock()
}

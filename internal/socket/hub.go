package socket

import (
	"chat-service/internal/models"
	"chat-service/internal/repository"
	"chat-service/internal/service"
	"chat-service/pkg/constants"
	"context"
	"encoding/json"
	"fmt"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"log"
	"runtime"
	"sync"
	"time"
)

func NewWorkerPool(workers int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	wp := &WorkerPool{
		workers:  workers,
		taskChan: make(chan func(), 1000),
		ctx:      ctx,
		cancel:   cancel,
	}

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

	userCache      map[string]*UserInfo
	userCacheList  []string
	userCacheMutex sync.RWMutex
	userCacheTTL   time.Duration
	maxCacheSize   int

	workerPool *WorkerPool

	rateLimiter map[string]*time.Ticker
	rateMutex   sync.RWMutex

	metrics struct {
		activeConnections int64
		messagesProcessed int64
		errors            int64
		mutex             sync.RWMutex
	}

	ctx    context.Context
	cancel context.CancelFunc
}

func NewHub(messageService service.ChatService,
	userService service.UserService,
	userOnlineRepo repository.UserOnlineRepository,
	groupService service.GroupService,
	voteService service.VoteService) *Hub {

	ctx, cancel := context.WithCancel(context.Background())

	workerCount := runtime.NumCPU() * 2
	if workerCount < 4 {
		workerCount = 4
	}

	return &Hub{
		broadcast:      make(chan []byte, 1000),
		register:       make(chan *Client, 100),
		unregister:     make(chan *Client, 100),
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
		maxCacheSize:  10000,

		workerPool:  NewWorkerPool(workerCount),
		rateLimiter: make(map[string]*time.Ticker),

		ctx:    ctx,
		cancel: cancel,
	}
}

func (h *Hub) getUserInfoWithContext(ctx context.Context, userID string) (*UserInfo, error) {
	h.userCacheMutex.RLock()
	if userInfo, ok := h.userCache[userID]; ok {
		if time.Since(userInfo.LastFetch) < h.userCacheTTL {
			h.userCacheMutex.RUnlock()
			return userInfo, nil
		}
	}
	h.userCacheMutex.RUnlock()

	userInfo, err := h.userService.GetUserInfor(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch user info: %v", err)
	}

	h.updateUserCache(userID, userInfo)
	return h.userCache[userID], nil
}

func (h *Hub) updateUserCache(userID string, userInfo *models.UserInfor) {
	h.userCacheMutex.Lock()
	defer h.userCacheMutex.Unlock()

	for i, id := range h.userCacheList {
		if id == userID {
			h.userCacheList = append(h.userCacheList[:i], h.userCacheList[i+1:]...)
			break
		}
	}

	h.userCacheList = append([]string{userID}, h.userCacheList...)

	if len(h.userCacheList) > h.maxCacheSize {
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
}

func (h *Hub) findSenderClient(groupID, senderID string) *Client {
	h.roomsMutex.RLock()
	defer h.roomsMutex.RUnlock()

	if clients, ok := h.rooms[groupID]; ok {
		for client := range clients {
			if client.userID == senderID {
				return client
			}
		}
	}
	return nil
}

func (h *Hub) getContextWithToken(client *Client) context.Context {
	return context.WithValue(context.Background(), constants.TokenKey, client.token)
}

func (h *Hub) getNotReactedMembers(ctx context.Context, groupID, senderID string, reactedUserIDs map[string]bool) ([]*models.UserInfor, error) {
	groupDetail, err := h.groupService.GetGroupDetail(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("error getting group detail: %v", err)
	}

	var notReactedMembers []*models.UserInfor
	for _, member := range groupDetail.Members {
		userID := member.GroupMember.UserID
		if userID == senderID {
			continue
		}
		if reactedUserIDs == nil || !reactedUserIDs[userID] {
			notReactedMembers = append(notReactedMembers, &models.UserInfor{
				UserID:   member.GroupMember.UserID,
				UserName: member.GroupMember.UserInfor.UserName,
			})
		}
	}
	return notReactedMembers, nil
}

func (h *Hub) populateUserInfoInReacts(ctx context.Context, reacts []*models.MessageReact) (int64, map[string]bool) {
	var totalAllReacts int64
	reactedUserIDs := make(map[string]bool)
	var wg sync.WaitGroup

	for i := range reacts {
		totalAllReacts += reacts[i].TotalReact
		for j := range reacts[i].UserReact {
			reactedUserIDs[reacts[i].UserReact[j].UserID] = true
			if reacts[i].UserReact[j].UserInfor == nil {
				wg.Add(1)
				go func(reactIdx, userIdx int, userID string) {
					defer wg.Done()
					user, err := h.userService.GetUserInfor(ctx, userID)
					if err != nil {
						log.Printf("Error getting user info: %v", err)
						return
					}
					reacts[reactIdx].UserReact[userIdx].UserInfor = &models.UserInfor{
						UserID:   user.UserID,
						UserName: user.UserName,
						Avartar:  user.Avartar,
					}
				}(i, j, reacts[i].UserReact[j].UserID)
			}
		}
	}
	wg.Wait()
	return totalAllReacts, reactedUserIDs
}

func (h *Hub) broadcastOnlineUsersUpdate(groupID string, clientForToken *Client) {
	h.workerPool.Submit(func() {
		time.Sleep(100 * time.Millisecond)

		h.onlineUsersMutex.RLock()
		defer h.onlineUsersMutex.RUnlock()

		var onlineCount int
		var onlineUsersList []*models.UserInfor

		if userMap, ok := h.onlineUsers[groupID]; ok {
			onlineCount = len(userMap)
			ctx := h.getContextWithToken(clientForToken)
			userInfoChan := make(chan *models.UserInfor, onlineCount)
			var wg sync.WaitGroup

			for userID := range userMap {
				wg.Add(1)
				go func(uid string) {
					defer wg.Done()
					userInfo, err := h.getUserInfoWithContext(ctx, uid)
					if err != nil {
						log.Printf("(Hub) 1 - Failed to get user info for %s: %v\n", uid, err)
						return
					}
					userInfoChan <- &models.UserInfor{
						UserID:   userInfo.UserID,
						UserName: userInfo.Username,
						Avartar:  userInfo.AvatarURL,
					}
				}(userID)
			}

			go func() {
				wg.Wait()
				close(userInfoChan)
			}()

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

	h.incrementActiveConnections()
	log.Printf("Client %s connected to group %s", client.userID, client.groupID)

	h.workerPool.Submit(func() {
		ctx := h.getContextWithToken(client)
		_, _ = h.getUserInfoWithContext(ctx, client.userID)
	})

	h.broadcastOnlineUsersUpdate(client.groupID, client)
}

func (h *Hub) handleClientUnregister(client *Client) {
	h.roomsMutex.Lock()
	if clients, ok := h.rooms[client.groupID]; ok {
		if _, found := clients[client]; found {
			h.workerPool.Submit(func() {
				ctx := h.getContextWithToken(client)
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

	h.decrementActiveConnections()

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

	if !h.checkRateLimit(msg.SenderID) {
		log.Printf("Rate limit exceeded for user %s", msg.SenderID)
		return
	}

	senderClient := h.findSenderClient(msg.GroupID, msg.SenderID)
	if senderClient == nil {
		log.Printf("Could not find sender client for user %s", msg.SenderID)
		return
	}

	h.workerPool.Submit(func() {
		ctx := h.getContextWithToken(senderClient)
		userInfo, err := h.getUserInfoWithContext(ctx, msg.SenderID)
		if err == nil {
			msg.SenderInfo = userInfo
		} else {
			log.Printf("(Hub) 2 - Failed to get user info for %s: %v\n", msg.SenderID, err)
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
		default:
			log.Printf("Unknown message type: %s", msg.Type)
		}

		h.incrementProcessedCount()
	})
}

func (h *Hub) checkRateLimit(userID string) bool {
	h.rateMutex.Lock()
	defer h.rateMutex.Unlock()

	if _, exists := h.rateLimiter[userID]; !exists {
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

	for userID, ticker := range h.rateLimiter {
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

func (h *Hub) saveAndBroadcastMessage(msg Message) {
	senderClient := h.findSenderClient(msg.GroupID, msg.SenderID)
	if senderClient == nil {
		log.Printf("Could not find sender client for user %s", msg.SenderID)
		return
	}

	ctx := h.getContextWithToken(senderClient)

	groupID, err := primitive.ObjectIDFromHex(msg.GroupID)
	if err != nil {
		log.Printf("Invalid group ID: %v", err)
		h.incrementErrorCount()
		return
	}

	notReactedMembers, err := h.getNotReactedMembers(ctx, msg.GroupID, msg.SenderID, nil)
	if err != nil {
		log.Printf("%v", err)
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
		"type":                msg.Type,
		"group_id":            msg.GroupID,
		"sender_id":           msg.SenderID,
		"sender_infor":        msg.SenderInfo,
		"content":             msg.Content,
		"content_type":        msg.ContenType,
		"image_key":           msg.ImageKey,
		"is_limit_time_react": false,
		"not_reacted_members": notReactedMembers,
		"is_reacted":          false,
		"created_at": time.Now().Add(7 * time.Hour),
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

func (h *Hub) editAndBroadcastMessage(msg Message) {
	senderClient := h.findSenderClient(msg.GroupID, msg.SenderID)
	if senderClient == nil {
		log.Printf("Could not find sender client for user %s", msg.SenderID)
		return
	}

	ctx := h.getContextWithToken(senderClient)

	reacts, err := h.messageService.GetMessageReacts(ctx, msg.ID, msg.GroupID)
	if err != nil {
		log.Printf("Error getting message reacts: %v", err)
		h.incrementErrorCount()
		return
	}

	if reacts == nil {
		reacts = []*models.MessageReact{}
	}

	_, reactedUserIDs := h.populateUserInfoInReacts(ctx, reacts)

	notReactedMembers, err := h.getNotReactedMembers(ctx, msg.GroupID, msg.SenderID, reactedUserIDs)
	if err != nil {
		log.Printf("%v", err)
		h.incrementErrorCount()
		return
	}

	message, err := h.messageService.GetMessageByID(ctx, msg.ID)
	if err != nil {
		log.Printf("Error getting message: %v", err)
		h.incrementErrorCount()
		return
	}

	groupDetail, err := h.groupService.GetGroupDetail(ctx, msg.GroupID)
	if err != nil {
		log.Printf("Error getting group detail: %v", err)
		h.incrementErrorCount()
		return
	}

	isLimitTimeReact := false
	if groupDetail.Group.LimitTimeReact > 0 {
		expireTime := message.CreatedAt.Add(time.Duration(groupDetail.Group.LimitTimeReact) * time.Minute)
		
		if time.Now().After(expireTime) {
			isLimitTimeReact = true
		}
	}

	dbMsg := models.EditMessage{
		ID:       msg.ID,
		Content:  msg.Content,
		UpdateAt: time.Now(),
	}

	res := map[string]interface{}{
		"id":                  msg.ID,
		"type":                msg.Type,
		"group_id":            msg.GroupID,
		"is_edit":             true,
		"is_limit_time_react": isLimitTimeReact,
		"sender_id":           msg.SenderID,
		"sender_infor":        msg.SenderInfo,
		"content":             msg.Content,
		"not_reacted_members": notReactedMembers,
		"is_reacted":          false,
		"created_at":          msg.Timestamp,
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
	senderClient := h.findSenderClient(msg.GroupID, msg.SenderID)
	if senderClient == nil {
		log.Printf("Could not find sender client for user %s", msg.SenderID)
		return
	}

	ctx := h.getContextWithToken(senderClient)

	err := h.messageService.InsertMessageReact(ctx, msg.ID, msg.GroupID, msg.SenderID, msg.ReactType)
	if err != nil {
		log.Printf("Error inserting message react: %v", err)
		h.incrementErrorCount()
		return
	}

	reacts, err := h.messageService.GetMessageReacts(ctx, msg.ID, msg.GroupID)
	if err != nil {
		log.Printf("Error getting message reacts: %v", err)
		h.incrementErrorCount()
		return
	}

	if reacts == nil {
		reacts = []*models.MessageReact{}
	}

	totalAllReacts, reactedUserIDs := h.populateUserInfoInReacts(ctx, reacts)

	message, err := h.messageService.GetMessageByID(ctx, msg.ID)
	if err != nil {
		log.Printf("Error getting message: %v", err)
		h.incrementErrorCount()
		return
	}

	groupDetail, err := h.groupService.GetGroupDetail(ctx, msg.GroupID)
	if err != nil {
		log.Printf("Error getting group detail: %v", err)
		h.incrementErrorCount()
		return
	}

	var notReactedMembers []map[string]interface{}
	for _, member := range groupDetail.Members {
		userID := member.GroupMember.UserID
		if userID == message.SenderID {
			continue
		}

		if !reactedUserIDs[userID] {
			memberInfo := map[string]interface{}{
				"user_id":    member.GroupMember.UserID,
				"user_name":  member.GroupMember.UserInfor.UserName,
				"avatar_url": member.GroupMember.UserInfor.Avartar,
			}
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
		"not_reacted_members": notReactedMembers,
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
			default:
				deadClients = append(deadClients, client)
			}
		}

		for _, client := range deadClients {
			go func(c *Client) {
				select {
				case h.unregister <- c:
				default:
				}
			}(client)
		}
	}
}

func (h *Hub) incrementActiveConnections() {
	h.metrics.mutex.Lock()
	h.metrics.activeConnections++
	h.metrics.mutex.Unlock()
}

func (h *Hub) decrementActiveConnections() {
	h.metrics.mutex.Lock()
	h.metrics.activeConnections--
	h.metrics.mutex.Unlock()
}

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

func (h *Hub) Shutdown() {
	h.cancel()
	h.rateMutex.Lock()
	for _, ticker := range h.rateLimiter {
		ticker.Stop()
	}
	h.rateMutex.Unlock()
}

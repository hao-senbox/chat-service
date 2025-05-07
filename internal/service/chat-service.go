package service

import (
	"bytes"
	"chat-service/internal/models"
	"chat-service/internal/repository"
	"context"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ChatService interface {
	GetGroupMessages(ctx context.Context, groupID string) ([]*models.MessageWithUser, error)
	IsUserInGroup(ctx context.Context, userID string, groupID string) (bool, error)
	DownloadGroupMessages(ctx *gin.Context, groupID string) error
}
type chatService struct {
	chatRepository repository.ChatRepository
	groupService   GroupService
	userService    UserService
}


func NewChatService(chatRepository repository.ChatRepository, groupService GroupService, userService UserService) ChatService {
	return &chatService{
		chatRepository: chatRepository,
		groupService:   groupService,
		userService:    userService,
	}
}

func (s *chatService) GetGroupMessages(ctx context.Context, groupID string) ([]*models.MessageWithUser, error) {

	objectID, err := primitive.ObjectIDFromHex(groupID)
	if err != nil {
		return nil, err
	}

	messages, err := s.chatRepository.GetMessagesByGroupID(ctx, objectID)
	if err != nil {
		return nil, err
	}

	var enrichedMessages []*models.MessageWithUser

	userCache := make(map[string]*models.UserInfor)

	for _, msg := range messages {
		var userInfo *models.UserInfor
		if cached, ok := userCache[msg.SenderID]; ok {
			userInfo = cached
		} else {
			fmt.Printf("Fetching user info for %s\n", msg.SenderID)
			fetchedInfo, err := s.userService.GetUserInfor(msg.SenderID)
			if err != nil {
				fmt.Printf("Failed to get user info for %s: %v\n", msg.SenderID, err)
				fetchedInfo = &models.UserInfor{} 
			}
			userInfo = &models.UserInfor{
				UserID:   fetchedInfo.UserID,
				UserName: fetchedInfo.UserName,
				Avartar:  fetchedInfo.Avartar,
			}
			userCache[msg.SenderID] = userInfo
		}

		enrichedMessages = append(enrichedMessages, &models.MessageWithUser{
			SenderID:    msg.SenderID,
			Content:     msg.Content,
			CreatedAt:   msg.CreatedAt,
			SenderInfor: userInfo,
		})
	}
	

	return enrichedMessages, nil
}

func(s *chatService) IsUserInGroup(ctx context.Context, userID string, groupID string) (bool, error) {

	objectID, err := primitive.ObjectIDFromHex(groupID)
	if err != nil {
		return false, err
	}

	return s.chatRepository.IsUserInGroup(ctx, userID, objectID)
	
}

func (s *chatService) DownloadGroupMessages(ctx *gin.Context, groupID string) error {

	_, err := s.groupService.GetGroupDetail(ctx, groupID)
	if err != nil {
		return err
	}

	messages, err := s.GetGroupMessages(ctx, groupID)
	if err != nil {
		return err
	}

	var buffer bytes.Buffer
	for _, msg := range messages {
		timeStr := msg.CreatedAt.Format("2006-01-02 15:04:05")
		line := fmt.Sprintf("[%s] %s: %s\n", timeStr, msg.SenderInfor.UserName, msg.Content)
		buffer.WriteString(line)
	}

	fileName := fmt.Sprintf("group%s.txt", groupID)
	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	ctx.Header("Content-Type", "text/plain")
	ctx.String(http.StatusOK, buffer.String())

	return nil 
}
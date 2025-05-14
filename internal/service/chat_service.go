package service

import (
	"bytes"
	"chat-service/internal/models"
	"chat-service/internal/repository"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/consul/api"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ChatService interface {
	GetGroupMessages(ctx context.Context, groupID string, from *time.Time, to *time.Time) ([]*models.MessageWithUser, error)
	IsUserInGroup(ctx context.Context, userID string, groupID string) (bool, error)
	DownloadGroupMessages(ctx *gin.Context, groupID string, from *time.Time, to *time.Time) error
	GetUserInformation(ctx context.Context, userID string) (*models.UserInfor, error)
	DeleteMessage(ctx context.Context, messageID primitive.ObjectID, token string) error
	SaveMessage(ctx context.Context, message *models.Message) (primitive.ObjectID, error)
	EditMessage(ctx context.Context, message *models.EditMessage) error
	DeleteMessageByGroupID(ctx context.Context, groupID string, token string) error
}
type chatService struct {
	chatRepository repository.ChatRepository
	groupService   GroupService
	userService    UserService
	client         *callAPI
}


func NewChatService(client *api.Client, chatRepository repository.ChatRepository, groupService GroupService, userService UserService) ChatService {
	mainServiceAPI := NewServiceAPI(client, mainService)
	return &chatService{
		client:         mainServiceAPI,
		chatRepository: chatRepository,
		groupService:   groupService,
		userService:    userService,
	}
}

func (s *chatService) EditMessage(ctx context.Context, message *models.EditMessage) error {
	return s.chatRepository.EditMessage(ctx, message)
}

func (s *chatService) GetUserInformation(ctx context.Context, userID string) (*models.UserInfor, error) {
	
	user, err := s.userService.GetUserOnline(ctx, userID)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *chatService) SaveMessage(ctx context.Context, message *models.Message) (primitive.ObjectID, error) {
	return s.chatRepository.SaveMessage(ctx, message)
}

func (s *chatService) GetGroupMessages(ctx context.Context, groupID string, from *time.Time, to *time.Time) ([]*models.MessageWithUser, error) {

	objectID, err := primitive.ObjectIDFromHex(groupID)
	if err != nil {
		return nil, err
	}

	messages, err := s.chatRepository.GetMessagesByGroupID(ctx, objectID, from, to)
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
			ID:          msg.ID.Hex(),
			SenderID:    msg.SenderID,
			Content:     msg.Content,
			IsEdit:      msg.IsEdit,
			IsDelete:    msg.IsDelete,
			ContenType:  msg.ContenType,
			ImageKey:    msg.ImageKey,
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

func (s *chatService) DownloadGroupMessages(ctx *gin.Context, groupID string, from *time.Time, to *time.Time) error {

	_, err := s.groupService.GetGroupDetail(ctx, groupID)
	if err != nil {
		return err
	}

	messages, err := s.GetGroupMessages(ctx, groupID, from, to)
	if err != nil {
		return err
	}

	var buffer bytes.Buffer
	for _, msg := range messages {
		timeStr := msg.CreatedAt.Format("2006-01-02 15:04:05")
		content := msg.Content
		if msg.Content == "" {
			content = "Image"
		}
		line := fmt.Sprintf("[%s] %s: %s\n", timeStr, msg.SenderInfor.UserName, content)
		buffer.WriteString(line)
	}

	fileName := fmt.Sprintf("group%s.txt", groupID)
	ctx.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	ctx.Header("Content-Type", "text/plain")
	ctx.String(http.StatusOK, buffer.String())

	return nil 
}

func (s *chatService) DeleteMessageByGroupID(ctx context.Context, groupID string, token string) error {

	objectID, err := primitive.ObjectIDFromHex(groupID)
	if err != nil {
		return err
	}

	messages, err := s.chatRepository.GetMessagesByGroupID(ctx, objectID, nil, nil)
	if err != nil {
		return err
	}

	for _, message := range messages {
		err := s.DeleteMessage(ctx, message.ID, token)
		if err != nil {
			return err
		}
	}
	
	return nil
}
func (s *chatService) DeleteMessage(ctx context.Context, messageID primitive.ObjectID, token string) error {
	err := s.chatRepository.DeleteMessage(ctx, messageID)
	if err != nil {
		return err
	}

	message, err := s.chatRepository.MessageDetail(ctx, messageID)
	if err != nil {
		return err
	}

	if message.ImageKey != "" {
		err = s.client.DeleteImage(message.ImageKey, token)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *callAPI) DeleteImage(key string, token string) error {
	requestBody := map[string]string{
		"key": key,
	}
	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %v", err)
	}

	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + token,
	}	

	endpoint := "/v1/images/delete"
	_, err = c.client.CallAPI(c.clientServer, endpoint, http.MethodPost, jsonData, headers)
	if err != nil {
		return fmt.Errorf("error calling API: %v", err)
	}

	return nil
}

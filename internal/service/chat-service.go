package service

import (
	"chat-service/internal/models"
	"chat-service/internal/repository"
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ChatService interface {
	GetGroupMessages(ctx context.Context, groupID string) ([]*models.MessageWithUser, error)
	IsUserInGroup(ctx context.Context, userID string, groupID string) (bool, error)
}
type chatService struct {
	chatRepository repository.ChatRepository
	userService    UserService
}


func NewChatService(chatRepository repository.ChatRepository, userService UserService) ChatService {
	return &chatService{
		chatRepository: chatRepository,
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
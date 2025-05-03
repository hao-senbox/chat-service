package service

import (
	"chat-service/internal/models"
	"chat-service/internal/repository"
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ChatService interface {
	GetGroupMessages(ctx context.Context, groupID string) ([]*models.Message, error)
	IsUserInGroup(ctx context.Context, userID string, groupID string) (bool, error)
}
type chatService struct {
	chatRepository repository.ChatRepository
}


func NewChatService(chatRepository repository.ChatRepository) ChatService {
	return &chatService{
		chatRepository: chatRepository,
	}
}

func (s *chatService) GetGroupMessages(ctx context.Context, groupID string) ([]*models.Message, error) {

	objectID, err := primitive.ObjectIDFromHex(groupID)
	if err != nil {
		return nil, err
	}

	messages, err := s.chatRepository.GetMessagesByGroupID(ctx, objectID)
	if err != nil {
		return nil, err
	}

	return messages, nil
}

func(s *chatService) IsUserInGroup(ctx context.Context, userID string, groupID string) (bool, error) {

	objectID, err := primitive.ObjectIDFromHex(groupID)
	if err != nil {
		return false, err
	}

	return s.chatRepository.IsUserInGroup(ctx, userID, objectID)
	
}
package service

import (
	"chat-service/internal/models"
	"chat-service/internal/repository"
	"context"
	"fmt"
	"log"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type EmergencyService interface {
	CreateEmergency(ctx context.Context, req *models.EmergencyRequest) error
	GetNotificationsUser(ctx context.Context, userID string) ([]*models.EmergencyLogsResponse, error)
}

type emergencyService struct {
	emergencyRepository     repository.EmergencyRepository
	emergencyLogsRepository repository.EmergencyLogsRepository
	groupService            GroupService
	userService             UserService
	fireBase                *firebase.App
}

func NewEmergencyService(emergencyRepository repository.EmergencyRepository,
	groupService GroupService,
	userService UserService,
	emergencyLogsRepository repository.EmergencyLogsRepository,
	fireBase *firebase.App) EmergencyService {
	return &emergencyService{
		emergencyRepository:     emergencyRepository,
		groupService:            groupService,
		userService:             userService,
		emergencyLogsRepository: emergencyLogsRepository,
		fireBase:                fireBase,
	}
}

func (s *emergencyService) CreateEmergency(ctx context.Context, req *models.EmergencyRequest) error {

	if req.GroupID == "" {
		return fmt.Errorf("group ID must not be empty")
	}

	objectGroupID, err := primitive.ObjectIDFromHex(req.GroupID)
	if err != nil {
		return fmt.Errorf("invalid group ID: %w", err)
	}

	if req.Type == "" {
		return fmt.Errorf("type must not be empty")
	}

	if req.UserID == "" {
		return fmt.Errorf("user ID must not be empty")
	}

	emergencyID := primitive.NewObjectID()
	emergency := &models.Emergency{
		ID:        emergencyID,
		GroupID:   objectGroupID,
		UserID:    req.UserID,
		Type:      req.Type,
		Message:   req.Message,
		CreatedAt: time.Now(),
		UpdateAt:  time.Now(),
	}

	if err := s.emergencyRepository.Create(ctx, emergency); err != nil {
		return fmt.Errorf("failed to create emergency: %w", err)
	}

	emergencyLogs := &models.EmergencyLogs{
		ID:          primitive.NewObjectID(),
		EmergencyID: emergencyID,
		GroupID:     objectGroupID,
		UserID:      req.UserID,
		Type:        req.Type,
		Status:      "completed",
		IsSender:    true,
		Message:     req.Message,
		CreatedAt:   time.Now(),
		UpdateAt:    time.Now(),
	}

	if err := s.emergencyLogsRepository.Create(ctx, emergencyLogs); err != nil {
		return fmt.Errorf("failed to create emergency logs: %w", err)
	}

	group, err := s.groupService.GetGroupDetail(ctx, req.GroupID)
	if err != nil {
		return fmt.Errorf("failed to get group detail: %w", err)
	}

	for _, member := range group.Members {
		userID := member.GroupMember.UserID
		if userID == req.UserID {
			continue
		}

		emergencyReceiverLogs := &models.EmergencyLogs{
			ID:          primitive.NewObjectID(),
			EmergencyID: emergencyID,
			GroupID:     objectGroupID,
			UserID:      userID,
			Type:        req.Type,
			Status:      "completed",
			IsSender:    false,
			Message:     req.Message,
			CreatedAt:   time.Now(),
			UpdateAt:    time.Now(),
		}

		if err := s.emergencyLogsRepository.Create(ctx, emergencyReceiverLogs); err != nil {
			return fmt.Errorf("failed to create emergency receiver logs: %w", err)
		}

		tokens, err := s.userService.GetTokenUser(ctx, userID)
		if err != nil {
			log.Printf("Failed to get tokens for user %s: %v", userID, err)
			continue
		}

		if tokens == nil || len(*tokens) == 0 {
			continue
		}

		for _, token := range *tokens {
			if token == "" {
				continue
			}
			if err := s.sendToToken(token); err != nil {
				log.Printf("Failed to send to token %s: %v", token, err)
				continue
			}
		}
	}

	return nil
}

func (s *emergencyService) sendToToken(token string) error {

	ctx := context.Background()

	client, err := s.fireBase.Messaging(ctx)
	if err != nil {
		return fmt.Errorf("error getting Messaging client: %w", err)
	}

	message := &messaging.Message{
		Notification: &messaging.Notification{
			Title: "Emergency Notification",
		},
		Token: token,
	}

	_, err = client.Send(ctx, message)
	if err != nil {
		return fmt.Errorf("error sending message: %w", err)
	}

	log.Printf("Sent FCM notification to token: %s", token)

	return nil

}

func (s *emergencyService) GetNotificationsUser(ctx context.Context, userID string) ([]*models.EmergencyLogsResponse, error) {
	var result []*models.EmergencyLogsResponse
	emergencies, err := s.emergencyLogsRepository.GetNotificationsUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	for _, emergency := range emergencies {
		group, err := s.groupService.GetGroupDetail(ctx, emergency.GroupID.Hex())
		if err != nil {
			return nil, err
		}

		emergencyResponse := &models.EmergencyLogsResponse{
			ID:          emergency.ID.Hex(),
			EmergencyID: emergency.EmergencyID.Hex(),
			GroupResponse: models.GroupResponse{
				ID:          group.Group.ID.Hex(),
				Name:        group.Group.Name,
				Description: group.Group.Description,
				CreatedAt:   group.Group.CreatedAt,
				UpdateAt:    group.Group.UpdateAt,
			},
			UserID:      emergency.UserID,
			Type:        emergency.Type,
			Status:      emergency.Status,
			IsSender:    emergency.IsSender,
			Message:     emergency.Message,
			CreatedAt:   emergency.CreatedAt,
			UpdateAt:    emergency.UpdateAt,
		}

		result = append(result, emergencyResponse)
	}

	return result, nil
}
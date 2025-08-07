package service

import (
	"chat-service/internal/models"
	"chat-service/internal/repository"
	"context"
	"errors"
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
	UpdateEmergency(ctx context.Context, status string, id string) error
	SenPendingNotifications(ctx context.Context) error
}

type emergencyService struct {
	emergencyRepository     repository.EmergencyRepository
	emergencyLogsRepository repository.EmergencyLogsRepository
	groupService            GroupService
	groupRepo               repository.GroupRepository
	messageRepo             repository.MessagesRepository
	userService             UserService
	fireBase                *firebase.App
}

func NewEmergencyService(emergencyRepository repository.EmergencyRepository,
	groupService GroupService,
	userService UserService,
	emergencyLogsRepository repository.EmergencyLogsRepository,
	groupRepo repository.GroupRepository,
	messageRepo repository.MessagesRepository,
	fireBase *firebase.App) EmergencyService {
	return &emergencyService{
		emergencyRepository:     emergencyRepository,
		groupService:            groupService,
		userService:             userService,
		emergencyLogsRepository: emergencyLogsRepository,
		groupRepo:               groupRepo,
		messageRepo:             messageRepo,
		fireBase:                fireBase,
	}
}

func (s *emergencyService) CreateEmergency(ctx context.Context, req *models.EmergencyRequest) error {

	if req == nil {
		return fmt.Errorf("request cannot be nil")
	}

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

	group, err := s.groupService.GetGroupDetail(ctx, req.GroupID)
	if err != nil {
		return fmt.Errorf("failed to get group detail: %w", err)
	}

	if group == nil {
		return fmt.Errorf("group not found")
	}

	if len(group.Members) <= 1 {
		return fmt.Errorf("group must have at least 2 members")
	}

	emergencyID := primitive.NewObjectID()
	now := time.Now()

	emergency := &models.Emergency{
		ID:        emergencyID,
		GroupID:   objectGroupID,
		UserID:    req.UserID,
		Type:      req.Type,
		Message:   req.Message,
		CreatedAt: now,
		UpdateAt:  now,
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
		CreatedAt:   now,
		UpdateAt:    now,
	}

	if err := s.emergencyLogsRepository.Create(ctx, emergencyLogs); err != nil {
		return fmt.Errorf("failed to create emergency logs: %w", err)
	}

	groupEmergencyID := primitive.NewObjectID()
	groupEmergency := models.Group{
		ID:             groupEmergencyID,
		Name:           fmt.Sprintf("EMERGENCY-%s", emergencyID.Hex()),
		Description:    fmt.Sprintf("EMERGENCY-%s", emergencyID.Hex()),
		GroupQr:        "",
		LimitTimeReact: 0,
		Type:           "emergency",
		MemberCount:    0,
		CreatedBy:      req.UserID,
		CreatedAt:      now,
		UpdateAt:       now,
	}

	if err := s.groupRepo.CreateGroup(ctx, &groupEmergency); err != nil {
		return fmt.Errorf("failed to create emergency group: %w", err)
	}

	var failedNotifications []string

	for _, member := range group.Members {
		if member.GroupMember == nil {
			continue
		}

		userID := member.GroupMember.UserID
		if userID == req.UserID {

			err := s.groupService.AddUserToGroup(ctx, &models.GroupUserRequest{
				GroupID: groupEmergencyID.Hex(),
				UserID:  userID,
				Type:    "member",
			})

			if err != nil {
				log.Printf("Failed to add user %s to emergency group: %v", userID, err)
				continue
			}

			continue
		}

		err := s.groupService.AddUserToGroup(ctx, &models.GroupUserRequest{
			GroupID: groupEmergencyID.Hex(),
			UserID:  userID,
			Type:    "member",
		})

		if err != nil {
			log.Printf("Failed to add user %s to emergency group: %v", userID, err)
			continue
		}

		emergencyReceiverLogs := &models.EmergencyLogs{
			ID:          primitive.NewObjectID(),
			EmergencyID: emergencyID,
			GroupID:     objectGroupID,
			UserID:      userID,
			Type:        req.Type,
			Status:      "pending",
			IsSender:    false,
			Message:     req.Message,
			CreatedAt:   now,
			UpdateAt:    now,
		}

		if err := s.emergencyLogsRepository.Create(ctx, emergencyReceiverLogs); err != nil {
			log.Printf("Failed to create emergency receiver logs for user %s: %v", userID, err)
			continue
		}

		tokens, err := s.userService.GetTokenUser(ctx, userID)
		if err != nil {
			log.Printf("Failed to get tokens for user %s: %v", userID, err)
			failedNotifications = append(failedNotifications, userID)
			continue
		}

		if tokens == nil || len(*tokens) == 0 {
			log.Printf("No tokens found for user %s", userID)
			continue
		}

		tokenSent := false
		for _, token := range *tokens {
			if token == "" {
				continue
			}

			if err := s.sendToToken(token); err != nil {
				log.Printf("Failed to send notification to token %s for user %s: %v", token, userID, err)
			} else {
				tokenSent = true
			}
		}

		if !tokenSent {
			failedNotifications = append(failedNotifications, userID)
		}
	}

	_, err = s.messageRepo.SaveMessage(ctx, &models.Message{
		ID:         primitive.NewObjectID(),
		GroupID:    groupEmergencyID,
		SenderID:   req.UserID,
		Content:    "Emergency Notification",
		ContenType: "text",
		CreatedAt:  now,
		UpdateAt:   now,
	})

	if err != nil {
		log.Printf("Failed to save initial emergency message: %v", err)
	}

	if len(failedNotifications) > 0 {
		log.Printf("Failed to send notifications to users: %v", failedNotifications)
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
			UserID:    emergency.UserID,
			Type:      emergency.Type,
			Status:    emergency.Status,
			IsSender:  emergency.IsSender,
			Message:   emergency.Message,
			CreatedAt: emergency.CreatedAt,
			UpdateAt:  emergency.UpdateAt,
		}

		result = append(result, emergencyResponse)
	}

	return result, nil
}

func (s *emergencyService) UpdateEmergency(ctx context.Context, status string, id string) error {

	if status == "" {
		return errors.New("status is required")
	}

	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	return s.emergencyLogsRepository.UpdateEmergencyStatus(ctx, objID, status)

}

func (s *emergencyService) SenPendingNotifications(ctx context.Context) error {
	pendings, err := s.emergencyLogsRepository.GetPendingNotifications(ctx)
	if err != nil {
		return err
	}

	for _, pending := range pendings {

		userID := pending.UserID

		tokens, err := s.userService.GetTokenUser(ctx, userID)
		if err != nil {
			log.Printf("Failed to get tokens for user %s: %v", userID, err)
			continue
		}

		if tokens == nil || len(*tokens) == 0 {
			log.Printf("No tokens found for user %s", userID)
			continue
		}

		for _, token := range *tokens {
			if token == "" {
				continue
			}

			if err := s.sendToToken(token); err != nil {
				log.Printf("Failed to send notification to token %s for user %s: %v", token, userID, err)
				continue
			}
		}
	}

	return nil

}


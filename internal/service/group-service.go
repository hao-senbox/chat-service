package service

import (
	"chat-service/internal/models"
	"chat-service/internal/repository"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type GroupService interface {
	CreateGroup(ctx context.Context, group *models.GroupRequest) error
	GetUserGroups(ctx context.Context, userID string) ([]*models.Group, error)
	CreateGroupUser(ctx context.Context, group *models.GroupUserRequest) error
}

type groupService struct {
	groupRepository repository.GroupRepository
	groupUserRepository repository.GroupMemberRepository
}

func NewGroupService(groupRepository repository.GroupRepository, groupUserRepository repository.GroupMemberRepository) GroupService {
	return &groupService{
		groupRepository: groupRepository,
		groupUserRepository: groupUserRepository,
	}
}

	func (s *groupService) GetUserGroups(ctx context.Context, userID string) ([]*models.Group, error) {
		
		groupUser, err := s.groupRepository.GetUserGroups(ctx, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to get user group data: %w", err)
		}

		return groupUser, nil
	}


func (s *groupService) CreateGroup(ctx context.Context, group *models.GroupRequest) error {
	
	if group.Name == "" {
		return fmt.Errorf("name cannot be empty")
	} 

	if group.Description == "" {
		return fmt.Errorf("description cannot be empty")
	}

	if group.CreatedBy == "" {
		return fmt.Errorf("created by cannot be empty")
	}

	groupInfor := models.Group{
		Name:        group.Name,
		Description: group.Description,
		CreatedBy:   group.CreatedBy,
		CreatedAt:   time.Now(),
		UpdateAt:    time.Now(),
	}

	if err := s.groupRepository.CreateGroup(ctx, &groupInfor); err != nil {
		return fmt.Errorf("failed to create group: %w", err)
	}

	return nil
}

func (s *groupService) CreateGroupUser(ctx context.Context, group *models.GroupUserRequest) error {
	
	objectID, err := primitive.ObjectIDFromHex(group.GroupID)
	if err != nil {
		return err
	}

	_, err = s.groupRepository.GetGroupDetail(ctx, objectID)
	if err != nil {
		return fmt.Errorf("failed to get group detail: %w", err)
	}

	if group.GroupID == "" {
		return fmt.Errorf("group id cannot be empty")
	}

	if group.UserID == "" {
		return fmt.Errorf("user id cannot be empty")
	}
	
	groupUserInfor := models.GroupMember {
		GroupID:    objectID,
		UserID:     group.UserID,
		Permission: models.Permission{
			CanRead:            group.CanRead,
			CanWrite:           group.CanWrite,
			CanEdit:            group.CanEdit,
			CanSendImages:      group.CanSendImages,
			CanUseCameraDevice: group.CanUseCameraDevice,
		},
		CreatedAt:  time.Now(),
		UpdateAt:   time.Now(),
	}

	if err := s.groupUserRepository.CreateGroupUser(ctx, &groupUserInfor); err != nil {
		return fmt.Errorf("failed to create group user: %w", err)
	}

	return nil
}	
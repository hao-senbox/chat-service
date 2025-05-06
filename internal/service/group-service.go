package service

import (
	"chat-service/internal/models"
	"chat-service/internal/repository"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type GroupService interface {
	CreateGroup(ctx context.Context, group *models.GroupRequest) error
	GetUserGroups(ctx context.Context, userID string) ([]*models.Group, error)
	AddUserToGroup(ctx context.Context, group *models.GroupUserRequest) error
	GetAllGroups(ctx context.Context) ([]models.GroupWithMembers, error)
	GetGroupDetail(ctx context.Context, groupID string) (*models.GroupWithMembers, error)
	UpdateGroup(ctx context.Context,groupID string ,group *models.GroupRequest) error
	DeleteGroup(ctx context.Context, groupID string) error
	RemoveUserFromGroup(ctx context.Context, groupID string, group *models.GroupUserRequest) error
}

type groupService struct {
	groupRepository     repository.GroupRepository
	groupUserRepository repository.GroupMemberRepository
	userService         UserService
}

func NewGroupService(groupRepository repository.GroupRepository, groupUserRepository repository.GroupMemberRepository, userService UserService) GroupService {
	return &groupService{
		groupRepository:     groupRepository,
		groupUserRepository: groupUserRepository,
		userService:         userService,
	}
}
func (s *groupService) GetAllGroups(ctx context.Context) ([]models.GroupWithMembers, error) {

	var result []models.GroupWithMembers
	groups, err := s.groupRepository.GetAllGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all groups: %w", err)
	}

	for _, group := range groups {
		members, err := s.groupUserRepository.GetGroupMembers(ctx, group.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get group members: %w", err)
		}

		var memberWithInfor []models.GroupMemberWithUserInfor

		for _, member := range members {
			userInfor, err := s.userService.GetUserInfor(member.UserID)
			member.UserInfor = userInfor
			if err != nil {
				return nil, fmt.Errorf("failed to get user infor: %w", err)
			}

			memberWithInfor = append(memberWithInfor, models.GroupMemberWithUserInfor{
				GroupMember: member,
			})

		}

		result = append(result, models.GroupWithMembers{
			Group:   *group,
			Members: memberWithInfor,
		})
	}

	return result, nil
}

func (s *groupService) GetGroupDetail(ctx context.Context, groupID string) (*models.GroupWithMembers, error) {

	var result *models.GroupWithMembers

	objectID, err := primitive.ObjectIDFromHex(groupID)
	if err != nil {
		return nil, fmt.Errorf("invalid group id: %w", err)
	}

	group, err := s.groupRepository.GetGroupDetail(ctx, objectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get group detail: %w", err)
	}

	members, err := s.groupUserRepository.GetGroupMembers(ctx, objectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get group members: %w", err)
	}

	var memberWithInfor []models.GroupMemberWithUserInfor

	for _, member := range members {
		userInfor, err := s.userService.GetUserInfor(member.UserID)
		member.UserInfor = userInfor
		if err != nil {
			return nil, fmt.Errorf("failed to get user infor: %w", err)
		}

		memberWithInfor = append(memberWithInfor, models.GroupMemberWithUserInfor{
			GroupMember: member,
		})
	}

	result = &models.GroupWithMembers{
		Group:   *group,
		Members: memberWithInfor,
	}

	return result, nil
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

func (s *groupService) AddUserToGroup(ctx context.Context, group *models.GroupUserRequest) error {
    // Kiểm tra ID hợp lệ
    if group.GroupID == "" {
        return fmt.Errorf("group id cannot be empty")
    }

    if group.UserID == "" {
        return fmt.Errorf("user id cannot be empty")
    }

    objectID, err := primitive.ObjectIDFromHex(group.GroupID)
    if err != nil {
        return err
    }

    // Kiểm tra group tồn tại
    _, err = s.groupRepository.GetGroupDetail(ctx, objectID)
    if err != nil {
        return fmt.Errorf("failed to get group detail: %w", err)
    }

    _, err = s.groupUserRepository.GetgroupMemberDetail(ctx, group.UserID)
    if err == nil {
        return fmt.Errorf("user already in group")
    } else if err != mongo.ErrNoDocuments {
        return fmt.Errorf("error checking if user is in group: %w", err)
    }

    groupUserInfor := models.GroupMember{
        GroupID: objectID,
        UserID:  group.UserID,
        Permission: models.Permission{
            CanRead:            group.CanRead,
            CanWrite:           group.CanWrite,
            CanEdit:            group.CanEdit,
            CanSendImages:      group.CanSendImages,
            CanUseCameraDevice: group.CanUseCameraDevice,
        },
        CreatedAt: time.Now(),
        UpdateAt:  time.Now(),
    }

    if err := s.groupUserRepository.AddUserToGroup(ctx, &groupUserInfor); err != nil {
        return fmt.Errorf("failed to create group user: %w", err)
    }

    return nil
}

func (s *groupService) UpdateGroup(ctx context.Context, groupID string, group *models.GroupRequest) error {
	
	objectID, err := primitive.ObjectIDFromHex(groupID)
	if err != nil {
		return err
	}

	_, err = s.groupRepository.GetGroupDetail(ctx, objectID)
	if err != nil {
		return fmt.Errorf("failed to get group detail: %w", err)
	}

	if group.Name == "" {
		return fmt.Errorf("name cannot be empty")
	}

	err = s.groupRepository.UpdateGroup(ctx, objectID, group)
	if err != nil {
		return fmt.Errorf("failed to update group: %w", err)
	}

	return nil
}

func (s *groupService) DeleteGroup(ctx context.Context, groupID string) error {

	objectID, err := primitive.ObjectIDFromHex(groupID)
	if err != nil {
		return err
	}

	_, err = s.groupRepository.GetGroupDetail(ctx, objectID)
	if err != nil {
		return fmt.Errorf("failed to get group detail: %w", err)
	}

	err = s.groupRepository.DeleteGroup(ctx, objectID)
	if err != nil {
		return fmt.Errorf("failed to delete group: %w", err)
	}

	return nil
}

func (s *groupService) RemoveUserFromGroup(ctx context.Context, groupID string, group *models.GroupUserRequest) error{

	objectID, err := primitive.ObjectIDFromHex(groupID)
	if err != nil {
		return err
	}

	if group.UserID == "" {
		return fmt.Errorf("user id cannot be empty")
	}

	err = s.groupUserRepository.DeleteUserFromGroup(ctx, objectID, group.UserID)
	if err != nil {
		return fmt.Errorf("failed to delete user from group: %w", err)
	}

	return nil
}
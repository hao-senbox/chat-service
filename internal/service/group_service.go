package service

import (
	"chat-service/internal/models"
	"chat-service/internal/repository"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"log"
	"time"
)

type GroupService interface {
	CreateGroup(ctx context.Context, group *models.GroupRequest) error
	GetUserGroups(ctx context.Context, userID string) ([]*models.Group, error)
	AddUserToGroup(ctx context.Context, group *models.GroupUserRequest) error
	GetAllGroups(ctx context.Context) ([]models.GroupWithMembers, error)
	GetGroupDetail(ctx context.Context, groupID string) (*models.GroupWithMembers, error)
	UpdateGroup(ctx context.Context, groupID string, group *models.GroupRequest) error
	DeleteGroup(ctx context.Context, groupID string, req *models.TokenUserRequest) error
	RemoveUserFromGroup(ctx context.Context, groupID string, group *models.GroupUserRequest) error
	CountKeywordAllGroups(ctx context.Context, keyword string) (*[]models.KeywordOfAllGroups, error)
	// GenerateGroupQrCode(ctx context.Context, group *models.GroupQrRequest) (string, error)
	JoinGroupByQrCode(ctx context.Context, group *models.JoinGroupByQrCodeRequest) error
	SetMessageService(chatService ChatService)
}

type groupService struct {
	groupRepository         repository.GroupRepository
	groupUserRepository     repository.GroupMemberRepository
	messagesRepository      repository.MessagesRepository
	messagesReactRepository repository.MessageReactRepository
	chatService             ChatService
	userService             UserService
}

func NewGroupService(groupRepository repository.GroupRepository,
	groupUserRepository repository.GroupMemberRepository,
	messagesRepository repository.MessagesRepository,
	userService UserService,
	chatService ChatService,
	messagesReactRepository repository.MessageReactRepository,
) GroupService {
	return &groupService{
		groupRepository:         groupRepository,
		groupUserRepository:     groupUserRepository,
		messagesRepository:      messagesRepository,
		userService:             userService,
		messagesReactRepository: messagesReactRepository,
		chatService:             chatService,
	}
}

func (s *groupService) SetMessageService(chatService ChatService) {
	s.chatService = chatService
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
			userInfor, err := s.userService.GetUserInfor(ctx, member.UserID)
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
		userInfor, err := s.userService.GetUserInfor(ctx, member.UserID)
		member.UserInfor = userInfor
		if err != nil {
			return nil, fmt.Errorf("failed to get user infor: %w", err)
		}

		reactCounts, err := s.messagesReactRepository.GetUserReactCountsInGroup(ctx, member.UserID, objectID)
		if err != nil {
			log.Printf("failed to get react counts for user %s: %v", member.UserID, err)
			reactCounts = []*models.ReactTypeCountOfUser{}
		}

		validReacts := []string{"understand", "ok", "done", "help", "unclear"}

		reactMap := make(map[string]int64)
		for _, reactCount := range reactCounts {
			reactMap[reactCount.ReactType] = reactCount.Count
		}

		var finalReactList []*models.ReactTypeCountOfUser
		for _, validReact := range validReacts {
			if reactCount, ok := reactMap[validReact]; ok {
				finalReactList = append(finalReactList, &models.ReactTypeCountOfUser{
					ReactType: validReact,
					Count:     reactCount,
				})
			} else {
				finalReactList = append(finalReactList, &models.ReactTypeCountOfUser{
					ReactType: validReact,
					Count:     0,
				})
			}
		}

		member.ReactOfUser = finalReactList

		memberWithInfor = append(memberWithInfor, models.GroupMemberWithUserInfor{
			GroupMember: member,
		})
	}

	totalMessage, err := s.messagesRepository.GetCountMessageGroup(ctx, objectID)
	if err != nil {
		return nil, fmt.Errorf("failed to get total message of group: %w", err)
	}

	result = &models.GroupWithMembers{
		Group:               *group,
		TotalMessageOfGroup: totalMessage,
		Members:             memberWithInfor,
	}

	return result, nil
}

func (s *groupService) GetUserGroups(ctx context.Context, userID string) ([]*models.Group, error) {

	groupUser, err := s.groupRepository.GetUserGroups(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user group data: %w", err)
	}

	for _, group := range groupUser {
		countMessages, err := s.messagesRepository.CountNonUserMessage(ctx, group.ID, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to count non user message: %w", err)
		}

		countReactUser, err := s.messagesReactRepository.CountMessageUserReacted(ctx, group.ID, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to count message user reacted: %w", err)
		}
		countMessagesGroup := countMessages - countReactUser
		group.UnreadCount = countMessagesGroup
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

	if group.LimitTimeReact == 0 {
		return fmt.Errorf("limit time react cannot be empty")
	}

	if group.LimitTimeReact < 0 {
		return fmt.Errorf("limit time react cannot be negative")
	}

	id := primitive.NewObjectID()

	groupInfor := models.Group{
		ID:             id,
		Name:           group.Name,
		Description:    group.Description,
		GroupQr:        fmt.Sprintf("SENBOX.ORG[EMERGENCY-ROOM]:%s", id.Hex()),
		LimitTimeReact: group.LimitTimeReact,
		Type:           "chat",
		CreatedBy:      group.CreatedBy,
		CreatedAt:      time.Now(), 
		UpdateAt:       time.Now(),
	}

	if err := s.groupRepository.CreateGroup(ctx, &groupInfor); err != nil {
		return fmt.Errorf("failed to create group: %w", err)
	}

	return nil
}

func (s *groupService) AddUserToGroup(ctx context.Context, group *models.GroupUserRequest) error {

	if group.GroupID == "" {
		return fmt.Errorf("group id cannot be empty")
	}

	if group.UserID == "" {
		return fmt.Errorf("user id cannot be empty")
	}

	if group.Type == "" {
		return fmt.Errorf("type cannot be empty")
	}

	objectID, err := primitive.ObjectIDFromHex(group.GroupID)
	if err != nil {
		return err
	}

	_, err = s.groupRepository.GetGroupDetail(ctx, objectID)
	if err != nil {
		return fmt.Errorf("failed to get group detail: %w", err)
	}

	_, err = s.groupUserRepository.GetgroupMemberDetail(ctx, group.UserID, objectID)
	if err == nil {
		return fmt.Errorf("user already in group")
	} else if err != mongo.ErrNoDocuments {
		return fmt.Errorf("error checking if user is in group: %w", err)
	}

	groupUserInfor := models.GroupMember{
		GroupID: objectID,
		UserID:  group.UserID,
		Type:    group.Type,
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

func (s *groupService) DeleteGroup(ctx context.Context, groupID string, req *models.TokenUserRequest) error {

	token := req.Token
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

	err = s.chatService.DeleteMessageByGroupID(ctx, groupID, token)
	if err != nil {
		return fmt.Errorf("failed to delete group members: %w", err)
	}

	return nil
}

func (s *groupService) RemoveUserFromGroup(ctx context.Context, groupID string, group *models.GroupUserRequest) error {

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

func (s *groupService) CountKeywordAllGroups(ctx context.Context, keyword string) (*[]models.KeywordOfAllGroups, error) {

	groups, err := s.GetAllGroups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all groups: %w", err)
	}

	var result []models.KeywordOfAllGroups

	for _, group := range groups {
		count, arrayID, err := s.messagesRepository.CountKeywordMessage(ctx, keyword, group.Group.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to count keyword all groups: %w", err)
		}
		data := models.KeywordOfAllGroups{
			Quantity:     count,
			Groups:       group.Group,
			ArrIdMessage: arrayID,
		}

		result = append(result, data)
	}

	return &result, nil
}

// func (s *groupService) GenerateGroupQrCode(ctx context.Context, group *models.GroupQrRequest) (string, error) {

// 	if group.GroupID == "" {
// 		return "", fmt.Errorf("group id cannot be empty")
// 	}

// 	objectID, err := primitive.ObjectIDFromHex(group.GroupID)
// 	if err != nil {
// 		return "", err
// 	}

// 	_, err = s.groupRepository.GetGroupDetail(ctx, objectID)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to get group detail: %w", err)
// 	}

// 	qrCode := models.QrCodeData{
// 		GroupID: objectID,
// 		Permission: models.Permission{
// 			CanRead:            group.CanRead,
// 			CanWrite:           group.CanWrite,
// 			CanEdit:            group.CanEdit,
// 			CanSendImages:      group.CanSendImages,
// 			CanUseCameraDevice: group.CanUseCameraDevice,
// 		},
// 		ExpiryTime: time.Now().Add(1 * time.Hour),
// 	}

// 	qrCodeJson, err := json.Marshal(qrCode)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to marshal qr code: %w", err)
// 	}

// 	qrCodeString := base64.StdEncoding.EncodeToString(qrCodeJson)

// 	// qr, err := qrcode.Encode(qrCodeString, qrcode.Medium, 256)
// 	// if err != nil {
// 	// 	return "", fmt.Errorf("failed to encode qr code: %w", err)
// 	// }

// 	// qrBase64 := base64.StdEncoding.EncodeToString(qr)

// 	groupQr := models.GroupQrCode{
// 		QRCode: qrCodeString,
// 		Permission: models.Permission{
// 			CanRead:            group.CanRead,
// 			CanWrite:           group.CanWrite,
// 			CanEdit:            group.CanEdit,
// 			CanSendImages:      group.CanSendImages,
// 			CanUseCameraDevice: group.CanUseCameraDevice,
// 		},
// 		ExpiryTime: time.Now().Add(1 * time.Hour),
// 		CreatedAt:  time.Now(),
// 	}

// 	err = s.groupRepository.CreateGroupQrCode(ctx, objectID, &groupQr)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to create group qr code: %w", err)
// 	}

// 	return qrCodeString, nil

// }

func (h *groupService) JoinGroupByQrCode(ctx context.Context, group *models.JoinGroupByQrCodeRequest) error {

	if group.QrCodeData == "" {
		return fmt.Errorf("qr code data cannot be empty")
	}

	if group.UserID == "" {
		return fmt.Errorf("user id cannot be empty")
	}

	qrCodeData, err := base64.StdEncoding.DecodeString(group.QrCodeData)
	if err != nil {
		return fmt.Errorf("failed to decode qr code data: %w", err)
	}

	var qrCode models.QrCodeData
	err = json.Unmarshal(qrCodeData, &qrCode)
	if err != nil {
		// Log chuỗi gây lỗi để debug
		log.Printf("Failed to unmarshal QR data: %v. Data received: %s", err, string(qrCodeData))
		return fmt.Errorf("failed to unmarshal qr code data: %w", err)
	}

	if qrCode.ExpiryTime.Before(time.Now()) {
		return fmt.Errorf("qr code has expired")
	}

	objectID, err := primitive.ObjectIDFromHex(qrCode.GroupID.Hex())
	if err != nil {
		return err
	}

	_, err = h.groupRepository.GetGroupDetail(ctx, objectID)
	if err != nil {
		return fmt.Errorf("failed to get group detail: %w", err)
	}

	groupUserInfor := models.GroupMember{
		GroupID:    objectID,
		UserID:     group.UserID,
		Permission: qrCode.Permission,
		CreatedAt:  time.Now(),
		UpdateAt:   time.Now(),
	}

	err = h.groupUserRepository.AddUserToGroup(ctx, &groupUserInfor)

	if err != nil {
		return fmt.Errorf("failed to create group member: %w", err)
	}

	return nil
}

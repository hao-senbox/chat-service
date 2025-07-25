package service

import (
	"bytes"
	"chat-service/internal/models"
	"chat-service/internal/repository"
	"chat-service/pkg/constants"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/consul/api"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ChatService interface {
	GetGroupMessages(ctx context.Context, groupID string, userID *string, from *time.Time, to *time.Time, pagination *models.Panigination) ([]*models.MessageWithUser, int64, error)
	IsUserInGroup(ctx context.Context, userID string, groupID string) (bool, error)
	DownloadGroupMessages(ctx *gin.Context, groupID string, from *time.Time, to *time.Time) error
	GetUserInformation(ctx context.Context, userID string) (*models.UserInfor, error)
	DeleteMessage(ctx context.Context, messageID primitive.ObjectID, token string) error
	SaveMessage(ctx context.Context, message *models.Message) (primitive.ObjectID, error)
	EditMessage(ctx context.Context, message *models.EditMessage) error
	DeleteMessageByGroupID(ctx context.Context, groupID string, token string) error
	MarkAsRead(ctx context.Context, messageID string, userID string, senderID string, groupID string) error
	InsertMessageReact(ctx context.Context, messageID, groupID, userID, reactType string) error
	GetMessageReacts(ctx context.Context, messageID, groupID string) ([]*models.MessageReact, error)
	DeleteMessageReacts(ctx context.Context, messageID, groupID string) error
	GetMessageByID(ctx context.Context, messageID string) (*models.Message, error)
}

type chatService struct {
	messagesRepository      repository.MessagesRepository
	messagesReadRepository  repository.ReadMessageRepository
	messagesReactRepository repository.MessageReactRepository
	groupService            GroupService
	userService             UserService
	client                  *callAPI
}

func NewChatService(client *api.Client,
	messagesRepository repository.MessagesRepository,
	messagesReadRepository repository.ReadMessageRepository,
	groupService GroupService,
	userService UserService,
	messagesReactRepository repository.MessageReactRepository) ChatService {
	mainServiceAPI := NewServiceAPI(client, mainService)
	return &chatService{
		client:                  mainServiceAPI,
		messagesRepository:      messagesRepository,
		groupService:            groupService,
		userService:             userService,
		messagesReadRepository:  messagesReadRepository,
		messagesReactRepository: messagesReactRepository,
	}
}

func (s *chatService) GetMessageReacts(ctx context.Context, messageID, groupID string) ([]*models.MessageReact, error) {

	objectGroupID, err := primitive.ObjectIDFromHex(groupID)
	if err != nil {
		return nil, err
	}

	objectMessageID, err := primitive.ObjectIDFromHex(messageID)
	if err != nil {
		return nil, err
	}

	messages, err := s.messagesReactRepository.GetMessageReact(ctx, objectMessageID, objectGroupID)

	if err != nil {
		return nil, err
	}

	return messages, nil
}
func (s *chatService) InsertMessageReact(ctx context.Context, messageID, groupID, userID, reactType string) error {

	objectGroupID, err := primitive.ObjectIDFromHex(groupID)
	if err != nil {
		return err
	}

	objectMessageID, err := primitive.ObjectIDFromHex(messageID)
	if err != nil {
		return err
	}

	messagesReact := &models.MessageReact{
		MessageID: objectMessageID,
		GroupID:   objectGroupID,
		UserID:    userID,
		React:     reactType,
	}

	err = s.messagesReactRepository.InsertMessageReact(ctx, messagesReact)
	if err != nil {
		return err
	}

	return nil
}

func (s *chatService) MarkAsRead(ctx context.Context, messageID string, userID string, senderID string, groupID string) error {

	objectGroupID, err := primitive.ObjectIDFromHex(groupID)
	if err != nil {
		return err
	}

	objectMessageID, err := primitive.ObjectIDFromHex(messageID)
	if err != nil {
		return err
	}

	return s.messagesReadRepository.MarkAsRead(ctx, objectMessageID, userID, senderID, objectGroupID)
}

func (s *chatService) EditMessage(ctx context.Context, message *models.EditMessage) error {
	return s.messagesRepository.EditMessage(ctx, message)
}

func (s *chatService) GetUserInformation(ctx context.Context, userID string) (*models.UserInfor, error) {

	user, err := s.userService.GetUserOnline(ctx, userID)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *chatService) SaveMessage(ctx context.Context, message *models.Message) (primitive.ObjectID, error) {
	return s.messagesRepository.SaveMessage(ctx, message)
}

func (s *chatService) GetGroupMessages(ctx context.Context, groupID string, userID *string, from *time.Time, to *time.Time, pagination *models.Panigination) ([]*models.MessageWithUser, int64, error) {

	objectID, err := primitive.ObjectIDFromHex(groupID)
	if err != nil {
		return nil, 0, err
	}

	groupDetail, err := s.groupService.GetGroupDetail(ctx, groupID)
	if err != nil {
		return nil, 0, err
	}

	messages, totalItems, err := s.messagesRepository.GetMessagesByGroupID(ctx, objectID, from, to, pagination)
	if err != nil {
		return nil, 0, err
	}

	var enrichedMessages []*models.MessageWithUser
	userCache := make(map[string]*models.UserInfor)

	for _, msg := range messages {
		var userInfo *models.UserInfor
		if cached, ok := userCache[msg.SenderID]; ok {
			userInfo = cached
		} else {
			fetchedInfo, err := s.userService.GetUserInfor(ctx, msg.SenderID)
			if err != nil {
<<<<<<< HEAD
				fmt.Printf("(ChatService) 1 - Failed to get user info for %s: %v\n", msg.SenderID, err)
=======
>>>>>>> 48320f7136fdd52eb650afbd1496544fb0d656f7
				fetchedInfo = &models.UserInfor{}
			}
			userInfo = &models.UserInfor{
				UserID:   fetchedInfo.UserID,
				UserName: fetchedInfo.UserName,
				Avartar:  fetchedInfo.Avartar,
			}
			userCache[msg.SenderID] = userInfo
		}

		reacts, err := s.messagesReactRepository.GetMessageReact(ctx, msg.ID, objectID)
		if err != nil {
			return nil, 0, err
		}
<<<<<<< HEAD

		for _, react := range reacts {
			for i := range react.UserReact {
				userID := react.UserReact[i].UserID

				if cached, ok := userCache[userID]; ok {
					react.UserReact[i].UserInfor = cached
				} else {
					fetchedInfo, err := s.userService.GetUserInfor(ctx, userID)
					if err != nil {
						fmt.Printf("(ChatService) 2 - Failed to get user info for %s: %v\n", userID, err)
						fetchedInfo = &models.UserInfor{}
=======
		isUnread := false
		if msg.SenderID == *userID {
			isUnread = true
		} else {
			for _, react := range reacts {
				for _, userReact := range react.UserReact {
					if userReact.UserID == *userID {
						isUnread = true
						break
>>>>>>> 48320f7136fdd52eb650afbd1496544fb0d656f7
					}
				}
				if !isUnread {
					break
				}
			}
		}

		if len(reacts) == 0 {
			reacts = []*models.MessageReact{}
		} else {
			for _, react := range reacts {
				react.UserID = msg.SenderID
				for i := range react.UserReact {
					userID := react.UserReact[i].UserID
					if cached, ok := userCache[userID]; ok {
						react.UserReact[i].UserInfor = cached
					} else {
						fetchedInfo, err := s.userService.GetUserInfor(ctx, userID)
						if err != nil {
							fetchedInfo = &models.UserInfor{}
						}
						react.UserReact[i].UserInfor = &models.UserInfor{
							UserID:   fetchedInfo.UserID,
							UserName: fetchedInfo.UserName,
							Avartar:  fetchedInfo.Avartar,
						}
						userCache[userID] = react.UserReact[i].UserInfor
					}
				}
			}
		}

		reactedUserIDs := make(map[string]bool)
		for _, react := range reacts {
			for _, userReact := range react.UserReact {
				reactedUserIDs[userReact.UserID] = true
			}
		}

		var currentNotReacted []*models.UserInfor
		for _, member := range groupDetail.Members {
			uid := member.GroupMember.UserID
			if uid == msg.SenderID {
				continue
			}
			if !reactedUserIDs[uid] {
				var userInfo *models.UserInfor
				if cached, ok := userCache[uid]; ok {
					userInfo = cached
				} else {
					fetchedInfo, err := s.userService.GetUserInfor(ctx, uid)
					if err != nil {
						fetchedInfo = &models.UserInfor{}
					}
					userInfo = &models.UserInfor{
						UserID:   fetchedInfo.UserID,
						UserName: fetchedInfo.UserName,
						Avartar:  fetchedInfo.Avartar,
					}
					userCache[uid] = userInfo
				}

				currentNotReacted = append(currentNotReacted, userInfo)
			}
		}

		isLimitTimeReact := false
		if groupDetail.Group.LimitTimeReact > 0 {
			expireTime := msg.CreatedAt.Add(time.Duration(groupDetail.Group.LimitTimeReact) * time.Minute)
			
			if time.Now().After(expireTime) {
				isLimitTimeReact = true
			}
		}

		enrichedMessages = append(enrichedMessages, &models.MessageWithUser{
			ID:                msg.ID.Hex(),
			SenderID:          msg.SenderID,
			Content:           msg.Content,
			IsEdit:            msg.IsEdit,
			IsLimitTimeReact:  isLimitTimeReact,
			IsDelete:          msg.IsDelete,
			ContenType:        msg.ContenType,
			ImageKey:          msg.ImageKey,
			CreatedAt:         msg.CreatedAt,
			SenderInfor:       userInfo,
			Reacts:            reacts,
			NotReactedMembers: currentNotReacted,
			IsUnread:          isUnread,
		})
	}

	return enrichedMessages, totalItems, nil
}

func (s *chatService) IsUserInGroup(ctx context.Context, userID string, groupID string) (bool, error) {

	objectID, err := primitive.ObjectIDFromHex(groupID)
	if err != nil {
		return false, err
	}

	return s.messagesRepository.IsUserInGroup(ctx, userID, objectID)

}

func (s *chatService) DownloadGroupMessages(ctx *gin.Context, groupID string, from *time.Time, to *time.Time) error {
	token, ok := ctx.Get(constants.Token)
	if !ok {
		return errors.New("need to be login")
	}

	c := context.WithValue(ctx, constants.TokenKey, token)

	_, err := s.groupService.GetGroupDetail(c, groupID)
	if err != nil {
		return err
	}

<<<<<<< HEAD
	messages, _, err := s.GetGroupMessages(c, groupID, from, to, nil)
=======
	messages, _, err := s.GetGroupMessages(c, groupID, nil, from, to, nil)
>>>>>>> 48320f7136fdd52eb650afbd1496544fb0d656f7
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

	messages, _, err := s.messagesRepository.GetMessagesByGroupID(ctx, objectID, nil, nil, nil)
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
	err := s.messagesRepository.DeleteMessage(ctx, messageID)
	if err != nil {
		return err
	}

	message, err := s.messagesRepository.MessageDetail(ctx, messageID)
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

func (s *chatService) DeleteMessageReacts(ctx context.Context, messageID, groupID string) error {

	objectID, err := primitive.ObjectIDFromHex(messageID)
	if err != nil {
		return err
	}

	objectIDGroup, err := primitive.ObjectIDFromHex(groupID)
	if err != nil {
		return err
	}

	err = s.messagesReactRepository.DeleteMessageReacts(ctx, objectID, objectIDGroup)
	if err != nil {
		return err
	}

	return nil
}

func (s *chatService) GetMessageByID(ctx context.Context, messageID string) (*models.Message, error) {
	objectID, err := primitive.ObjectIDFromHex(messageID)
	if err != nil {
		return nil, err
	}
	return s.messagesRepository.GetMessageByID(ctx, objectID)
}

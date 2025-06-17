package socket

import (
	"chat-service/internal/models"
	"context"
	"sync"
	"time"
)

type Message struct {
	ID         string      `json:"id"`
	Type       string      `json:"type"`
	Token      string      `json:"token"`
	MessageID  string      `json:"message_id"`
	IsEdit     bool        `json:"is_edit"`
	IsDelete   bool        `json:"is_delete"`
	GroupID    string      `json:"group_id"`
	ReaderID   string      `json:"reader_id"`
	SenderID   string      `json:"sender_id"`
	SenderInfo *UserInfo   `json:"sender_infor,omitempty"`
	Content    string      `json:"content"`
	ReactType  string      `json:"react_type"`
	ContenType string      `json:"content_type"`
	ImageKey   string      `json:"image_key,omitempty"`
	VoteData   *VoteData   `json:"vote_data,omitempty"`
	VoteAction *VoteAction `json:"vote_action,omitempty"`
	Timestamp  string      `json:"created_at"`
}

type VoteData struct {
	Question string   `json:"question"`
	Options  []string `json:"options"`
	VoteType string   `json:"vote_type"`
	EndTime  string   `json:"end_time"`
	IsActive bool     `json:"is_active"`
}

type VoteAction struct {
	VoteID      string `json:"vote_id"`
	OptionIndex int    `json:"option_index"`
	Action      string `json:"action"`
}

type OnlineUsersUpdate struct {
	Type        string              `json:"type"`
	GroupID     string              `json:"group_id"`
	OnlineCount int                 `json:"online_count"`
	OnlineUsers []*models.UserInfor `json:"online_users"`
}

type UserInfo struct {
	UserID    string    `json:"user_id"`
	Username  string    `json:"user_name"`
	AvatarURL string    `json:"avatar_url"`
	LastFetch time.Time `json:"-"`
}

type WorkerPool struct {
	workers  int
	taskChan chan func()
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}
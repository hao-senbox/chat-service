package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Message struct {
	ID         primitive.ObjectID  `bson:"_id,omitempty" json:"id,omitempty"`
	GroupID    primitive.ObjectID  `bson:"group_id" json:"group_id"`
	VoteID     *primitive.ObjectID `bson:"vote_id" json:"vote_id"`
	SenderID   string              `bson:"sender_id" json:"sender_id"`
	Content    string              `bson:"content" json:"content"`
	ContenType string              `bson:"content_type" json:"content_type"`
	ImageKey   string              `bson:"image_key" json:"image_key,omitempty"`
	IsEdit     bool                `bson:"is_edit" json:"is_edit"`
	IsDelete   bool                `bson:"is_delete" json:"is_delete"`
	CreatedAt  time.Time           `bson:"created_at" json:"created_at"`
	UpdateAt   time.Time           `bson:"update_at" json:"update_at"`
}

type MessageWithUser struct {
	ID                string          `json:"id"`
	SenderID          string          `json:"sender_id"`
	Content           string          `json:"content"`
	IsEdit            bool            `json:"is_edit"`
	IsLimitTimeReact  bool            `json:"is_limit_time_react"`
	ContenType        string          `json:"content_type"`
	ImageKey          string          `json:"image_key,omitempty"`
	IsDelete          bool            `json:"is_delete"`
	CreatedAt         time.Time       `json:"created_at"`
	SenderInfor       *UserInfor      `json:"sender_infor"`
	NotReactedMembers []*UserInfor    `json:"not_reacted_members"`
	Reacts            []*MessageReact `json:"reacts"`
	IsUnread          bool            `json:"is_reacted"`
}

type UserInfor struct {
	UserID     string     `json:"user_id,omitempty"`
	UserName   string     `json:"user_name,omitempty"`
	FullName   string     `json:"full_name,omitempty"`
	Role       string     `json:"role,omitempty"`
	Avartar    string     `json:"avatar,omitempty"`
	LastOnline *time.Time `json:"last_online,omitempty"`
}

type ReadReceipt struct {
	UserID string    `bson:"user_id" json:"user_id"`
	ReadAt time.Time `bson:"read_at" json:"read_at"`
}

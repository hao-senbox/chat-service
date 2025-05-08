package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Message struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	GroupID     primitive.ObjectID `bson:"group_id" json:"group_id"`
	SenderID    string             `bson:"sender_id" json:"sender_id"`
	Content     string             `bson:"content" json:"content"`
	ContenType  string             `bson:"content_type" json:"content_type"`
	Attachments []Attachment       `bson:"attachments" json:"attachments"`
	IsEdit      bool               `bson:"is_edit" json:"is_edit"`
	IsDelete    bool               `bson:"is_delete" json:"is_delete"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
	UpdateAt    time.Time          `bson:"update_at" json:"update_at"`
}

type MessageWithUser struct {
	ID          string     `json:"id"`
	SenderID    string     `json:"sender_id"`
	Content     string     `json:"content"`
	IsEdit      bool       `json:"is_edit"`
	IsDelete    bool       `json:"is_delete"`
	CreatedAt   time.Time  `json:"created_at"`
	SenderInfor *UserInfor `json:"sender_infor"`
}

type UserInfor struct {
	UserID     string     `json:"user_id"`
	UserName   string     `json:"user_name"`
	FullName   string     `json:"full_name"`
	Role       string     `json:"role"`
	Avartar    string     `json:"avatar"`
	LastOnline *time.Time `json:"last_online"`
}

type Attachment struct {
	URL        string    `bson:"url" json:"url"`
	FileType   string    `bson:"file_type" json:"file_type"`
	FileSize   int64     `bson:"file_size" json:"file_size"`
	UploadedAt time.Time `bson:"uploaded_at" json:"uploaded_at"`
}

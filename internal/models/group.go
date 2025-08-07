package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Group struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Name           string             `bson:"name" json:"name"`
	Description    string             `bson:"description" json:"description"`
	CreatedBy      string             `bson:"created_by" json:"created_by"`
	Type           string             `bson:"type" json:"type"`
	CreatedAt      time.Time          `bson:"created_at" json:"created_at"`
	UpdateAt       time.Time          `bson:"update_at" json:"update_at"`
	MemberCount    int64              `bson:"member_count" json:"member_count"`
	UnreadCount    int                `bson:"unread_count" json:"unread_count"`
	LimitTimeReact int64              `bson:"limit_time_react" json:"limit_time_react"`
	GroupQr        string             `bson:"group_qr" json:"group_qr"`
}

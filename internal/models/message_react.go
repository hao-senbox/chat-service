package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type MessageReact struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	MessageID  primitive.ObjectID `bson:"message_id" json:"message_id"`
	GroupID    primitive.ObjectID `bson:"group_id" json:"group_id"`
	UserID     string             `bson:"user_id" json:"user_id"`
	React      string             `bson:"react" json:"react"`
	TotalReact int64              `bson:"total_react" json:"total_react"`
	CreatedAt  time.Time          `bson:"created_at" json:"created_at"`
	UpdateAt   time.Time          `bson:"update_at" json:"update_at"`
}

type MessageReactResponse struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	MessageID  string             `bson:"message_id" json:"message_id"`
	GroupID    string             `bson:"group_id" json:"group_id"`
	UserID     string             `bson:"user_id" json:"user_id"`
	React      string             `bson:"react" json:"react"`
	TotalReact int64              `bson:"total_react" json:"total_react"`
	CreatedAt  time.Time          `bson:"created_at" json:"created_at"`
	UpdateAt   time.Time          `bson:"update_at" json:"update_at"`
	UserInfor  *UserInfor         `bson:"user_infor" json:"user_infor"`
}

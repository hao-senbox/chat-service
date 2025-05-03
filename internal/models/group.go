package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Group struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	Name        string             `bson:"name" json:"name"`
	Description string             `bson:"description" json:"description"`
	CreatedBy   string             `bson:"created_by" json:"created_by"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
	UpdateAt    time.Time          `bson:"update_at" json:"update_at"`
	MemberCount int64              `bson:"member_count" json:"member_count"`
	QrCode      string             `bson:"qr_code" json:"qr_code"`
}

package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type GroupMember struct {
	ID         primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	GroupID    primitive.ObjectID `bson:"group_id" json:"group_id"`
	UserID     string             `bson:"user_id" json:"user_id"`
	UserInfor  *UserInfor         `bson:"user_infor" json:"user_infor"`
	Permission Permission         `bson:"permission" json:"permission"`
	CreatedAt  time.Time          `bson:"created_at" json:"created_at"`
	UpdateAt   time.Time          `bson:"update_at" json:"update_at"`
}

type Permission struct {
	CanRead            bool `bson:"can_read" json:"can_read"`
	CanWrite           bool `bson:"can_write" json:"can_write"`
	CanEdit            bool `bson:"can_edit" json:"can_edit"`
	CanSendImages      bool `bson:"can_send_images" json:"can_send_images"`
	CanUseCameraDevice bool `bson:"can_use_camera_device" json:"can_use_camera_device"`
}
	
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type APIResponse struct {
	StatusCode int         `json:"status_code"`
	Message    string      `json:"message,omitempty"`
	Data       interface{} `json:"data,omitempty"`
	Error      string      `json:"error,omitempty"`
	ErrorCode  string      `json:"error_code,omitempty"`
}

type GroupMemberResponse struct {
	ID          primitive.ObjectID     `bson:"_id,omitempty" json:"id,omitempty"`
	GroupID     primitive.ObjectID     `bson:"group_id" json:"group_id"`
	UserID      string                 `bson:"user_id" json:"user_id"`
	ReactOfUser []*ReactTypeCountOfUser `bson:"react_of_user" json:"react_of_user"`
	UserInfor   *UserInfor             `bson:"user_infor" json:"user_infor"`
	Permission  Permission             `bson:"permission" json:"permission"`
	CreatedAt   time.Time              `bson:"created_at" json:"created_at"`
	UpdateAt    time.Time              `bson:"update_at" json:"update_at"`
}

type ReactTypeCountOfUser struct {
	ReactType string `json:"react_type" bson:"react_type"`
	Count     int64  `json:"count" bson:"count"`
}

type GroupWithMembers struct {
	Group   Group                      `json:"group"`
	Members []GroupMemberWithUserInfor `json:"members"`
}

type GroupMemberWithUserInfor struct {
	GroupMember *GroupMemberResponse `json:"group_member"`
}

type KeywordOfAllGroups struct {
	Quantity     int      `json:"quantity"`
	Groups       Group    `json:"groups"`
	ArrIdMessage []string `json:"arr_id_message"`
}

type QrCodeData struct {
	GroupID    primitive.ObjectID `json:"group_id"`
	Permission Permission         `json:"permission"`
	ExpiryTime time.Time          `json:"expiry_time"`
}

type PaniginationResponse struct {
	TotalItems int64       `json:"total"`
	TotalPages int         `json:"total_pages"`
	Limit      int         `json:"limit"`
	Page       int         `json:"page"`
	Data       interface{} `json:"data"`
}

const (
	ErrInvalidOperation = "ERR_INVALID_OPERATION"
	ErrInvalidRequest   = "ERR_INVALID_REQUEST"
)

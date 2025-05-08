package models

import "time"

type UserOnline struct {
	UserID     string    `bson:"user_id" json:"user_id"`
	LastOnline time.Time `bson:"last_online" json:"last_online"`
}

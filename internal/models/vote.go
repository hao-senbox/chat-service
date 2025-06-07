package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Vote struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id,omitempty"`
	GroupID   primitive.ObjectID `bson:"group_id" json:"group_id"`
	CreatedBy string             `bson:"created_by" json:"created_by"`
	Question  string             `bson:"question" json:"question"`
	Options   []VoteOption       `bson:"options" json:"options"`
	VoteType  string             `bson:"vote_type" json:"vote_type"`
	IsActive  bool               `bson:"is_active" json:"is_active"`
	EndTime   time.Time          `bson:"end_time" json:"end_time"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdateAt  time.Time          `bson:"update_at" json:"update_at"`
}

type VoteOption struct {
	Text      string   `bson:"text" json:"text"`
	VoteBy    []string `bson:"vote_by" json:"vote_by"`
	VoteCount int64    `bson:"vote_count" json:"vote_count"`
}

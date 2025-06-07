package repository

import (
	"chat-service/internal/models"
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type VoteRepository interface {
	InsertVote(ctx context.Context, vote *models.Vote) (primitive.ObjectID, error)
}

type voteRepository struct {
	collection *mongo.Collection
}

func NewVoteRepository(collection *mongo.Collection) VoteRepository {
	return &voteRepository{
		collection: collection,
	}
}

func (s *voteRepository) InsertVote(ctx context.Context, vote *models.Vote) (primitive.ObjectID, error) {
	
	res, err := s.collection.InsertOne(ctx, vote)
	if err != nil {
		return primitive.NilObjectID, err
	}

	insertedID, ok := res.InsertedID.(primitive.ObjectID)
	if !ok {
		return primitive.NilObjectID, err
	}

	return  insertedID, err
}
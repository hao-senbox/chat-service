package repository

import (
	"chat-service/internal/models"
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type UserOnlineRepository interface {
	SaveUserOnline(ctx context.Context, userOnline *models.UserOnline) error
	GetUserOnline(ctx context.Context, userID string) (*models.UserOnline, error)
}

type userOnlineRepository struct {
	collection *mongo.Collection
}

func NewUserOnlineRepository(collection *mongo.Collection) UserOnlineRepository {
	return &userOnlineRepository{
		collection: collection,
	}
}

func (r * userOnlineRepository) GetUserOnline(ctx context.Context, userID string) (*models.UserOnline, error) {

	var userOnline models.UserOnline

	filter := bson.M{"user_id": userID}
	err := r.collection.FindOne(ctx, filter).Decode(&userOnline)
	if err != nil {
		return nil, err
	}
	return &userOnline, nil
	
}

func (r *userOnlineRepository) SaveUserOnline(ctx context.Context, userOnline *models.UserOnline) error {

	filter := bson.M{"user_id": userOnline.UserID}
	check, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return err
	}

	if check > 0 {
		filter := bson.M{"user_id": userOnline.UserID}
		update := bson.M{"$set": bson.M{"last_online": userOnline.LastOnline}}
		_, err := r.collection.UpdateOne(ctx, filter, update)
		if err != nil {
			return err
		}
	} else {
		_, err := r.collection.InsertOne(ctx, userOnline)
		if err != nil {
			return err
		}
	}

	return nil
}

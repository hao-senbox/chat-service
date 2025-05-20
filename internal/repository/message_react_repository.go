package repository

import (
	"chat-service/internal/models"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type MessageReactRepository interface {
	InsertMessageReact(ctx context.Context, reactMessage *models.MessageReact) error
	GetMessageReact(ctx context.Context, messageID primitive.ObjectID, groupID primitive.ObjectID) ([]*models.MessageReact, error)
}

type messageReactRepository struct {
	collection *mongo.Collection
}

func NewMessageReactRepository(collection *mongo.Collection) MessageReactRepository {
	return &messageReactRepository{
		collection: collection,
	}
}

func (r *messageReactRepository) GetMessageReact(ctx context.Context, messageID primitive.ObjectID, groupID primitive.ObjectID) ([]*models.MessageReact, error) {
	
	var messageReacts []*models.MessageReact
	filter := bson.M{"message_id": messageID, "group_id": groupID}
	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	for cursor.Next(ctx) {
		var messageReact models.MessageReact
		if err := cursor.Decode(&messageReact); err != nil {
			return nil, err
		}
		messageReacts = append(messageReacts, &messageReact)
	}
	return messageReacts, nil
}

func (r *messageReactRepository) InsertMessageReact(ctx context.Context, reacMessage *models.MessageReact) error {
	filter := bson.M{
		"message_id": reacMessage.MessageID,
		"group_id":   reacMessage.GroupID,
		"react":      reacMessage.React,
	}

	var existing bson.M
	err := r.collection.FindOne(ctx, filter).Decode(&existing)
	if err == mongo.ErrNoDocuments {
		doc := bson.M{
			"message_id":  reacMessage.MessageID,
			"group_id":    reacMessage.GroupID,
			"react":       reacMessage.React,
			"total_react": 1,
			"user_reacts": []bson.M{
				{
					"user_id": reacMessage.UserID,
					"count":   1,
				},
			},
			"created_at": time.Now(),
		}
		_, err := r.collection.InsertOne(ctx, doc)
		return err
	} else if err != nil {
		return err
	}

	userReacts, ok := existing["user_reacts"].(primitive.A)
	found := false
	if ok {
		for _, item := range userReacts {
			reactItem, _ := item.(bson.M)
			if reactItem["user_id"] == reacMessage.UserID {
				found = true
				break
			}
		}
	}

	if found {
		return fmt.Errorf("user has already reacted")
	} else {
		update := bson.M{
			"$inc": bson.M{"total_react": 1},
			"$push": bson.M{"user_reacts": bson.M{
				"user_id": reacMessage.UserID,
				"count":   1,
			}},
		}
		_, err := r.collection.UpdateOne(ctx, filter, update)
		return err
	}
}

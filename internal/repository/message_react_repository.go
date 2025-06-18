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
	DeleteMessageReacts(ctx context.Context, messageID primitive.ObjectID, groupID primitive.ObjectID) error
	CountMessageUserReacted(ctx context.Context, groupID primitive.ObjectID, userID string) (int, error)
	GetUserReactCountsInGroup(ctx context.Context, userID string, groupID primitive.ObjectID) ([]*models.ReactTypeCountOfUser, error)
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

	filterRemove := bson.M{
		"message_id": reacMessage.MessageID,
		"group_id":   reacMessage.GroupID,
		"user_reacts.user_id": reacMessage.UserID,
	}
	
	cursor, err := r.collection.Find(ctx, filterRemove)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)
	
	for cursor.Next(ctx) {

		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			continue
		}
	
		id := doc["_id"]
	
		_, err := r.collection.UpdateOne(ctx, bson.M{
			"_id": id,
		}, bson.M{
			"$inc": bson.M{"total_react": -1},
			"$pull": bson.M{"user_reacts": bson.M{"user_id": reacMessage.UserID}},
		})
		if err != nil {
			return err
		}
	
		var updatedDoc bson.M
		if err := r.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&updatedDoc); err != nil {
			return err
		}
	
		if updatedDoc["total_react"].(int32) == 0 {
			_, err := r.collection.DeleteOne(ctx, bson.M{"_id": id})
			if err != nil {
				return err
			}
		}
	}
	

	filter := bson.M{
		"message_id": reacMessage.MessageID,
		"group_id":   reacMessage.GroupID,
		"react":      reacMessage.React,
	}

	var existing bson.M
	err = r.collection.FindOne(ctx, filter).Decode(&existing)
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

	update := bson.M{
		"$inc": bson.M{"total_react": 1},
		"$push": bson.M{"user_reacts": bson.M{
			"user_id": reacMessage.UserID,
			"count":   1,
		}},
	}
	_, err = r.collection.UpdateOne(ctx, filter, update)
	return err
}

func (r *messageReactRepository) DeleteMessageReacts(ctx context.Context, messageID primitive.ObjectID, groupID primitive.ObjectID) error {

	filter := bson.M{"message_id": messageID, "group_id": groupID}
	
	_, err := r.collection.DeleteMany(ctx, filter)
	if err != nil {
		return err
	}

	return nil
}

func (r *messageReactRepository) CountMessageUserReacted(ctx context.Context, groupID primitive.ObjectID, userID string) (int, error) {

	filter := bson.M{
		"group_id": groupID,
		"user_reacts.user_id": userID,
	}

	count, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return 0, err
	}

	return int(count), nil
	
}

func (r *messageReactRepository) GetUserReactCountsInGroup(ctx context.Context, userID string, groupID primitive.ObjectID) ([]*models.ReactTypeCountOfUser, error) {
	pipeline := mongo.Pipeline{
		// Match reactions trong group và của user cụ thể
		{
			{Key: "$match", Value: bson.M{
				"group_id": groupID,
				"user_reacts.user_id": userID,
			}},
		},
		// Unwind user_reacts array để xử lý từng react
		{
			{Key: "$unwind", Value: "$user_reacts"},
		},
		// Filter chỉ lấy react của user cụ thể
		{
			{Key: "$match", Value: bson.M{
				"user_reacts.user_id": userID,
			}},
		},
		// Group theo react type và sum count
		{
			{Key: "$group", Value: bson.M{
				"_id": "$react",
				"total_count": bson.M{"$sum": "$user_reacts.count"},
			}},
		},
		// Project để format output
		{
			{Key: "$project", Value: bson.M{
				"react_type": "$_id",
				"count": "$total_count",
				"_id": 0,
			}},
		},
	}

	cursor, err := r.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate user react counts: %w", err)
	}
	defer cursor.Close(ctx)

	var results []*models.ReactTypeCountOfUser
	if err := cursor.All(ctx, &results); err != nil {
		return nil, fmt.Errorf("failed to decode react counts: %w", err)
	}

	return results, nil
}
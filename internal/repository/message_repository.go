package repository

import (
	"chat-service/internal/models"
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MessagesRepository interface {
	SaveMessage(ctx context.Context, message *models.Message) (primitive.ObjectID, error)
	EditMessage(ctx context.Context, message *models.EditMessage) error
	IsUserInGroup(ctx context.Context, userID string, groupID primitive.ObjectID) (bool, error)
	GetMessagesByGroupID(ctx context.Context, groupID primitive.ObjectID, from *time.Time, to *time.Time, pagination *models.Panigination) ([]*models.Message, int64, error)
    DeleteMessage(ctx context.Context, messageID primitive.ObjectID) error
    CountKeywordMessage(ctx context.Context, keyword string, groupID primitive.ObjectID) (int, []string, error)
    MessageDetail(ctx context.Context, messageID primitive.ObjectID) (*models.Message, error)
	CountNonUserMessage(ctx context.Context, groupID primitive.ObjectID, userID string) (int, error)
	GetMessageByID(ctx context.Context, messageID primitive.ObjectID) (*models.Message, error)
	GetCountMessageGroup(ctx context.Context, groupID primitive.ObjectID) (int, error)
}

type messagesRepository struct {
	collection            *mongo.Collection
	collectionMemberGroup *mongo.Collection
}

func NewChatRepository(collection, collectionMemberGroup *mongo.Collection) MessagesRepository {
	return &messagesRepository{
		collection:            collection,
		collectionMemberGroup: collectionMemberGroup,
	}
}

func (r *messagesRepository) IsUserInGroup(ctx context.Context, userID string, groupID primitive.ObjectID) (bool, error) {

	filter := bson.M{"user_id": userID, "group_id": groupID}

	count, err := r.collectionMemberGroup.CountDocuments(ctx, filter)
	if err != nil {
		return false, err
	}

	return count > 0, err
}

func (r *messagesRepository) DeleteMessage(ctx context.Context, messageID primitive.ObjectID) error {

    filter := bson.M{"_id": messageID}
    update := bson.M{"$set": bson.M{
        "is_delete": true,
    }}

    _, err := r.collection.UpdateOne(ctx, filter, update)
    if err != nil {
        return err
    }

    return nil
}

func (r *messagesRepository) SaveMessage(ctx context.Context, message *models.Message) (primitive.ObjectID, error) {

	res, err := r.collection.InsertOne(ctx, message)
	if err != nil {
		return primitive.NilObjectID, err
	}
	return res.InsertedID.(primitive.ObjectID), nil

}

func (r *messagesRepository) EditMessage(ctx context.Context, message *models.EditMessage) error {

	objectID, err := primitive.ObjectIDFromHex(message.ID)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": objectID}
	update := bson.M{"$set": bson.M{
		"content":    message.Content,
		"updated_at": message.UpdateAt,
		"is_edit":    true,
	}}

	_, err = r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	return nil
}

func (r *messagesRepository) GetMessagesByGroupID(ctx context.Context, groupID primitive.ObjectID, from *time.Time, to *time.Time, pagination *models.Panigination) ([]*models.Message, int64, error) {

	filter := bson.M{
		"group_id": groupID,
	}

	if from != nil || to != nil {
		timeFilter := bson.M{}
		if from != nil {
			timeFilter["$gte"] = *from
		}
		if to != nil {
			timeFilter["$lte"] = *to
		}
		filter["created_at"] = timeFilter
	}

	totalItems, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	skip := (pagination.Page - 1) * pagination.Limit
	opts := options.Find().
		SetSkip(int64(skip)).
		SetLimit(int64(pagination.Limit)).
		SetSort(bson.D{{Key: "created_at", Value: -1}})

	cur, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close(ctx)

	var messages []*models.Message
	if err := cur.All(ctx, &messages); err != nil {
		return nil, 0, err
	}

	return messages, totalItems, nil
}


func (r *messagesRepository) CountKeywordMessage(ctx context.Context, keyword string, groupID primitive.ObjectID) (int, []string, error) {

    filter := bson.M{
        "content": primitive.Regex{
            Pattern: fmt.Sprintf(".*%s.*", keyword),
            Options: "i",
        },
        "group_id": groupID,
        "is_delete": false,
    }

    cursor, err := r.collection.Find(ctx, filter)
    if err != nil {
        return 0, nil, err
    }
    defer cursor.Close(ctx)

    var messages []*models.Message
    if err := cursor.All(ctx, &messages); err != nil {
        return 0, nil, err
    }

    messagesID := make([]string, 0, len(messages))
    for _, msg := range messages {
        messagesID = append(messagesID, msg.ID.Hex())
    }

    return len(messages), messagesID, nil
}

func (r *messagesRepository) MessageDetail(ctx context.Context, messageID primitive.ObjectID) (*models.Message, error) {

    filter := bson.M{"_id": messageID}

    var message models.Message
    err := r.collection.FindOne(ctx, filter).Decode(&message)
    if err != nil {
        return nil, err
    }

    return &message, nil
}

func (r *messagesRepository) CountNonUserMessage(ctx context.Context, groupID primitive.ObjectID, userID string) (int, error) {
	
	filter := bson.M{
		"group_id": groupID,
		"sender_id":  bson.M{"$ne": userID},
	}

	count, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return 0, err
	}

	return int(count), nil

}

func (r *messagesRepository) GetMessageByID(ctx context.Context, messageID primitive.ObjectID) (*models.Message, error) {

	filter := bson.M{"_id": messageID}

	var message models.Message
	err := r.collection.FindOne(ctx, filter).Decode(&message)
	if err != nil {
		return nil, err
	}

	return &message, nil
	
}

func (r *messagesRepository) GetCountMessageGroup(ctx context.Context, groupID primitive.ObjectID) (int, error) {

	filter := bson.M{"group_id": groupID}

	count, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return 0, err
	}

	return int(count), nil
}
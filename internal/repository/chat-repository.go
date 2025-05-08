package repository

import (
	"chat-service/internal/models"
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type ChatRepository interface {
	SaveMessage(ctx context.Context, message *models.Message) (primitive.ObjectID, error)
	EditMessage(ctx context.Context, message *models.EditMessage) error
	IsUserInGroup(ctx context.Context, userID string, groupID primitive.ObjectID) (bool, error)
	GetMessagesByGroupID(ctx context.Context, groupID primitive.ObjectID) ([]*models.Message, error)
	DeleteMessageGroup(ctx context.Context, groupID primitive.ObjectID) error
    DeleteMessage(ctx context.Context, messageID primitive.ObjectID) error
}

type chatRepository struct {
	collection            *mongo.Collection
	collectionMemberGroup *mongo.Collection
}

func NewChatRepository(collection, collectionMemberGroup *mongo.Collection) ChatRepository {
	return &chatRepository{
		collection:            collection,
		collectionMemberGroup: collectionMemberGroup,
	}
}

func (r *chatRepository) IsUserInGroup(ctx context.Context, userID string, groupID primitive.ObjectID) (bool, error) {

	filter := bson.M{"user_id": userID, "group_id": groupID}

	count, err := r.collectionMemberGroup.CountDocuments(ctx, filter)
	if err != nil {
		return false, err
	}

	return count > 0, err
}

func (r *chatRepository) DeleteMessage(ctx context.Context, messageID primitive.ObjectID) error {

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

func (r *chatRepository) SaveMessage(ctx context.Context, message *models.Message) (primitive.ObjectID, error) {

	res, err := r.collection.InsertOne(ctx, message)
	if err != nil {
		return primitive.NilObjectID, err
	}
	return res.InsertedID.(primitive.ObjectID), nil

}

func (r *chatRepository) EditMessage(ctx context.Context, message *models.EditMessage) error {

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

func (r *chatRepository) GetMessagesByGroupID(ctx context.Context, groupID primitive.ObjectID) ([]*models.Message, error) {

	filter := bson.M{"group_id": groupID}

	cur, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var messages []*models.Message
	if err := cur.All(ctx, &messages); err != nil {
		return nil, err
	}

	return messages, nil
}

func (r *chatRepository) DeleteMessageGroup(ctx context.Context, groupID primitive.ObjectID) error {

	filter := bson.M{"group_id": groupID}

	_, err := r.collection.DeleteMany(ctx, filter)
	if err != nil {
		return err
	}

	return nil
}

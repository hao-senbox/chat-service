package repository

import (
	"chat-service/internal/models"
	"context"
	"fmt"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type GroupRepository interface {
	GetUserGroups(ctx context.Context, userID string) ([]*models.Group, error)
	CreateGroup(ctx context.Context, group *models.Group) error
	GetGroupDetail(ctx context.Context, groupID primitive.ObjectID) (*models.Group, error)
	IncrementMemberCount(ctx context.Context, groupID primitive.ObjectID) error
}

type GroupMemberRepository interface {
	CreateGroupUser(ctx context.Context, group *models.GroupMember) error
}

type groupRepository struct {
	collection *mongo.Collection
	groupMemberCollection *mongo.Collection

}

type groupMemberRepository struct {
	collection *mongo.Collection
	groupRepository GroupRepository
}

func NewGroupRepository(collection, groupMemberRepository *mongo.Collection) GroupRepository {
	return &groupRepository {
		collection: collection,
		groupMemberCollection: groupMemberRepository,
	}
}

func NewGroupMemberRepository(collection *mongo.Collection, groupRepository GroupRepository) GroupMemberRepository {
	return &groupMemberRepository {
		collection: collection,
		groupRepository: groupRepository,
	}
}

func (r *groupRepository) CreateGroup(ctx context.Context, group *models.Group) error {

	_, err := r.collection.InsertOne(ctx, group)
	if err != nil {
		return err
	}

	return nil
}

func (r *groupRepository) GetGroupDetail(ctx context.Context, groupID primitive.ObjectID) (*models.Group, error) {

	var group models.Group

	filter := bson.M{"_id": groupID}

	err := r.collection.FindOne(ctx, filter).Decode(&group)

	if err != nil {
		return nil, err
	}

	return &group, nil
}

func (r *groupMemberRepository) CreateGroupUser(ctx context.Context, group *models.GroupMember) error {

	_, err := r.collection.InsertOne(ctx, group)
	if err != nil {
		return err
	}

	if err:= r.groupRepository.IncrementMemberCount(ctx, group.GroupID); err != nil {
		return err
	}
	
	return nil
}

func (r *groupRepository) IncrementMemberCount(ctx context.Context, groupID primitive.ObjectID) error {

	filter := bson.M{"_id": groupID}
	update := bson.M{"$inc": bson.M{"member_count": 1}}

	_, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	return nil
}

func (r *groupRepository) GetUserGroups(ctx context.Context, userID string) ([]*models.Group, error) {
	
	filter := bson.M{"user_id": userID}

	cursor, err := r.groupMemberCollection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var groupMembers[]models.GroupMember
	if err := cursor.All(ctx, &groupMembers); err != nil {
        return nil, fmt.Errorf("failed to decode memberships: %w", err)
    }

	if len(groupMembers) == 0 {
        return []*models.Group{}, nil
    }

	var groupIDs []primitive.ObjectID
	for _, groupMember := range groupMembers {
		groupIDs = append(groupIDs, groupMember.GroupID)
	}

	groupFilter := bson.M{"_id": bson.M{"$in": groupIDs}}
    groupCursor, err := r.collection.Find(ctx, groupFilter)
    if err != nil {
        return nil, fmt.Errorf("failed to find groups: %w", err)
    }
    defer groupCursor.Close(ctx)
    
    var groups []*models.Group
    if err := groupCursor.All(ctx, &groups); err != nil {
        return nil, fmt.Errorf("failed to decode groups: %w", err)
    }
    
    return groups, nil

}
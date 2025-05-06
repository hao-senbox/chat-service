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
	GetAllGroups(ctx context.Context) ([]*models.Group, error)
	IncrementMemberCount(ctx context.Context, groupID primitive.ObjectID) error
	DecrementMemberCount(ctx context.Context, groupID primitive.ObjectID) error
	UpdateGroup(ctx context.Context, groupID primitive.ObjectID, group *models.GroupRequest) error
	DeleteGroup(ctx context.Context, groupID primitive.ObjectID) error
	SetGroupMemberRepo (repo GroupMemberRepository)
	
}

type GroupMemberRepository interface {
	AddUserToGroup(ctx context.Context, group *models.GroupMember) error
	GetGroupMembers(ctx context.Context, groupID primitive.ObjectID) ([]*models.GroupMember, error)
	GetgroupMemberDetail(ctx context.Context, userId string) (*models.GroupMember, error)
	DeleteUserFromGroup(ctx context.Context, groupID primitive.ObjectID, userID string) error
}

type groupRepository struct {
	collection            *mongo.Collection
	groupMemberCollection *mongo.Collection
	groupMemberRepo       GroupMemberRepository
}

type groupMemberRepository struct {
	collection      *mongo.Collection
	groupRepository GroupRepository
}

func NewGroupRepository(collection, groupMemberCollection *mongo.Collection, groupMemberRepo GroupMemberRepository) GroupRepository {
	return &groupRepository{
		collection:            collection,
		groupMemberCollection: groupMemberCollection,
		groupMemberRepo:       groupMemberRepo,
	}
}

func NewGroupMemberRepository(collection *mongo.Collection, groupRepository GroupRepository) GroupMemberRepository {
	return &groupMemberRepository{
		collection:      collection,
		groupRepository: groupRepository,
	}
}

func (r *groupRepository) SetGroupMemberRepo (repo GroupMemberRepository) {
	r.groupMemberRepo = repo
}

func (r *groupMemberRepository) GetGroupMembers(ctx context.Context, groupID primitive.ObjectID) ([]*models.GroupMember, error) {

	filter := bson.M{"group_id": groupID}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var groupMembers []*models.GroupMember

	if err := cursor.All(ctx, &groupMembers); err != nil {
		return nil, fmt.Errorf("failed to decode group members: %w", err)
	}

	return groupMembers, nil
}

func (r *groupRepository) GetAllGroups(ctx context.Context) ([]*models.Group, error) {

	cursor, err := r.collection.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var groups []*models.Group
	if err := cursor.All(ctx, &groups); err != nil {
		return nil, fmt.Errorf("failed to decode groups: %w", err)
	}
	return groups, nil
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

func (r *groupMemberRepository) AddUserToGroup(ctx context.Context, group *models.GroupMember) error {

	_, err := r.collection.InsertOne(ctx, group)
	if err != nil {
		return err
	}

	if err := r.groupRepository.IncrementMemberCount(ctx, group.GroupID); err != nil {
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

func (r *groupRepository) DecrementMemberCount(ctx context.Context, groupID primitive.ObjectID) error {

	filter := bson.M{"_id": groupID}
	update := bson.M{"$inc": bson.M{"member_count": -1}}

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

	var groupMembers []models.GroupMember
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

func (r *groupRepository) UpdateGroup(ctx context.Context, groupID primitive.ObjectID, group *models.GroupRequest) error {

	filter := bson.M{"_id": groupID}
	update := bson.M{"$set": bson.M{
		"name": group.Name, "description": group.Description,
	}}

	_, err := r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	return nil
}

func (r *groupMemberRepository) GetgroupMemberDetail(ctx context.Context, userID string) (*models.GroupMember, error) {

	var groupMember models.GroupMember

	filter := bson.M{"user_id": userID}

	err := r.collection.FindOne(ctx, filter).Decode(&groupMember)
	if err != nil {
		return nil, err
	}

	return &groupMember, nil
}

func (r *groupRepository) DeleteGroup(ctx context.Context, groupID primitive.ObjectID) error {

	filter := bson.M{"_id": groupID}

	_, err := r.collection.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}

	groupUsers, err := r.groupMemberRepo.GetGroupMembers(ctx, groupID)
	if err != nil {
		return err
	}

	for _, groupUser := range groupUsers {
		err := r.groupMemberRepo.DeleteUserFromGroup(ctx, groupID, groupUser.UserID)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *groupMemberRepository) DeleteUserFromGroup(ctx context.Context, groupID primitive.ObjectID, userID string) error {
	filter := bson.M{"group_id": groupID, "user_id": userID}

	_, err := r.collection.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}

	if err := r.groupRepository.DecrementMemberCount(ctx, groupID); err != nil {
		return err
	}

	return nil
}

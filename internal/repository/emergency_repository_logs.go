package repository

import (
	"chat-service/internal/models"
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type EmergencyLogsRepository interface {
	Create(ctx context.Context, emergencyLogs *models.EmergencyLogs) error
	GetNotificationsUser(ctx context.Context, userID string) ([]*models.EmergencyLogs, error)
}

type emergencyLogsRepository struct {
	collecion *mongo.Collection
}

func NewEmergencyLogsRepository(collection *mongo.Collection) EmergencyLogsRepository {
	return &emergencyLogsRepository{
		collecion: collection,
	}
}

func (e *emergencyLogsRepository) Create(ctx context.Context, emergencyLogs *models.EmergencyLogs) error {
	_, err := e.collecion.InsertOne(ctx, emergencyLogs)
	if err != nil {
		return err
	}
	return nil
}

func (e *emergencyLogsRepository) GetNotificationsUser(ctx context.Context, userID string) ([]*models.EmergencyLogs, error) {
	
	filter := bson.M{"user_id": userID}

	cursor, err := e.collecion.Find(ctx, filter)
	if err != nil {
		return nil, err
	}

	var emergencyLogs []*models.EmergencyLogs
	if err := cursor.All(ctx, &emergencyLogs); err != nil {
		return nil, err
	}

	return emergencyLogs, nil
	
}
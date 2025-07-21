package repository

import (
	"chat-service/internal/models"
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

type EmergencyLogsRepository interface {
	Create(ctx context.Context, emergencyLogs *models.EmergencyLogs) error
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
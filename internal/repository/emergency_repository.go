package repository

import (
	"chat-service/internal/models"
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

type EmergencyRepository interface {
	Create(ctx context.Context, emergency *models.Emergency) error
}

type emergencyRepository struct {
	collecion *mongo.Collection
}

func NewEmergencyRepository(collection *mongo.Collection) EmergencyRepository {
	return &emergencyRepository{
		collecion: collection,
	}
}

func (e *emergencyRepository) Create(ctx context.Context, emergency *models.Emergency) error {
	_, err := e.collecion.InsertOne(ctx, emergency)
	if err != nil {
		return err
	}
	return nil
}
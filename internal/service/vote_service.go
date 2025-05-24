package service

import (
	"chat-service/internal/models"
	"chat-service/internal/repository"
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type VoteService interface {
	InsertVote(ctx context.Context, vote *models.Vote) (primitive.ObjectID, error)
	InsertVoteOption(ctx context.Context, voteOption *models.VoteOption) (primitive.ObjectID, error)
}

type voteService struct {
	voteRepository repository.VoteRepository	
}

func NewVoteService(voteRepository repository.VoteRepository) VoteService {
	return &voteService{
		voteRepository: voteRepository,
	}
}

func (s *voteService) InsertVote(ctx context.Context, vote *models.Vote) (primitive.ObjectID, error) {

	idVote, err := s.voteRepository.InsertVote(ctx, vote)
	if err != nil {
		return primitive.NilObjectID, err
	}

	return idVote, nil

}

func (s *voteService) InsertVoteOption(ctx context.Context, voteOption *models.VoteOption) (primitive.ObjectID, error) {
	return primitive.NilObjectID, nil
}



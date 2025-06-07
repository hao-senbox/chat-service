package api

import "chat-service/internal/service"

type VoteHandler struct {
	voteService service.VoteService
}

func NewVoteHandler(voteService service.VoteService) *VoteHandler {
	return &VoteHandler{
		voteService: voteService,
	}
}

package api

import (
	"chat-service/internal/models"
	"chat-service/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

type GroupHandlers struct {
	groupService service.GroupService
}

func NewGroupService(groupService service.GroupService) *GroupHandlers {
	return &GroupHandlers{
		groupService: groupService,
	}
}

func (h *GroupHandlers) GetAllGroups(c *gin.Context) {

}

func (h *GroupHandlers) CreateGroup(c *gin.Context) {

	var req models.GroupRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		SendError(c, http.StatusBadRequest, err, models.ErrInvalidRequest)
	}

	if err := h.groupService.CreateGroup(c, &req); err != nil {
		SendError(c, http.StatusInternalServerError, err, models.ErrInvalidOperation)
		return
	}

	SendSuccess(c, http.StatusOK, "Group created successfully", nil)
}

func (h *GroupHandlers) CreateGroupUser(c *gin.Context) {

	var req models.GroupUserRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		SendError(c, http.StatusBadRequest, err, models.ErrInvalidRequest)
	}

	if err := h.groupService.CreateGroupUser(c, &req); err != nil {
		SendError(c, http.StatusInternalServerError, err, models.ErrInvalidOperation)
		return
	}

	SendSuccess(c, http.StatusOK, "Add user created successfully", nil)
}

func (h *GroupHandlers) GetUserGroups(c *gin.Context) {

	userID := c.Param("user_id")

	if userID == "" {
		SendError(c, http.StatusBadRequest, nil, models.ErrInvalidRequest)
		return
	}

	groups, err := h.groupService.GetUserGroups(c, userID)
	if err != nil {
		SendError(c, http.StatusInternalServerError, err, models.ErrInvalidOperation)
		return
	}

	SendSuccess(c, http.StatusOK, "Get user groups successfully", groups)
	
}
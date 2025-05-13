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
	
	groups, err := h.groupService.GetAllGroups(c)
	if err != nil {
		SendError(c, http.StatusInternalServerError, err, models.ErrInvalidOperation)
		return
	}

	SendSuccess(c, http.StatusOK, "Get all groups successfully", groups)
}

func (h *GroupHandlers) GetGroupDetail(c *gin.Context) {

	groupID := c.Param("group_id")

	if groupID == "" {
		SendError(c, http.StatusBadRequest, nil, models.ErrInvalidRequest)
		return
	}

	group, err := h.groupService.GetGroupDetail(c, groupID)
	if err != nil {
		SendError(c, http.StatusInternalServerError, err, models.ErrInvalidOperation)
		return
	}

	SendSuccess(c, http.StatusOK, "Get group detail successfully", group)
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

func (h *GroupHandlers) AddUserToGroup(c *gin.Context) {

	var req models.GroupUserRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		SendError(c, http.StatusBadRequest, err, models.ErrInvalidRequest)
	}

	if err := h.groupService.AddUserToGroup(c, &req); err != nil {
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

func (h *GroupHandlers) UpdateGroup(c *gin.Context) {

	var req models.GroupRequest

	groupID := c.Param("group_id")
	if groupID == "" {
		SendError(c, http.StatusBadRequest, nil, models.ErrInvalidRequest)
		return
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		SendError(c, http.StatusBadRequest, err, models.ErrInvalidRequest)
		return
	}

	if err := h.groupService.UpdateGroup(c, groupID, &req); err != nil {
		SendError(c, http.StatusInternalServerError, err, models.ErrInvalidOperation)
		return
	}

	SendSuccess(c, http.StatusOK, "Group updated successfully", nil)
}

func (h* GroupHandlers) DeleteGroup(c *gin.Context) {

	var req models.TokenUserRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		SendError(c, http.StatusBadRequest, err, models.ErrInvalidRequest)
		return
	}

	groupID := c.Param("group_id")
	if groupID == "" {
		SendError(c, http.StatusBadRequest, nil, models.ErrInvalidRequest)
		return
	}

	if err := h.groupService.DeleteGroup(c, groupID, &req); err != nil {
		SendError(c, http.StatusInternalServerError, err, models.ErrInvalidOperation)
		return
	}

	SendSuccess(c, http.StatusOK, "Group deleted successfully", nil)
}

func (h *GroupHandlers) RemoveUserFromGroup(c *gin.Context) {

	groupID := c.Param("group_id")
	var req models.GroupUserRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		SendError(c, http.StatusBadRequest, err, models.ErrInvalidRequest)
	}

	if err := h.groupService.RemoveUserFromGroup(c, groupID, &req); err != nil {
		SendError(c, http.StatusInternalServerError, err, models.ErrInvalidOperation)
		return
	}

	SendSuccess(c, http.StatusOK, "Remove user from group successfully", nil)
}

func (h* GroupHandlers) CountKeywordAllGroups(c *gin.Context) {

	var req models.KeywordRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		SendError(c, http.StatusBadRequest, err, models.ErrInvalidRequest)
	}
	
	count, err := h.groupService.CountKeywordAllGroups(c, req.Keyword)
	if err != nil {
		SendError(c, http.StatusInternalServerError, err, models.ErrInvalidOperation)
		return
	}

	SendSuccess(c, http.StatusOK, "Count keyword all groups successfully", count)
}

func (h *GroupHandlers) GenerateGroupQrCode(c *gin.Context) {
	
	var req models.GroupQrRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		SendError(c, http.StatusBadRequest, err, models.ErrInvalidRequest)
		return
	}

	qrCodeBase64, err := h.groupService.GenerateGroupQrCode(c, &req)
	if err != nil {
		SendError(c, http.StatusInternalServerError, err, models.ErrInvalidOperation)
		return
	}

	SendSuccess(c, http.StatusOK, "Generate group qr code successfully", qrCodeBase64)
}

func (h *GroupHandlers) JoinGroupByQrCode(c *gin.Context) {

	var req models.JoinGroupByQrCodeRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		SendError(c, http.StatusBadRequest, err, models.ErrInvalidRequest)
		return
	}

	if err := h.groupService.JoinGroupByQrCode(c, &req); err != nil {
		SendError(c, http.StatusInternalServerError, err, models.ErrInvalidOperation)
		return
	}

	SendSuccess(c, http.StatusOK, "Join group by qr code successfully", nil)
}
package models

import "time"

type GroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedBy   string `json:"created_by"`
}

type EditMessage struct {
	ID       string `json:"id"`
	Content  string `json:"content"`
	UpdateAt time.Time `json:"update_at"`
}

type GroupUserRequest struct {
	GroupID            string `json:"group_id"`
	UserID             string `json:"user_id"`
	CanRead            bool   `json:"can_read"`
	CanWrite           bool   `json:"can_write"`
	CanEdit            bool   `json:"can_edit"`
	CanSendImages      bool   `json:"can_send_images"`
	CanUseCameraDevice bool   `json:"can_use_camera_device"`
}

package models

import "time"

type GroupRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	CreatedBy   string `json:"created_by"`
}

type EditMessage struct {
	ID       string    `json:"id"`
	Content  string    `json:"content"`
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

type GroupQrRequest struct {
	GroupID            string    `json:"group_id"`
	CanRead            bool      `json:"can_read"`
	CanWrite           bool      `json:"can_write"`
	CanEdit            bool      `json:"can_edit"`
	CanSendImages      bool      `json:"can_send_images"`
	CanUseCameraDevice bool      `json:"can_use_camera_device"`
}

type JoinGroupByQrCodeRequest struct {
	UserID     string `json:"user_id"`
	QrCodeData string `json:"qr_code_data"`
}

type KeywordRequest struct {
	Keyword string `json:"keyword"`
}

type TokenUserRequest struct {
	Token string `json:"token"`
}

type ReactMessageRequest struct {
	MessageID string `json:"message_id"`
	GroupID   string `json:"group_id"`
	UserID    string `json:"user_id"`
	React     string `json:"react"`
}

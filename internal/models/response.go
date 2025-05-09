package models


type APIResponse struct  {
	StatusCode int `json:"status_code"`
	Message string `json:"message,omitempty"`
	Data interface{} `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`
}

type GroupWithMembers struct {
	Group Group `json:"group"`
	Members []GroupMemberWithUserInfor `json:"members"`
}

type GroupMemberWithUserInfor struct {
	GroupMember *GroupMember `json:"group_member"`
}

type KeywordOfAllGroups struct {
	Quantity int `json:"quantity"`
	Groups Group `json:"groups"`
	ArrIdMessage []string `json:"arr_id_message"`
}

const (
    ErrInvalidOperation   = "ERR_INVALID_OPERATION"
    ErrInvalidRequest     = "ERR_INVALID_REQUEST"
)

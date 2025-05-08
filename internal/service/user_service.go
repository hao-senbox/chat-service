package service

import (
	"chat-service/internal/models"
	"chat-service/internal/repository"
	"chat-service/pkg/consul"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/hashicorp/consul/api"
)

type UserService interface {
	GetUserInfor(userID string) (*models.UserInfor, error)
	GetUserOnline (ctx context.Context, userID string) (*models.UserInfor, error)
}

type userService struct {
	userOnlineRepo repository.UserOnlineRepository
	client *callAPI
}

type callAPI struct {
	client       consul.ServiceDiscovery
	clientServer *api.CatalogService
}

var (
	mainService = "go-main-service"
)

func NewUserService(client *api.Client, userOnlineRepo repository.UserOnlineRepository) UserService {
	mainServiceAPI := NewServiceAPI(client, mainService)
	return &userService{
		userOnlineRepo: userOnlineRepo,
		client: mainServiceAPI,
	}
}

func NewServiceAPI(client *api.Client, serviceName string) *callAPI {
	sd, err := consul.NewServiceDiscovery(client, serviceName)
	if err != nil {
		fmt.Printf("Error creating service discovery: %v\n", err)
		return nil
	}

	service, err := sd.DiscoverService()
	if err != nil {
		fmt.Printf("Error discovering service: %v\n", err)
		return nil
	}

	if os.Getenv("LOCAL_TEST") == "true" {
        fmt.Println("Running in LOCAL_TEST mode — overriding service address to localhost")
        service.ServiceAddress = "localhost"
    }

	return &callAPI{
		client:       sd,
		clientServer: service,
	}
}

func (u *userService) GetUserOnline(ctx context.Context, userID string) (*models.UserInfor, error) {

	lastUserOnline, err := u.userOnlineRepo.GetUserOnline(ctx, userID)
	if err != nil {
		return nil, err
	}

	userInfor, err := u.GetUserInfor(userID)
	if err != nil {
		return nil, err
	}

	return &models.UserInfor{
		UserID:     userInfor.UserID,
		UserName:   userInfor.UserName,
		FullName:   userInfor.FullName,
		Avartar:    userInfor.Avartar,
		Role:       userInfor.Role,
		LastOnline: &lastUserOnline.LastOnline,
	}, nil

}

func (u *userService) GetUserInfor(userID string) (*models.UserInfor, error) {
    data := u.client.GetUserInfor(userID)
	
    if data == nil {
        return nil, fmt.Errorf("no user data found for userID: %s", userID)
    }

    innerData, ok := data["data"].(map[string]interface{})
    if !ok {
        return nil, fmt.Errorf("invalid response format: missing 'data' field")
    }

    var roleName string
    rolesRaw, ok := innerData["roles"].([]interface{})
    if ok && len(rolesRaw) > 0 {
        firstRole, ok := rolesRaw[0].(map[string]interface{})
        if ok {
            roleName = safeString(firstRole["role_name"])
        }
    }

    return &models.UserInfor{
        UserID:   safeString(innerData["id"]),
        UserName: safeString(innerData["username"]),
        FullName: safeString(innerData["fullname"]),
        Avartar:  safeString(innerData["avatar"]),
        Role:     roleName,
    }, nil
}


func safeString(val interface{}) string {
    if val == nil {
        return ""
    }
    str, ok := val.(string)
    if !ok {
        return ""
    }
    return str
}


func (c *callAPI) GetUserInfor(userID string) map[string]interface{} {
	endpoint := fmt.Sprintf("/v1/user/%s", userID)
	res, err := c.client.CallAPI(c.clientServer, endpoint, http.MethodGet, nil, nil)
	if err != nil {
		fmt.Printf("Error calling API: %v\n", err)
		return nil
	}

	var userData interface{}
	json.Unmarshal([]byte(res), &userData)
	if userData == nil {
		fmt.Println("User data is nil")
		return nil
	}

	myMap := userData.(map[string]interface{})

	return myMap
}

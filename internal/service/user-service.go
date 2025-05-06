package service

import (
	"chat-service/internal/models"
	"chat-service/pkg/consul"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/hashicorp/consul/api"
)

type UserService interface {
	GetUserInfor(userID string) (*models.UserInfor, error)
}

type userService struct {
	client *callAPI
}

type callAPI struct {
	client       consul.ServiceDiscovery
	clientServer *api.CatalogService
}

var (
	mainService = "go-main-service"
)

func NewUserService(client *api.Client) UserService {
	mainServiceAPI := NewServiceAPI(client, mainService)
	return &userService{
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

func (u *userService) GetUserInfor(userID string) (*models.UserInfor, error) {
    data := u.client.GetUserInfor(userID)
    if data == nil {
        return nil, fmt.Errorf("no user data found for userID: %s", userID)
    }

    innerData, ok := data["data"].(map[string]interface{})
    if !ok {
        return nil, fmt.Errorf("invalid response format: missing 'data' field")
    }

    return &models.UserInfor{
        UserID:   safeString(innerData["id"]),
        UserName: safeString(innerData["username"]),
        FullName: safeString(innerData["fullname"]),
        Avartar:  safeString(innerData["avatar"]),
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

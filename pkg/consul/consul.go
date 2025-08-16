package consul

import (
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"chat-service/config"
	"chat-service/pkg/zap"

	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/api/watch"
)

const (
	serviceName = "chat-service"
	ttl         = time.Second * 15
	checkId     = "chat-service-health-check"
	
	// Retry configuration
	maxRetries      = 10
	retryInterval   = 5 * time.Second
	connectionTimeout = 60 * time.Second
)

var (
	serviceId     = fmt.Sprintf("%s-%d", serviceName, rand.Intn(10000))
	defaultConfig *api.Config
)

type Client interface {
	Connect() *api.Client
	Deregister()
	IsHealthy() bool
}

type service struct {
	client    *api.Client
	log       zap.Logger
	cfg       *config.Config
	isHealthy bool
}

// NewConsulConn creates a new Consul connection with retry logic
func NewConsulConn(log zap.Logger, cfg *config.Config) *service {
	consulHost := cfg.Consul.Host
	if consulHost == "" {
		// Fallback to localhost if the host is not set in the config
		consulHost = "localhost"
	}

	defaultConfig = &api.Config{
		Address: fmt.Sprintf("%s:%s", consulHost, cfg.Consul.Port),
		HttpClient: &http.Client{
			Timeout: connectionTimeout,
		},
	}

	return &service{
		client: nil, // Will be initialized in Connect()
		log:    log,
		cfg:    cfg,
		isHealthy: false,
	}
}

// Connect establishes connection to Consul with retry logic
func (c *service) Connect() *api.Client {
	var client *api.Client
	var err error

	// Retry logic for connecting to Consul
	for attempt := 1; attempt <= maxRetries; attempt++ {
		client, err = api.NewClient(defaultConfig)
		if err != nil {
			c.log.Warnf("Failed to create Consul client (attempt %d/%d): %v", attempt, maxRetries, err)
		} else {
			// Test the connection by calling a simple API
			_, err = client.Agent().Self()
			if err == nil {
				c.log.Infof("Successfully connected to Consul on attempt %d", attempt)
				c.client = client
				c.isHealthy = true
				
				// Setup Consul after successful connection
				c.setupConsul()
				
				// Start health check updater
				go c.updateHealthCheck()
				
				return c.client
			}
			c.log.Warnf("Failed to test Consul connection (attempt %d/%d): %v", attempt, maxRetries, err)
		}

		if attempt < maxRetries {
			c.log.Infof("Retrying Consul connection in %v...", retryInterval)
			time.Sleep(retryInterval)
		}
	}

	c.log.Fatalf("Failed to connect to Consul after %d attempts: %v", maxRetries, err)
	return nil
}

// Deregister removes the service from Consul
func (c *service) Deregister() {
	if c.client == nil {
		c.log.Warn("Cannot deregister: Consul client is nil")
		return
	}

	// Deregister service
	err := c.client.Agent().ServiceDeregister(serviceId)
	if err != nil {
		c.log.Errorf("Failed to deregister service: %v", err)
	} else {
		c.log.Infof("Successfully deregistered service: %s", serviceId)
	}
}

// IsHealthy checks if the Consul connection is healthy
func (c *service) IsHealthy() bool {
	if c.client == nil {
		return false
	}

	_, err := c.client.Agent().Self()
	c.isHealthy = err == nil
	return c.isHealthy
}

// updateHealthCheck periodically updates the TTL health check
func (c *service) updateHealthCheck() {
	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if c.client == nil {
				c.log.Error("Cannot update health check: Consul client is nil")
				continue
			}

			err := c.client.Agent().UpdateTTL(checkId, "online", api.HealthPassing)
			if err != nil {
				c.log.Errorf("Failed to update health check: %v", err)
				c.isHealthy = false
				
				// Try to reconnect if health check fails consistently
				if !c.IsHealthy() {
					c.log.Warn("Health check failed, attempting to reconnect...")
					go c.attemptReconnect()
				}
			} else {
				c.isHealthy = true
			}
		}
	}
}

// attemptReconnect tries to reconnect to Consul
func (c *service) attemptReconnect() {
	// Prevent multiple reconnection attempts
	if !c.isHealthy {
		c.log.Info("Attempting to reconnect to Consul...")
		
		for attempt := 1; attempt <= 3; attempt++ {
			client, err := api.NewClient(defaultConfig)
			if err != nil {
				c.log.Warnf("Reconnection attempt %d failed: %v", attempt, err)
				if attempt < 3 {
					time.Sleep(retryInterval)
				}
				continue
			}

			// Test the connection
			_, err = client.Agent().Self()
			if err == nil {
				c.log.Info("Successfully reconnected to Consul")
				c.client = client
				c.isHealthy = true
				
				// Re-register the service
				c.setupConsul()
				return
			}
			
			c.log.Warnf("Reconnection test failed (attempt %d): %v", attempt, err)
			if attempt < 3 {
				time.Sleep(retryInterval)
			}
		}
		
		c.log.Error("Failed to reconnect to Consul after 3 attempts")
	}
}

// setupConsul registers the service and sets up service discovery
func (c *service) setupConsul() {
	if c.client == nil {
		c.log.Error("Cannot setup Consul: client is nil")
		return
	}

	hostname := c.cfg.Registry.Host
	port, err := strconv.Atoi(c.cfg.App.API.Rest.Port)
	if err != nil {
		c.log.Errorf("Invalid port configuration: %v", err)
		return
	}

	// Health check configuration
	check := &api.AgentServiceCheck{
		DeregisterCriticalServiceAfter: ttl.String(),
		TTL:     ttl.String(),
		CheckID: checkId,
	}

	// Service registration
	registration := &api.AgentServiceRegistration{
		ID:      serviceId,
		Name:    serviceName,
		Port:    port,
		Address: hostname,
		Tags:    []string{"go", "chat-service", "v1"},
		Check:   check,
	}

	// Setup service discovery watcher
	query := map[string]any{
		"type":        "service",
		"service":     serviceName,
		"passingonly": true,
	}

	plan, err := watch.Parse(query)
	if err != nil {
		c.log.Errorf("Failed to create watch plan: %v", err)
		return
	}

	plan.HybridHandler = func(index watch.BlockingParamVal, result interface{}) {
		switch msg := result.(type) {
		case []*api.ServiceEntry:
			for _, entry := range msg {
				if entry.Service.ID != serviceId { // Don't log our own service
					c.log.Infof("Service discovered: <%s> on node <%s>, address: %s:%d", 
						entry.Service.Service, 
						entry.Node.Node,
						entry.Service.Address,
						entry.Service.Port)
				}
			}
		default:
			c.log.Warnf("Unexpected watch result type: %T", msg)
		}
	}

	// Start the watcher in a separate goroutine
	go func() {
		consulAddr := fmt.Sprintf("%s:%s", c.cfg.Consul.Host, c.cfg.Consul.Port)
		err := plan.RunWithConfig(consulAddr, api.DefaultConfig())
		if err != nil {
			c.log.Errorf("Service discovery watcher failed: %v", err)
		}
	}()

	// Register the service
	err = c.client.Agent().ServiceRegister(registration)
	if err != nil {
		c.log.Errorf("Failed to register service %s:%d - %v", hostname, port, err)
		return
	}

	c.log.Infof("Successfully registered service: %s:%d with ID: %s", hostname, port, serviceId)
}
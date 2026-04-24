package handlers

import (
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"clawreef/internal/models"
	"clawreef/internal/services"
	"clawreef/internal/services/k8s"
	"clawreef/internal/utils"

	"github.com/gin-gonic/gin"
)

// InternalHandler handles internal API requests from other services
// (e.g. Five-Star AI island / openclaw-service).
type InternalHandler struct {
	instanceService services.InstanceService
	internalToken   string
}

// NewInternalHandler creates a new internal handler
func NewInternalHandler(instanceService services.InstanceService) *InternalHandler {
	return &InternalHandler{
		instanceService: instanceService,
		internalToken:   strings.TrimSpace(os.Getenv("INTERNAL_API_TOKEN")),
	}
}

// InternalAuthMiddleware validates the X-Internal-Token header.
// If INTERNAL_API_TOKEN is not configured, requests are allowed (dev mode).
func (h *InternalHandler) InternalAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h.internalToken == "" {
			c.Next()
			return
		}
		token := c.GetHeader("X-Internal-Token")
		if token != h.internalToken {
			utils.Error(c, http.StatusUnauthorized, "invalid internal token")
			c.Abort()
			return
		}
		c.Next()
	}
}

// InternalLoginRequest represents an internal login request
type InternalLoginRequest struct {
	InstanceID string `json:"instance_id" binding:"required"`
}

// InternalLogin generates a short-lived access token for an instance.
func (h *InternalHandler) InternalLogin(c *gin.Context) {
	var req InternalLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.ValidationError(c, err)
		return
	}

	id, err := strconv.Atoi(req.InstanceID)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid instance id")
		return
	}

	instance, err := h.instanceService.GetByID(id)
	if err != nil || instance == nil {
		utils.Error(c, http.StatusNotFound, "instance not found")
		return
	}

	token := generateInternalToken(id)

	utils.Success(c, http.StatusOK, "token generated", gin.H{
		"token": token,
	})
}

// ListInstances returns all instances enriched with cross-cluster gateway routing info.
func (h *InternalHandler) ListInstances(c *gin.Context) {
	instances, total, err := h.instanceService.GetVisibleInstances(0, "admin", 0, 10000)
	if err != nil {
		utils.HandleError(c, err)
		return
	}

	gatewayBase := sidecarGatewayBaseURL()
	client := k8s.GetClient()

	result := make([]gin.H, 0, len(instances))
	for i := range instances {
		result = append(result, instanceToInternalResponse(&instances[i], gatewayBase, client))
	}

	utils.Success(c, http.StatusOK, "instances retrieved", gin.H{
		"instances": result,
		"total":     total,
	})
}

// GetInstance returns a single instance with gateway routing info.
func (h *InternalHandler) GetInstance(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid instance id")
		return
	}

	instance, err := h.instanceService.GetByID(id)
	if err != nil || instance == nil {
		utils.Error(c, http.StatusNotFound, "instance not found")
		return
	}

	gatewayBase := sidecarGatewayBaseURL()
	client := k8s.GetClient()

	utils.Success(c, http.StatusOK, "instance retrieved",
		instanceToInternalResponse(instance, gatewayBase, client))
}

// RestartInstance restarts an instance.
func (h *InternalHandler) RestartInstance(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "invalid instance id")
		return
	}

	if err := h.instanceService.Restart(id); err != nil {
		utils.HandleError(c, err)
		return
	}

	utils.Success(c, http.StatusOK, "instance restarted", nil)
}

// instanceToInternalResponse converts an Instance to the response shape expected
// by openclaw-service (Five-Star AI island).
func instanceToInternalResponse(inst *models.Instance, gatewayBase string, client *k8s.Client) gin.H {
	namespace := ""
	serviceName := ""
	proxyTargetBase := ""
	sidecarServiceDNS := ""

	if client != nil {
		namespace = client.GetNamespace(inst.UserID)
		serviceName = client.GetServiceName(inst.ID, inst.Name)
	}

	if gatewayBase != "" && namespace != "" && serviceName != "" {
		proxyTargetBase = gatewayBase + "/api/" + namespace + "/" + serviceName + "/desktop"
		sidecarServiceDNS = gatewayBase + "/api/" + namespace + "/" + serviceName + "/sidecar"
	}

	return gin.H{
		"id":                  inst.ID,
		"name":                inst.Name,
		"user_id":             inst.UserID,
		"type":                inst.Type,
		"status":              inst.Status,
		"proxy_target_base":   proxyTargetBase,
		"sidecar_service_dns": sidecarServiceDNS,
		"namespace":           namespace,
		"service_name":        serviceName,
		"pod_name":            inst.PodName,
		"pod_namespace":       inst.PodNamespace,
		"pod_ip":              inst.PodIP,
		"cpu_cores":           inst.CPUCores,
		"memory_gb":           inst.MemoryGB,
		"disk_gb":             inst.DiskGB,
		"gpu_enabled":         inst.GPUEnabled,
		"gpu_count":           inst.GPUCount,
		"os_type":             inst.OSType,
		"os_version":          inst.OSVersion,
		"created_at":          inst.CreatedAt,
		"updated_at":          inst.UpdatedAt,
	}
}

// sidecarGatewayBaseURL reads the sidecar gateway base URL from environment.
func sidecarGatewayBaseURL() string {
	return strings.TrimSpace(os.Getenv("SIDECAR_GATEWAY_BASE_URL"))
}

func generateInternalToken(instanceID int) string {
	return "internal_" + strconv.Itoa(instanceID) + "_" + strconv.FormatInt(time.Now().Unix(), 10)
}

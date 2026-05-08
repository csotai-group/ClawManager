package k8s

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// PodService handles Pod operations
type PodService struct {
	client *Client
}

const (
	podDeletionPollInterval = 500 * time.Millisecond
	podDeletionTimeout      = 60 * time.Second
)

// NewPodService creates a new Pod service
func NewPodService() *PodService {
	return &PodService{
		client: globalClient,
	}
}

// GetClient returns the k8s client
func (s *PodService) GetClient() *Client {
	return s.client
}

// PodConfig holds configuration for creating a pod
type PodConfig struct {
	InstanceID           int
	InstanceName         string
	UserID               int
	Type                 string
	CPUCores             float64
	MemoryGB             int
	GPUEnabled           bool
	GPUCount             int
	Image                string
	MountPath            string
	ContainerPort        int32
	ExtraEnv             map[string]string
	EnvFromSecretNames   []string
	SidecarEnabled       bool
	SidecarImage         string
	InitContainerEnabled bool
	InitContainerImage   string
	InitContainerToken   string
}

// CreatePod creates a new pod for an instance
func (s *PodService) CreatePod(ctx context.Context, config PodConfig) (*corev1.Pod, error) {
	if s.client == nil {
		return nil, fmt.Errorf("k8s client not initialized")
	}

	podName := s.client.GetPodName(config.InstanceID, config.InstanceName)
	namespace := s.client.GetNamespace(config.UserID)
	pvcName := s.client.GetPVCName(config.InstanceID)

	// Build resource requirements
	resources := corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%g", config.CPUCores)),
			corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dGi", config.MemoryGB)),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(fmt.Sprintf("%g", config.CPUCores)),
			corev1.ResourceMemory: resource.MustParse(fmt.Sprintf("%dGi", config.MemoryGB)),
		},
	}

	// Add GPU resources if enabled
	if config.GPUEnabled && config.GPUCount > 0 {
		resources.Limits["nvidia.com/gpu"] = resource.MustParse(fmt.Sprintf("%d", config.GPUCount))
		resources.Requests["nvidia.com/gpu"] = resource.MustParse(fmt.Sprintf("%d", config.GPUCount))
	}

	// Default container port
	if config.ContainerPort == 0 {
		config.ContainerPort = 3001
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      podName,
			Namespace: namespace,
			Labels: map[string]string{
				"app":           "clawreef",
				"instance-id":   fmt.Sprintf("%d", config.InstanceID),
				"instance-name": config.InstanceName,
				"user-id":       fmt.Sprintf("%d", config.UserID),
				"instance-type": config.Type,
				"managed-by":    "clawreef",
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:  "desktop",
					Image: config.Image,
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: config.ContainerPort,
							Name:          "http",
						},
					},
					StartupProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							TCPSocket: &corev1.TCPSocketAction{
								Port: intstrFromInt32(config.ContainerPort),
							},
						},
						FailureThreshold: 30,
						PeriodSeconds:    5,
						TimeoutSeconds:   2,
					},
					ReadinessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							TCPSocket: &corev1.TCPSocketAction{
								Port: intstrFromInt32(config.ContainerPort),
							},
						},
						InitialDelaySeconds: 3,
						PeriodSeconds:       5,
						TimeoutSeconds:      2,
						FailureThreshold:    6,
					},
					LivenessProbe: &corev1.Probe{
						ProbeHandler: corev1.ProbeHandler{
							TCPSocket: &corev1.TCPSocketAction{
								Port: intstrFromInt32(config.ContainerPort),
							},
						},
						InitialDelaySeconds: 15,
						PeriodSeconds:       10,
						TimeoutSeconds:      2,
						FailureThreshold:    3,
					},
					Resources: resources,
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "data",
							MountPath: config.MountPath,
						},
					},
					Env: []corev1.EnvVar{
						{
							Name:  "INSTANCE_ID",
							Value: fmt.Sprintf("%d", config.InstanceID),
						},
						{
							Name:  "USER_ID",
							Value: fmt.Sprintf("%d", config.UserID),
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "data",
					VolumeSource: corev1.VolumeSource{
						PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
							ClaimName: pvcName,
						},
					},
				},
			},
		},
	}

	for key, value := range config.ExtraEnv {
		pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, corev1.EnvVar{
			Name:  key,
			Value: value,
		})
	}

	for _, secretName := range config.EnvFromSecretNames {
		if secretName == "" {
			continue
		}
		pod.Spec.Containers[0].EnvFrom = append(pod.Spec.Containers[0].EnvFrom, corev1.EnvFromSource{
			SecretRef: &corev1.SecretEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
			},
		})
	}

	// Add init container for openclaw instances
	if config.InitContainerEnabled && config.InitContainerImage != "" {
		initContainer := corev1.Container{
			Name:    "bootstrap",
			Image:   config.InitContainerImage,
			Command: []string{"/bin/sh", "-c"},
			Args: []string{`set -eu

OPENCLAW_DIR="/config/.openclaw"
OPENCLAW_CONFIG="${OPENCLAW_DIR}/openclaw.json"
SENTINEL="/config/.bootstrap-v1.done"

mkdir -p "$OPENCLAW_DIR" /config/workspace /config/skills /config/scripts

if [ ! -f "$SENTINEL" ]; then
  if [ -d /seed/workspace ]; then
    cp -R /seed/workspace/. /config/workspace/
  fi

  if [ -d /seed/skills ]; then
    cp -R /seed/skills/. /config/skills/
  fi

  if [ -d /seed/scripts ]; then
    cp -R /seed/scripts/. /config/scripts/
  fi
fi

if [ ! -f "$OPENCLAW_CONFIG" ]; then
  TOKEN_ESCAPED="$(printf '%s' "$OPENCLAW_BOOTSTRAP_TOKEN" | sed 's/[\\/&]/\\\\&/g')"
  sed "s/__GATEWAY_TOKEN__/${TOKEN_ESCAPED}/g" /seed/openclaw.json.tpl > "$OPENCLAW_CONFIG"
fi

if command -v node >/dev/null 2>&1; then
  OPENCLAW_CONFIG_TOKEN="$OPENCLAW_BOOTSTRAP_TOKEN" OPENCLAW_CONFIG_PATH="$OPENCLAW_CONFIG" node <<'NODE'
const fs = require('fs');

const configPath = process.env.OPENCLAW_CONFIG_PATH;
const raw = fs.readFileSync(configPath, 'utf8').trim();
const config = raw ? JSON.parse(raw) : {};
if (!config.gateway || typeof config.gateway !== 'object' || Array.isArray(config.gateway)) {
  config.gateway = {};
}
if (!config.gateway.auth || typeof config.gateway.auth !== 'object' || Array.isArray(config.gateway.auth)) {
  config.gateway.auth = {};
}
config.gateway.auth.mode = 'token';
config.gateway.auth.token = process.env.OPENCLAW_CONFIG_TOKEN;
fs.writeFileSync(configPath, JSON.stringify(config, null, 2) + '\n');
NODE
else
  echo "node is required to configure gateway.auth.token" >&2
  exit 1
fi

chmod 600 "$OPENCLAW_CONFIG" || true
touch "$SENTINEL"`},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "data",
					MountPath: "/config",
				},
			},
			Env: []corev1.EnvVar{
				{
					Name:  "OPENCLAW_BOOTSTRAP_TOKEN",
					Value: config.InitContainerToken,
				},
			},
		}
		pod.Spec.InitContainers = []corev1.Container{initContainer}
	}

	// Add sidecar container for openclaw instances
	if config.SidecarEnabled && config.SidecarImage != "" {
		sidecarContainer := corev1.Container{
			Name:  "sidecar",
			Image: config.SidecarImage,
			Ports: []corev1.ContainerPort{
				{
					ContainerPort: 5000,
					Name:          "sidecar",
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "data",
					MountPath: "/config",
				},
			},
			Env: []corev1.EnvVar{
				{
					Name:  "INSTANCE_ID",
					Value: fmt.Sprintf("%d", config.InstanceID),
				},
				{
					Name:  "USER_ID",
					Value: fmt.Sprintf("%d", config.UserID),
				},
			},
		}
		pod.Spec.Containers = append(pod.Spec.Containers, sidecarContainer)
	}

	createdPod, err := s.client.Clientset.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		// Check if pod already exists
		if errors.IsAlreadyExists(err) {
			// Try to get the existing pod with the same name. It may still be terminating.
			existingPod, getErr := s.client.Clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
			if getErr == nil && existingPod != nil {
				if existingPod.DeletionTimestamp == nil {
					deleteErr := s.client.Clientset.CoreV1().Pods(namespace).Delete(ctx, existingPod.Name, metav1.DeleteOptions{})
					if deleteErr != nil && !errors.IsNotFound(deleteErr) {
						return nil, fmt.Errorf("failed to delete existing pod %s: %w", existingPod.Name, deleteErr)
					}
				}

				if waitErr := s.waitForPodDeletion(ctx, namespace, existingPod.Name); waitErr != nil {
					return nil, fmt.Errorf("failed waiting for pod deletion %s: %w", existingPod.Name, waitErr)
				}

				// Retry creation
				createdPod, err = s.client.Clientset.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
				if err != nil {
					return nil, fmt.Errorf("failed to create pod after deletion %s: %w", podName, err)
				}
				return createdPod, nil
			}
		}
		return nil, fmt.Errorf("failed to create pod %s: %w", podName, err)
	}

	return createdPod, nil
}

func intstrFromInt32(port int32) intstr.IntOrString {
	return intstr.FromInt32(port)
}

// GetPod gets a pod by instance ID
func (s *PodService) GetPod(ctx context.Context, userID, instanceID int) (*corev1.Pod, error) {
	if s.client == nil {
		return nil, fmt.Errorf("k8s client not initialized")
	}

	// List pods with instance-id label
	namespace := s.client.GetNamespace(userID)
	selector := fmt.Sprintf("instance-id=%d", instanceID)

	pods, err := s.client.Clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("pod not found for instance %d", instanceID)
	}

	return &pods.Items[0], nil
}

// DeletePod deletes a pod
func (s *PodService) DeletePod(ctx context.Context, userID, instanceID int) error {
	if s.client == nil {
		return fmt.Errorf("k8s client not initialized")
	}

	pod, err := s.GetPod(ctx, userID, instanceID)
	if err != nil {
		// Pod doesn't exist, nothing to delete
		if isNotFoundError(err) {
			return nil
		}
		return err
	}

	err = s.client.Clientset.CoreV1().Pods(pod.Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to delete pod %s: %w", pod.Name, err)
	}

	if err := s.waitForPodDeletion(ctx, pod.Namespace, pod.Name); err != nil {
		return fmt.Errorf("failed waiting for pod %s to be deleted: %w", pod.Name, err)
	}

	return nil
}

// GetPodStatus gets the status of a pod
func (s *PodService) GetPodStatus(ctx context.Context, userID, instanceID int) (*corev1.PodStatus, error) {
	pod, err := s.GetPod(ctx, userID, instanceID)
	if err != nil {
		return nil, err
	}
	return &pod.Status, nil
}

// GetPodIP gets the pod IP
func (s *PodService) GetPodIP(ctx context.Context, userID, instanceID int) (string, error) {
	pod, err := s.GetPod(ctx, userID, instanceID)
	if err != nil {
		return "", err
	}
	return pod.Status.PodIP, nil
}

// PodExists checks if a pod exists
func (s *PodService) PodExists(ctx context.Context, userID, instanceID int) (bool, error) {
	_, err := s.GetPod(ctx, userID, instanceID)
	if err != nil {
		if isNotFoundError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return containsSubstring(errStr, "not found") ||
		containsSubstring(errStr, "NotFound")
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func (s *PodService) waitForPodDeletion(ctx context.Context, namespace, podName string) error {
	waitCtx, cancel := context.WithTimeout(ctx, podDeletionTimeout)
	defer cancel()

	ticker := time.NewTicker(podDeletionPollInterval)
	defer ticker.Stop()

	for {
		_, err := s.client.Clientset.CoreV1().Pods(namespace).Get(waitCtx, podName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("failed to check pod %s: %w", podName, err)
		}

		select {
		case <-waitCtx.Done():
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("timed out waiting for pod %s deletion", podName)
		case <-ticker.C:
		}
	}
}

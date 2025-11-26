package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ginbear/k8s-flowtop/internal/types"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Client wraps kubernetes clients
type Client struct {
	clientset     *kubernetes.Clientset
	dynamicClient dynamic.Interface
	namespace     string
	context       string
	cluster       string
}

// NewClient creates a new kubernetes client
func NewClient(namespace string) (*Client, error) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		home, _ := os.UserHomeDir()
		kubeconfig = filepath.Join(home, ".kube", "config")
	}

	// Load kubeconfig to get context and cluster info
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig}
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	rawConfig, err := kubeConfig.RawConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	currentContext := rawConfig.CurrentContext
	var clusterName string
	if ctx, ok := rawConfig.Contexts[currentContext]; ok {
		clusterName = ctx.Cluster
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	return &Client{
		clientset:     clientset,
		dynamicClient: dynamicClient,
		namespace:     namespace,
		context:       currentContext,
		cluster:       clusterName,
	}, nil
}

// GetNamespace returns the current namespace
func (c *Client) GetNamespace() string {
	return c.namespace
}

// GetContext returns the current context name
func (c *Client) GetContext() string {
	return c.context
}

// GetCluster returns the current cluster name
func (c *Client) GetCluster() string {
	return c.cluster
}

// SetNamespace sets the namespace to watch
func (c *Client) SetNamespace(ns string) {
	c.namespace = ns
}

// ListJobs returns all jobs in the namespace
func (c *Client) ListJobs(ctx context.Context) ([]types.AsyncResource, error) {
	var resources []types.AsyncResource

	jobs, err := c.clientset.BatchV1().Jobs(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, job := range jobs.Items {
		resources = append(resources, jobToResource(job))
	}

	return resources, nil
}

// ListCronJobs returns all cronjobs in the namespace
func (c *Client) ListCronJobs(ctx context.Context) ([]types.AsyncResource, error) {
	var resources []types.AsyncResource

	cronJobs, err := c.clientset.BatchV1().CronJobs(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, cj := range cronJobs.Items {
		resources = append(resources, cronJobToResource(cj))
	}

	return resources, nil
}

// Argo Workflows GVR
var (
	workflowGVR = schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "workflows",
	}
	cronWorkflowGVR = schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "cronworkflows",
	}
	sensorGVR = schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "sensors",
	}
	eventSourceGVR = schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "eventsources",
	}
)

// ListWorkflows returns all Argo Workflows
func (c *Client) ListWorkflows(ctx context.Context) ([]types.AsyncResource, error) {
	var resources []types.AsyncResource

	list, err := c.dynamicClient.Resource(workflowGVR).Namespace(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		// Argo Workflows might not be installed
		return resources, nil
	}

	for _, item := range list.Items {
		resources = append(resources, workflowToResource(item))
	}

	return resources, nil
}

// ListCronWorkflows returns all Argo CronWorkflows
func (c *Client) ListCronWorkflows(ctx context.Context) ([]types.AsyncResource, error) {
	var resources []types.AsyncResource

	list, err := c.dynamicClient.Resource(cronWorkflowGVR).Namespace(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return resources, nil
	}

	for _, item := range list.Items {
		resources = append(resources, cronWorkflowToResource(item))
	}

	return resources, nil
}

// ListSensors returns all Argo Events Sensors
func (c *Client) ListSensors(ctx context.Context) ([]types.AsyncResource, error) {
	var resources []types.AsyncResource

	list, err := c.dynamicClient.Resource(sensorGVR).Namespace(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return resources, nil
	}

	for _, item := range list.Items {
		resources = append(resources, sensorToResource(item))
	}

	return resources, nil
}

// ListEventSources returns all Argo Events EventSources
func (c *Client) ListEventSources(ctx context.Context) ([]types.AsyncResource, error) {
	var resources []types.AsyncResource

	list, err := c.dynamicClient.Resource(eventSourceGVR).Namespace(c.namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return resources, nil
	}

	for _, item := range list.Items {
		resources = append(resources, eventSourceToResource(item))
	}

	return resources, nil
}

// ListAll returns all async resources
func (c *Client) ListAll(ctx context.Context) ([]types.AsyncResource, error) {
	var all []types.AsyncResource

	jobs, _ := c.ListJobs(ctx)
	all = append(all, jobs...)

	cronJobs, _ := c.ListCronJobs(ctx)
	all = append(all, cronJobs...)

	workflows, _ := c.ListWorkflows(ctx)
	all = append(all, workflows...)

	cronWorkflows, _ := c.ListCronWorkflows(ctx)
	all = append(all, cronWorkflows...)

	sensors, _ := c.ListSensors(ctx)
	all = append(all, sensors...)

	eventSources, _ := c.ListEventSources(ctx)
	all = append(all, eventSources...)

	return all, nil
}

// Helper functions to convert k8s resources to AsyncResource

func jobToResource(job batchv1.Job) types.AsyncResource {
	r := types.AsyncResource{
		Kind:      types.KindJob,
		Name:      job.Name,
		Namespace: job.Namespace,
		Status:    types.StatusUnknown,
	}

	// Extract parent from ownerReferences (for Jobs spawned by CronJob)
	for _, ref := range job.OwnerReferences {
		if ref.Kind == "CronJob" {
			r.ParentKind = ref.Kind
			r.ParentName = ref.Name
			break
		}
	}

	if job.Status.StartTime != nil {
		t := job.Status.StartTime.Time
		r.StartTime = &t
	}

	if job.Status.CompletionTime != nil {
		t := job.Status.CompletionTime.Time
		r.EndTime = &t
		if r.StartTime != nil {
			r.Duration = r.EndTime.Sub(*r.StartTime)
		}
	} else if r.StartTime != nil {
		r.Duration = time.Since(*r.StartTime)
	}

	// Determine status
	if job.Status.Succeeded > 0 {
		r.Status = types.StatusSucceeded
	} else if job.Status.Failed > 0 {
		r.Status = types.StatusFailed
		r.Retries = int(job.Status.Failed)
	} else if job.Status.Active > 0 {
		r.Status = types.StatusRunning
	} else {
		r.Status = types.StatusPending
	}

	r.SuccessCount = int(job.Status.Succeeded)
	r.FailureCount = int(job.Status.Failed)

	return r
}

func cronJobToResource(cj batchv1.CronJob) types.AsyncResource {
	r := types.AsyncResource{
		Kind:      types.KindCronJob,
		Name:      cj.Name,
		Namespace: cj.Namespace,
		Schedule:  cj.Spec.Schedule,
		Status:    types.StatusRunning,
	}

	// Extract timezone if specified (Kubernetes 1.25+)
	if cj.Spec.TimeZone != nil {
		r.Timezone = *cj.Spec.TimeZone
	}

	if cj.Status.LastScheduleTime != nil {
		t := cj.Status.LastScheduleTime.Time
		r.LastRun = &t
	}

	if cj.Status.LastSuccessfulTime != nil {
		t := cj.Status.LastSuccessfulTime.Time
		r.EndTime = &t
	}

	// Count active jobs
	if len(cj.Status.Active) > 0 {
		r.Status = types.StatusRunning
	}

	return r
}

func workflowToResource(obj unstructured.Unstructured) types.AsyncResource {
	r := types.AsyncResource{
		Kind:      types.KindWorkflow,
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Status:    types.StatusUnknown,
	}

	// Extract parent from ownerReferences (for Workflows spawned by CronWorkflow)
	ownerRefs := obj.GetOwnerReferences()
	for _, ref := range ownerRefs {
		if ref.Kind == "CronWorkflow" {
			r.ParentKind = ref.Kind
			r.ParentName = ref.Name
			break
		}
	}

	status, _, _ := unstructured.NestedMap(obj.Object, "status")
	if status != nil {
		if phase, ok := status["phase"].(string); ok {
			switch phase {
			case "Running":
				r.Status = types.StatusRunning
			case "Succeeded":
				r.Status = types.StatusSucceeded
			case "Failed", "Error":
				r.Status = types.StatusFailed
			case "Pending":
				r.Status = types.StatusPending
			}
		}

		if msg, ok := status["message"].(string); ok {
			r.Message = msg
		}

		if startedAt, ok := status["startedAt"].(string); ok {
			if t, err := time.Parse(time.RFC3339, startedAt); err == nil {
				r.StartTime = &t
			}
		}

		if finishedAt, ok := status["finishedAt"].(string); ok {
			if t, err := time.Parse(time.RFC3339, finishedAt); err == nil {
				r.EndTime = &t
			}
		}

		if r.StartTime != nil {
			if r.EndTime != nil {
				r.Duration = r.EndTime.Sub(*r.StartTime)
			} else {
				r.Duration = time.Since(*r.StartTime)
			}
		}

		// Extract DAG nodes
		if nodes, ok := status["nodes"].(map[string]interface{}); ok {
			for _, nodeData := range nodes {
				if node, ok := nodeData.(map[string]interface{}); ok {
					dagNode := types.DAGNode{}
					if name, ok := node["displayName"].(string); ok {
						dagNode.Name = name
					}
					if nodeType, ok := node["type"].(string); ok {
						dagNode.Type = nodeType
					}
					if phase, ok := node["phase"].(string); ok {
						dagNode.Phase = phase
					}
					// Only include meaningful nodes (skip empty names)
					if dagNode.Name != "" {
						r.DAGNodes = append(r.DAGNodes, dagNode)
					}
				}
			}
		}
	}

	return r
}

func cronWorkflowToResource(obj unstructured.Unstructured) types.AsyncResource {
	r := types.AsyncResource{
		Kind:      types.KindCronWorkflow,
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Status:    types.StatusRunning,
	}

	spec, _, _ := unstructured.NestedMap(obj.Object, "spec")
	if spec != nil {
		if schedule, ok := spec["schedule"].(string); ok {
			r.Schedule = schedule
		}
		if timezone, ok := spec["timezone"].(string); ok {
			r.Timezone = timezone
		}
	}

	status, _, _ := unstructured.NestedMap(obj.Object, "status")
	if status != nil {
		if lastScheduled, ok := status["lastScheduledTime"].(string); ok {
			if t, err := time.Parse(time.RFC3339, lastScheduled); err == nil {
				r.LastRun = &t
			}
		}
	}

	return r
}

func sensorToResource(obj unstructured.Unstructured) types.AsyncResource {
	r := types.AsyncResource{
		Kind:      types.KindSensor,
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Status:    types.StatusUnknown,
	}

	status, _, _ := unstructured.NestedMap(obj.Object, "status")
	if status != nil {
		conditions, _, _ := unstructured.NestedSlice(status, "conditions")
		for _, c := range conditions {
			if cond, ok := c.(map[string]interface{}); ok {
				if cond["type"] == "Ready" {
					if cond["status"] == "True" {
						r.Status = types.StatusRunning
					} else {
						r.Status = types.StatusFailed
						if msg, ok := cond["message"].(string); ok {
							r.Message = msg
						}
					}
				}
			}
		}
	}

	return r
}

func eventSourceToResource(obj unstructured.Unstructured) types.AsyncResource {
	r := types.AsyncResource{
		Kind:      types.KindEventSource,
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Status:    types.StatusUnknown,
	}

	status, _, _ := unstructured.NestedMap(obj.Object, "status")
	if status != nil {
		conditions, _, _ := unstructured.NestedSlice(status, "conditions")
		for _, c := range conditions {
			if cond, ok := c.(map[string]interface{}); ok {
				if cond["type"] == "Ready" {
					if cond["status"] == "True" {
						r.Status = types.StatusRunning
					} else {
						r.Status = types.StatusFailed
						if msg, ok := cond["message"].(string); ok {
							r.Message = msg
						}
					}
				}
			}
		}
	}

	return r
}

package k8s

import (
	"context"
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type ContainerInfo struct {
	Name  string `json:"name"`
	Image string `json:"image"`
	Tag   string `json:"tag"`
}

type Workload struct {
	Kind            string          `json:"kind"`
	Name            string          `json:"name"`
	Namespace       string          `json:"namespace"`
	ReadyReplicas   int32           `json:"ready_replicas"`
	DesiredReplicas int32           `json:"desired_replicas"`
	Containers      []ContainerInfo `json:"containers"`
}

type WorkloadClient interface {
	ListWorkloads(ctx context.Context, namespace string) ([]Workload, error)
}

type k8sWorkloadClient struct {
	clientset *kubernetes.Clientset
}

func NewWorkloadClient(kubeconfig []byte) (WorkloadClient, error) {
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("parse kubeconfig: %w", err)
	}
	cs, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create clientset: %w", err)
	}
	return &k8sWorkloadClient{clientset: cs}, nil
}

func (c *k8sWorkloadClient) ListWorkloads(ctx context.Context, namespace string) ([]Workload, error) {
	workloads := make([]Workload, 0)

	deps, err := c.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, d := range deps.Items {
		workloads = append(workloads, deploymentToWorkload(d))
	}

	sts, err := c.clientset.AppsV1().StatefulSets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, s := range sts.Items {
		workloads = append(workloads, statefulSetToWorkload(s))
	}

	return workloads, nil
}

func deploymentToWorkload(d appsv1.Deployment) Workload {
	w := Workload{
		Kind:          "Deployment",
		Name:          d.Name,
		Namespace:     d.Namespace,
		ReadyReplicas: d.Status.ReadyReplicas,
	}
	if d.Spec.Replicas != nil {
		w.DesiredReplicas = *d.Spec.Replicas
	}
	for _, c := range d.Spec.Template.Spec.Containers {
		_, tag := ParseImageTag(c.Image)
		w.Containers = append(w.Containers, ContainerInfo{Name: c.Name, Image: c.Image, Tag: tag})
	}
	return w
}

func statefulSetToWorkload(s appsv1.StatefulSet) Workload {
	w := Workload{
		Kind:          "StatefulSet",
		Name:          s.Name,
		Namespace:     s.Namespace,
		ReadyReplicas: s.Status.ReadyReplicas,
	}
	if s.Spec.Replicas != nil {
		w.DesiredReplicas = *s.Spec.Replicas
	}
	for _, c := range s.Spec.Template.Spec.Containers {
		_, tag := ParseImageTag(c.Image)
		w.Containers = append(w.Containers, ContainerInfo{Name: c.Name, Image: c.Image, Tag: tag})
	}
	return w
}

func ParseImageTag(image string) (repo, tag string) {
	if idx := strings.LastIndex(image, ":"); idx > 0 {
		return image[:idx], image[idx+1:]
	}
	return image, "latest"
}

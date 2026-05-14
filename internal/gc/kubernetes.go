package gc

import (
	"context"
	"fmt"
	"io"

	"github.com/eznix86/docker-registry-cleaner/internal/config"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/remotecommand"
)

type KubernetesRunner struct {
	clientset *kubernetes.Clientset
	restCfg   *rest.Config
	cfg       config.Kubernetes
}

func NewKubernetes(cfg config.Kubernetes) (*KubernetesRunner, error) {
	var kubeCfg *rest.Config
	var err error

	kubeCfg, err = rest.InClusterConfig()
	if err != nil {
		kubeCfg, err = clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
		if err != nil {
			return nil, fmt.Errorf("loading kubernetes config: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(kubeCfg)
	if err != nil {
		return nil, fmt.Errorf("creating kubernetes client: %w", err)
	}

	return &KubernetesRunner{
		clientset: clientset,
		restCfg:   kubeCfg,
		cfg:       cfg,
	}, nil
}

func (r *KubernetesRunner) RunGC(ctx context.Context) error {
	podName, err := r.resolvePod(ctx)
	if err != nil {
		return fmt.Errorf("resolving pod: %w", err)
	}

	args := []string{"registry", "garbage-collect", r.cfg.GCConfigPath}
	if r.cfg.GCDeleteUnreferencedBlobs {
		args = append(args, "--delete-unreferenced-blobs")
	}

	return r.execInPod(ctx, podName, args)
}

func (r *KubernetesRunner) resolvePod(ctx context.Context) (string, error) {
	labelSelector := r.cfg.LabelSelector
	if labelSelector == "" {
		labelSelector = fmt.Sprintf("app=%s", r.cfg.Name)
	}

	pods, err := r.clientset.CoreV1().Pods(r.cfg.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return "", fmt.Errorf("listing pods: %w", err)
	}

	if len(pods.Items) == 0 {
		return "", fmt.Errorf("no pods found for %s/%s with selector %q", r.cfg.Workload, r.cfg.Name, labelSelector)
	}

	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			return pod.Name, nil
		}
	}

	return pods.Items[0].Name, nil
}

func (r *KubernetesRunner) execInPod(ctx context.Context, podName string, command []string) error {
	req := r.clientset.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(podName).
		Namespace(r.cfg.Namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Command: command,
		Stdin:   false,
		Stdout:  true,
		Stderr:  true,
		TTY:     false,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(r.restCfg, "POST", req.URL())
	if err != nil {
		return fmt.Errorf("creating executor: %w", err)
	}

	return exec.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdout: io.Discard,
		Stderr: io.Discard,
	})
}

package watcher

import (
	"context"
	"fmt"
	"time"

	"github.com/yourusername/kube-watcher/pkg/config"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

// Event represents a Kubernetes resource event
type Event struct {
	Kind      string
	Namespace string
	Name      string
	EventType string
	Timestamp time.Time
	Object    runtime.Object
	Labels    map[string]string
}

// EventHandler is a function that handles resource events
type EventHandler func(event *Event)

// Watcher watches Kubernetes resources and triggers events
type Watcher struct {
	clientset *kubernetes.Clientset
	config    *config.Config
	handler   EventHandler
	stopCh    chan struct{}
}

// NewWatcher creates a new Watcher instance
func NewWatcher(cfg *config.Config, handler EventHandler) (*Watcher, error) {
	// Try in-cluster config first, fall back to kubeconfig
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		// Try loading from kubeconfig
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		configOverrides := &clientcmd.ConfigOverrides{}
		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
		k8sConfig, err = kubeConfig.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to create kubernetes config: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	return &Watcher{
		clientset: clientset,
		config:    cfg,
		handler:   handler,
		stopCh:    make(chan struct{}),
	}, nil
}

// Start begins watching configured resources
func (w *Watcher) Start(ctx context.Context) error {
	factory := informers.NewSharedInformerFactoryWithOptions(
		w.clientset,
		time.Second*30,
		informers.WithNamespace(w.config.Namespace),
	)

	// Register informers for each configured resource
	for _, resource := range w.config.Resources {
		if err := w.registerInformer(factory, resource.Kind); err != nil {
			return fmt.Errorf("failed to register informer for %s: %w", resource.Kind, err)
		}
	}

	// Start all informers
	factory.Start(w.stopCh)

	// Wait for cache sync
	factory.WaitForCacheSync(w.stopCh)

	// Block until context is cancelled
	<-ctx.Done()
	close(w.stopCh)

	return nil
}

// registerInformer registers an informer for a specific resource kind
func (w *Watcher) registerInformer(factory informers.SharedInformerFactory, kind string) error {
	switch kind {
	case "Pod":
		informer := factory.Core().V1().Pods().Informer()
		informer.AddEventHandler(w.createEventHandler("Pod"))
	case "Deployment":
		informer := factory.Apps().V1().Deployments().Informer()
		informer.AddEventHandler(w.createEventHandler("Deployment"))
	case "Service":
		informer := factory.Core().V1().Services().Informer()
		informer.AddEventHandler(w.createEventHandler("Service"))
	case "ConfigMap":
		informer := factory.Core().V1().ConfigMaps().Informer()
		informer.AddEventHandler(w.createEventHandler("ConfigMap"))
	case "Secret":
		informer := factory.Core().V1().Secrets().Informer()
		informer.AddEventHandler(w.createEventHandler("Secret"))
	case "ReplicaSet":
		informer := factory.Apps().V1().ReplicaSets().Informer()
		informer.AddEventHandler(w.createEventHandler("ReplicaSet"))
	case "StatefulSet":
		informer := factory.Apps().V1().StatefulSets().Informer()
		informer.AddEventHandler(w.createEventHandler("StatefulSet"))
	case "DaemonSet":
		informer := factory.Apps().V1().DaemonSets().Informer()
		informer.AddEventHandler(w.createEventHandler("DaemonSet"))
	default:
		return fmt.Errorf("unsupported resource kind: %s", kind)
	}

	return nil
}

// createEventHandler creates a ResourceEventHandler for a specific resource kind
func (w *Watcher) createEventHandler(kind string) cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			event := w.convertToEvent(obj, kind, "ADDED")
			if event != nil {
				w.handler(event)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			event := w.convertToEvent(newObj, kind, "UPDATED")
			if event != nil {
				w.handler(event)
			}
		},
		DeleteFunc: func(obj interface{}) {
			event := w.convertToEvent(obj, kind, "DELETED")
			if event != nil {
				w.handler(event)
			}
		},
	}
}

// convertToEvent converts a Kubernetes object to an Event
func (w *Watcher) convertToEvent(obj interface{}, kind, eventType string) *Event {
	var meta metav1.Object
	var labels map[string]string

	// Extract metadata based on object type
	switch o := obj.(type) {
	case *corev1.Pod:
		meta = o
		labels = o.Labels
	case *appsv1.Deployment:
		meta = o
		labels = o.Labels
	case *corev1.Service:
		meta = o
		labels = o.Labels
	case *corev1.ConfigMap:
		meta = o
		labels = o.Labels
	case *corev1.Secret:
		meta = o
		labels = o.Labels
	case *appsv1.ReplicaSet:
		meta = o
		labels = o.Labels
	case *appsv1.StatefulSet:
		meta = o
		labels = o.Labels
	case *appsv1.DaemonSet:
		meta = o
		labels = o.Labels
	default:
		return nil
	}

	return &Event{
		Kind:      kind,
		Namespace: meta.GetNamespace(),
		Name:      meta.GetName(),
		EventType: eventType,
		Timestamp: time.Now(),
		Object:    obj.(runtime.Object),
		Labels:    labels,
	}
}

// Stop stops the watcher
func (w *Watcher) Stop() {
	close(w.stopCh)
}

package runner

import (
	"context"
	"fmt"
	"sync"

	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/tools/clientcmd"
)

type Pool struct {
	sync.Mutex

	AvailableC chan *kubernetes.Clientset
}

// NewPool returns a pool of Kubernetes clientset connections
func NewPool(workers int, config KubernetesConfig) (*Pool, error) {
	k8scfg, err := clientcmd.BuildConfigFromFlags("", config.KubeConfigPath)
	if err != nil {
		return nil, fmt.Errorf("could not start k8s client from config: %v", err)
	}

	pool := &Pool{
		AvailableC: make(chan *kubernetes.Clientset, workers),
	}

	for i := 0; i < workers; i++ {
		// Create the clientset
		cs, err := kubernetes.NewForConfig(k8scfg)
		if err != nil {
			return nil, fmt.Errorf("could not create clientset: %v", err)
		}

		pool.AvailableC <- cs
	}

	return pool, nil
}

func (p *Pool) Get(ctx context.Context) (*kubernetes.Clientset, error) {
	select {
	case cs := <-p.AvailableC:
		return cs, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (p *Pool) Put(cs *kubernetes.Clientset) {
	p.Lock()
	defer p.Unlock()

	p.AvailableC <- cs
}

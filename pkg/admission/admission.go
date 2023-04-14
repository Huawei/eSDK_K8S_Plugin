package admission

import (
	"fmt"
	"os"
	"sync"

	apiAdmissionsClient "k8s.io/client-go/kubernetes/typed/admissionregistration/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	instance Ops
	once     sync.Once
)

// Ops is an interface to the admission client wrapper.
type Ops interface {
	ValidatingWebhookCfgOps
}

// Instance returns a singleton instance of the client.
func Instance() Ops {
	once.Do(func() {
		if instance == nil {
			instance = &Client{}
		}
	})
	return instance
}

// Client provides a wrapper for kubernetes admission interface.
type Client struct {
	config    *rest.Config
	admission apiAdmissionsClient.AdmissionregistrationV1Interface
}

// initClient the k8s client if uninitialized
func (c *Client) initClient() error {
	if c.admission != nil {
		return nil
	}

	return c.setClient()
}

// setClient instantiates a client.
func (c *Client) setClient() error {
	var err error

	if c.config != nil {
		err = c.loadClient()
	} else {
		kubeConfig := os.Getenv("KUBECONFIG")
		if len(kubeConfig) > 0 {
			err = c.loadClientFromKubeConfig(kubeConfig)
		} else {
			err = c.loadClientFromServiceAccount()
		}
	}

	return err
}

// loadClientFromServiceAccount loads a k8s client from a ServiceAccount specified in the pod running px
func (c *Client) loadClientFromServiceAccount() error {
	config, err := rest.InClusterConfig()
	if err != nil {
		return err
	}

	c.config = config
	return c.loadClient()
}

func (c *Client) loadClientFromKubeConfig(kubeConfig string) error {
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		return err
	}

	c.config = config
	return c.loadClient()
}

func (c *Client) loadClient() error {
	if c.config == nil {
		return fmt.Errorf("rest config is not provided")
	}

	var err error

	c.admission, err = apiAdmissionsClient.NewForConfig(c.config)
	if err != nil {
		return err
	}

	return nil
}

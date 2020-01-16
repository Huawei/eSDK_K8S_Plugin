package main

import (
	"flag"
	"flexvolume/config"
	"runtime/debug"
	"strings"
	"utils/log"

	"github.com/golang/glog"
	"github.com/kubernetes-sigs/sig-storage-lib-external-provisioner/controller"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	provisioner = flag.String("provisioner", "huawei/storage", "Name of the provisioner. The provisioner will only provision volumes for claims that request a StorageClass with a provisioner field set equal to this name.")
	master      = flag.String("master", "", "Master URL to build a client config from. Either this or kubeconfig needs to be set if the provisioner is being run out of cluster.")
	kubeconfig  = flag.String("kubeconfig", "", "Absolute path to the kubeconfig file. Either this or master needs to be set if the provisioner is being run out of cluster.")
)

func init() {
	flag.Set("log_dir", "/var/log/huawei")
	flag.Parse()

	config.ParseConfig("/etc/huawei/provisioner.json")

	logFilePrefix := config.Config.LogFilePrefix
	if logFilePrefix == "" {
		logFilePrefix = "provisioner"
	}

	err := log.Init(map[string]string{
		"logFilePrefix": logFilePrefix,
		"logFileMaxCap": config.Config.MaxLogFileSize,
		"logDir":        config.Config.LogDir,
	})
	if err != nil {
		glog.Fatalf("Init log error: %v.", err)
	}
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Runtime error caught: %v", r)
			log.Errorf("%s", debug.Stack())
		}

		log.Flush()
		log.Close()
	}()

	errs := validateProvisioner(*provisioner, field.NewPath("provisioner"))
	if len(errs) != 0 {
		log.Fatalf("Invalid provisioner specified: %v", errs)
	}

	log.Infof("Provisioner %s specified", *provisioner)

	// Create the client according to whether we are running in or out-of-cluster
	outOfCluster := *master != "" || *kubeconfig != ""

	var restConfig *rest.Config
	var err error

	if outOfCluster {
		log.Infof("Building kube config for running out of cluster.")
		restConfig, err = clientcmd.BuildConfigFromFlags(*master, *kubeconfig)
	} else {
		log.Infof("Building kube configs for running in cluster.")
		restConfig, err = rest.InClusterConfig()
	}

	if err != nil {
		log.Fatalf("Failed to create config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	// The controller needs to know what the server version is because out-of-tree
	// provisioners aren't officially supported until 1.5
	serverVersion, err := clientset.Discovery().ServerVersion()
	if err != nil {
		log.Fatalf("Error getting server version: %v", err)
	}

	// Create the provisioner: it implements the Provisioner interface expected by
	// the controller
	p := NewProvisioner()

	// Start the provision controller which will dynamically provision NFS PVs
	pc := controller.NewProvisionController(clientset, *provisioner, p, serverVersion.GitVersion)
	pc.Run(wait.NeverStop)
}

func validateProvisioner(provisioner string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(provisioner) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, provisioner))
	}
	if len(provisioner) > 0 {
		for _, msg := range validation.IsQualifiedName(strings.ToLower(provisioner)) {
			allErrs = append(allErrs, field.Invalid(fldPath, provisioner, msg))
		}
	}
	return allErrs
}

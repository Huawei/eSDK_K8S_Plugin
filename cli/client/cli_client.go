package client

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"huawei-csi-driver/cli/config"
	"huawei-csi-driver/cli/utils"

	"github.com/cenkalti/backoff"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
)

type KubectlClient struct {
	cli       string
	flavor    OrchestratorFlavor
	version   *utils.Version
	namespace string
	timeout   time.Duration
}

type Version struct {
	ClientVersion struct {
		Major        string    `json:"major"`
		Minor        string    `json:"minor"`
		GitVersion   string    `json:"gitVersion"`
		GitCommit    string    `json:"gitCommit"`
		GitTreeState string    `json:"gitTreeState"`
		BuildDate    time.Time `json:"buildDate"`
		GoVersion    string    `json:"goVersion"`
		Compiler     string    `json:"compiler"`
		Platform     string    `json:"platform"`
	} `json:"clientVersion"`
	ServerVersion struct {
		Major        string    `json:"major"`
		Minor        string    `json:"minor"`
		GitVersion   string    `json:"gitVersion"`
		GitCommit    string    `json:"gitCommit"`
		GitTreeState string    `json:"gitTreeState"`
		BuildDate    time.Time `json:"buildDate"`
		GoVersion    string    `json:"goVersion"`
		Compiler     string    `json:"compiler"`
		Platform     string    `json:"platform"`
	} `json:"serverVersion"`
	OpenshiftVersion string `json:"openshiftVersion"`
}

type OrchestratorFlavor string

const (
	CLIKubernetes = "kubectl"
	CLIOpenShift  = "oc"

	FlavorKubernetes OrchestratorFlavor = "k8s"
	FlavorOpenShift  OrchestratorFlavor = "openshift"

	YAMLSeparator = `\n---\s*\n`
)

type Interface interface {
	ServerVersion() *utils.Version
	CheckSecretExists(secretName string) (bool, error)
	GetConfigMap(configMapName string) (*v1.ConfigMap, error)
	CheckConfigMapExists(configMapName string) (bool, error)
	CreateObjectByYAML(yaml string) error
	GetSecret(secretName string) (*v1.Secret, error)
}

func NewCliClient(namespace string, k8sTimeout time.Duration) (Interface, error) {
	cli, err := discoverKubernetesCLI()
	if err != nil {
		return nil, err
	}

	var flavor OrchestratorFlavor
	var k8sVersion *utils.Version

	// Discover Kubernetes server version
	switch cli {
	default:
		fallthrough
	case CLIKubernetes:
		flavor = FlavorKubernetes
		k8sVersion, err = discoverKubernetesServerVersion(cli)
	case CLIOpenShift:
		flavor = FlavorOpenShift
		k8sVersion, err = discoverKubernetesServerVersion(cli)
		if err != nil {
			k8sVersion, err = discoverOpenShift3ServerVersion(cli)
		}
	}
	if err != nil {
		return nil, err
	}

	k8sMMVersion := k8sVersion.ToMajorMinorVersion()
	minOptionalCSIVersion := utils.MustParseSemantic(config.MinKubernetesCSIVersion).ToMajorMinorVersion()
	maxOptionalCSIVersion := utils.MustParseSemantic(config.MaxKubernetesCSIVersion).ToMajorMinorVersion()

	if k8sMMVersion.LessThan(minOptionalCSIVersion) || k8sMMVersion.GreaterThan(maxOptionalCSIVersion) {
		return nil, fmt.Errorf("%s %s supports Kubernetes versions in the range [%s, %s]",
			strings.Title(config.OrchestratorName), config.OrchestratorVersion,
			minOptionalCSIVersion.ToMajorMinorString(), maxOptionalCSIVersion.ToMajorMinorString())
	}

	client := &KubectlClient{
		cli:       cli,
		flavor:    flavor,
		version:   k8sVersion,
		namespace: namespace,
		timeout:   k8sTimeout,
	}

	// Get current namespace if one wasn't specified
	if namespace == "" {
		client.namespace, err = client.getCurrentNamespace()
		if err != nil {
			return nil, fmt.Errorf("could not determine current namespace; %v", err)
		}
	}

	log.WithFields(log.Fields{
		"cli":       cli,
		"flavor":    flavor,
		"version":   k8sVersion.String(),
		"timeout":   client.timeout,
		"namespace": client.namespace,
	}).Debug("Initialized Kubernetes CLI client.")

	return client, nil
}

func discoverKubernetesCLI() (string, error) {
	// Try the OpenShift CLI first
	_, err := exec.Command(CLIOpenShift, "version").CombinedOutput()
	if err == nil {
		if verifyOpenShiftAPIResources() {
			return CLIOpenShift, nil
		}
	}

	// Fall back to the K8S CLI
	out, err := exec.Command(CLIKubernetes, "version").CombinedOutput()
	if err == nil {
		return CLIKubernetes, nil
	}

	return "", fmt.Errorf("could not find the Kubernetes CLI; %s", string(out))
}

func verifyOpenShiftAPIResources() bool {
	out, err := exec.Command("oc", "api-resources").CombinedOutput()
	if err != nil {
		return false
	}

	lines := strings.Split(string(out), "\n")
	for _, l := range lines {
		if strings.Contains(l, "config.openshift.io") {
			return true
		}
	}

	log.Debug("Couldn't find OpenShift api-resources, hence not using oc tools for CLI")
	return false
}

func discoverKubernetesServerVersion(kubernetesCLI string) (*utils.Version, error) {

	cmd := exec.Command(kubernetesCLI, "version", "-o", "json")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	versionBytes, err := ioutil.ReadAll(stdout)
	if err != nil {
		return nil, fmt.Errorf("could not read version data from stdout; %v", err)
	}

	var cliVersion Version
	err = json.Unmarshal(versionBytes, &cliVersion)
	if err != nil {
		return nil, fmt.Errorf("could not parse version data: %s; %v", string(versionBytes), err)
	}

	versionInfo, err := utils.ParseSemantic(cliVersion.ServerVersion.GitVersion)
	return versionInfo, err
}

func discoverOpenShift3ServerVersion(kubernetesCLI string) (*utils.Version, error) {

	cmd := exec.Command(kubernetesCLI, "version")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	inServerSection := false
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Server") {
			inServerSection = true
		} else if inServerSection {
			if strings.HasPrefix(line, "kubernetes ") {
				serverVersion := strings.TrimPrefix(line, "kubernetes ")
				return utils.ParseSemantic(serverVersion)
			}
		}
	}

	return nil, errors.New("could not get OpenShift server version")
}

func (c *KubectlClient) ServerVersion() *utils.Version {
	return c.version
}

func (c *KubectlClient) getCurrentNamespace() (string, error) {

	// Get current namespace from service account info
	cmd := exec.Command(c.cli, "get", "serviceaccount", "default", "-o=json")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}

	var serviceAccount v1.ServiceAccount
	if err := json.NewDecoder(stdout).Decode(&serviceAccount); err != nil {
		return "", err
	}
	if err := cmd.Wait(); err != nil {
		return "", err
	}

	// Get Trident pod name & namespace
	namespace := serviceAccount.ObjectMeta.Namespace

	return namespace, nil
}

// GetSecret get secret object by name
func (c *KubectlClient) GetSecret(secretName string) (*v1.Secret, error) {
	cmdArgs := []string{"get", "secret", secretName, "--namespace", c.namespace, "-o=json"}
	cmd := exec.Command(c.cli, cmdArgs...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	var createdSecret v1.Secret
	if err := json.NewDecoder(stdout).Decode(&createdSecret); err != nil {
		return nil, err
	}
	if err := cmd.Wait(); err != nil {
		return nil, err
	}

	return &createdSecret, nil
}

func (c *KubectlClient) CheckSecretExists(secretName string) (bool, error) {
	args := []string{"get", "secret", secretName, "--namespace", c.namespace, "--ignore-not-found"}
	out, err := exec.Command(c.cli, args...).CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("%s; %v", string(out), err)
	}
	return len(out) > 0, nil
}

func (c *KubectlClient) GetConfigMap(configMapName string) (*v1.ConfigMap, error) {
	cmdArgs := []string{"get", "configmap", configMapName, "--namespace", c.namespace, "-o=json"}
	cmd := exec.Command(c.cli, cmdArgs...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	var createdConfigmap v1.ConfigMap
	if err := json.NewDecoder(stdout).Decode(&createdConfigmap); err != nil {
		return nil, err
	}
	if err := cmd.Wait(); err != nil {
		return nil, err
	}

	return &createdConfigmap, nil
}

func (c *KubectlClient) CheckConfigMapExists(configMapName string) (bool, error) {
	args := []string{"get", "configmap", configMapName, "--namespace", c.namespace, "--ignore-not-found"}
	out, err := exec.Command(c.cli, args...).CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("%s; %v", string(out), err)
	}
	return len(out) > 0, nil
}

func (c *KubectlClient) CreateObjectByYAML(yamlData string) error {
	for _, yamlDocument := range regexp.MustCompile(YAMLSeparator).Split(yamlData, -1) {
		checkCreateObjectByYAML := func() error {
			if returnError := c.createObjectByYAML(yamlDocument); returnError != nil {
				log.WithFields(log.Fields{
					"yamlDocument": yamlDocument,
					"err":          returnError,
				}).Errorf("Object creation failed.")
				return returnError
			}
			return nil
		}

		createObjectNotify := func(err error, duration time.Duration) {
			log.WithFields(log.Fields{
				"yamlDocument": yamlDocument,
				"increment":    duration,
				"err":          err,
			}).Debugf("Object not created, waiting.")
		}
		createObjectBackoff := backoff.NewExponentialBackOff()
		createObjectBackoff.MaxElapsedTime = c.timeout

		log.WithField("yamlDocument", yamlDocument).Trace("Waiting for object to be created.")

		if err := backoff.RetryNotify(checkCreateObjectByYAML, createObjectBackoff, createObjectNotify); err != nil {
			returnError := fmt.Errorf("yamlDocument %s was not created after %3.2f seconds",
				yamlDocument, c.timeout.Seconds())
			return returnError
		}
	}
	return nil
}

func (c *KubectlClient) createObjectByYAML(yaml string) error {

	args := []string{fmt.Sprintf("--namespace=%s", c.namespace), "apply", "-f", "-"}
	cmd := exec.Command(c.cli, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	go func() {
		defer stdin.Close()
		_, _ = stdin.Write([]byte(yaml)) // nolint
	}()

	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s; %v", string(out), err)
	}

	log.Debug("Created Kubernetes object by YAML.")

	return nil
}

func (c *KubectlClient) updateObjectByYAML(yaml string) error {
	args := []string{fmt.Sprintf("--namespace=%s", c.namespace), "apply", "-f", "-"}
	cmd := exec.Command(c.cli, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	go func() {
		defer stdin.Close()
		_, _ = stdin.Write([]byte(yaml)) // nolint
	}()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s; %v", string(out), err)
	}
	log.Debug("Applied changes to Kubernetes object by YAML.")
	return nil
}

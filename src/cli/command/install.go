package command

import (
	"cli/utils"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
	"utils/log"
	"utils/pwd"

	k8sClient "cli/client"
	logging "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
)

// CSIConfig is to store the base config object
type CSIConfig struct {
	Backends []map[string]interface{} `json:"backends"`
}

// CSISecret is to store the secret object
type CSISecret struct {
	Secrets map[string]interface{}  `json:"secrets"`
}

const (
	KubernetesCSIVersionMin = "v1.13.0"

	HUAWEICSIConfigMap      = "huawei-csi-configmap"
	HUAWEICSISecret         = "huawei-csi-secret"
	HUAWEINamespace         = "kube-system"
)

var (
	client k8sClient.Interface
	storageConfig CSIConfig
	storageSecret CSISecret
	storageNamespace string
)

func Init() {
	if len(os.Args) >= inputArgsLength {
		storageNamespace = os.Args[1]
	} else {
		storageNamespace = HUAWEINamespace
	}

	initInstallerLogging()
	processInstallationArguments()
	installSecret()
}


func installSecret() {
	exist, err := client.CheckConfigMapExists(HUAWEICSIConfigMap)
	if err != nil {
		log.Fatalf("Could not find csi config map. Error: %v", err)
	} else if !exist {
		log.Fatalf("The configMap %s does not exist. Please config configMap first.", HUAWEICSIConfigMap)
	}

	exist, err = client.CheckSecretExists(HUAWEICSISecret)
	if err != nil {
		log.Fatalf("Could not find csi secret. Error: %v", err)
	} else if exist {
		fmt.Printf("The secret info is exist. Do you force to update it? Y/N\n")
		input, err := terminal.ReadPassword(0)
		if err != nil {
			log.Fatalf("Input error: %v", err)
		}
		if strings.TrimSpace(strings.ToUpper(string(input))) != "Y" &&
			strings.TrimSpace(strings.ToUpper(string(input))) != "YES" {
			fmt.Printf("The secret already exists and is not updated.")
			os.Exit(0)
		}
	}

	if err := createSecret(); err != nil {
		log.Fatalf("Create secret object error %v. See /var/log/huawei/huawei-csi-install for details.", err)
	}
}

func initClient() (k8sClient.Interface, error) {
	client, err := k8sClient.NewCliClient(storageNamespace, 180*time.Second)
	if err != nil {
		return nil, fmt.Errorf("Could not new a Kubernetes client, err: %v", err)
	}

	return client, nil
}

func initInstallerLogging() {
	// Installer logs to stdout only
	logging.SetOutput(os.Stdout)
	logging.SetFormatter(&logging.TextFormatter{DisableTimestamp: true})

	err := log.Init(map[string]string{
		"logFilePrefix": "huawei-csi-install",
		"logDebug":      "info",
	})
	if err != nil {
		logging.WithField("error", err).Fatal("Failed to initialize logging.")
	}

	logging.WithField("logLevel", logging.GetLevel().String()).Debug("Initialized logging.")
}

func processInstallationArguments() {
	var err error
	if client, err = initClient(); err != nil {
		recordErrorf("could not initialize Kubernetes client; %v", err)
	}

	minOptionalCSIVersion := utils.MustParseSemantic(KubernetesCSIVersionMin)
	if client.ServerVersion().LessThan(minOptionalCSIVersion) {
		recordErrorf("CSI OceanStor requires Kubernetes %s or later.",
			minOptionalCSIVersion.ShortString())
	}
}

func generateKeyText() (string, error) {
	cmd := "head -c32 /dev/urandom | base64"
	shCmd := exec.Command("/bin/sh", "-c", cmd)
	output, err := shCmd.CombinedOutput()
	if err != nil{
		fmt.Printf("Generate random string error: %v", err)
		return "", err
	}
	output = output[:32]
	return string(output), nil
}

func terminalInput(keyText, objectName string) (string, error) {
	var plainText string
	input, err := terminal.ReadPassword(0)
	if err != nil {
		fmt.Printf("Input %s error: %v", objectName, err)
		os.Exit(1)
	}
	plainText = string(input)
	if objectName == "password" {
		encrypted, err := pwd.Encrypt(plainText, keyText)
		if err != nil {
			fmt.Printf("Encrypt storage %s error: %v", objectName, err)
			return "", err
		}
		return encrypted, nil
	}
	return plainText, nil
}

func getPassword(keyText, backendName string) string {
	for {
		fmt.Printf("Enter backend %s's password: \n", backendName)
		inputPassword1, err := terminalInput(keyText, "password")
		if err != nil {
			msg := fmt.Sprintf("Input password error %v, try it again", err)
			log.Errorln(msg)
			continue
		}

		fmt.Println("Please enter the password again: ")
		inputPassword2, err := terminalInput(keyText, "password")
		if err != nil {
			msg := fmt.Sprintf("Input password error %v, try it again", err)
			log.Errorln(msg)
			continue
		}

		if inputPassword1 != inputPassword2 {
			msg := fmt.Sprintln("The two passwords are inconsistent. Please enter again.")
			log.Errorln(msg)
			fmt.Println(msg)
		} else {
			return inputPassword1
		}
	}
}

func generateSecret(backendName string) (map[string]string, error) {
	keyText, err := generateKeyText()
	if err != nil {
		log.Errorf("Generate random string error %v", err)
		return nil, err
	}

	fmt.Printf("Enter backend %s's user: ", backendName)
	inputUser, err := terminalInput(keyText, "user")
	if err != nil {
		log.Errorf("Input user error %v", err)
		return nil, err
	}
	fmt.Printf("%s\n", inputUser)

	inputPassword := getPassword(keyText, backendName)
	secretInfo := map[string]string{
		"user":     inputUser,
		"password": inputPassword,
		"keyText": keyText,
	}
	return secretInfo, nil
}

func createSecret() error {
	// step 1. query the configMap to get the all backend names
	configMap, err := client.GetConfigMap(HUAWEICSIConfigMap)
	if err != nil {
		log.Errorf("failed to get configmap %s. Err: %v", HUAWEICSIConfigMap, err)
		return err
	}

	err = json.Unmarshal([]byte(configMap.Data["csi.json"]), &storageConfig)
	if err != nil {
		log.Errorf("Unmarshal config file %s error: %v", configMap.Data["csi.json"], err)
		return err
	}

	secretMap := make(map[string]string)
	var url interface{}
	var exist bool
	for index, config := range storageConfig.Backends {
		if url, exist = config["urls"].([]interface{}); !exist {
			url, _ = config["url"].(interface{})
		}
		msg := fmt.Sprintf(
			"**************************The %d Backend Info***************************\n" +
			"Current backend name is: %s\n" +
			"Current backend url is: %s\n" +
			"***********************************************************************",
			index+1, config["name"].(string), url)
		fmt.Println(msg)
		log.Infoln(msg)

		secretInfo, err := generateSecret(config["name"].(string))
		if err != nil {
			recordErrorf("generate Secret info error: %v", err)
			return err
		}
		secretBytes, _ := json.Marshal(secretInfo)
		secretMap[config["name"].(string)] = string(secretBytes)
		fmt.Printf("\n")
	}

	// step 2. construct the yaml of the secret
	secretYAML := k8sClient.GetSecretYAML(HUAWEICSISecret, storageNamespace, nil, secretMap)

	// step 3. create the secret
	err = client.CreateObjectByYAML(secretYAML)
	if err != nil {
		log.Errorf("could not create Huawei CSI Secret; secret YAML: %s, err: %v", secretYAML, err)
		return err
	}
	recordInfof("*********************Create CSI Secret Successful**********************\n")
	return nil
}

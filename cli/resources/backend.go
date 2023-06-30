/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2023-2023. All rights reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *       http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package resources

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	k8string "k8s.io/utils/strings"

	"huawei-csi-driver/cli/client"
	"huawei-csi-driver/cli/config"
	"huawei-csi-driver/cli/helper"
	xuanwuV1 "huawei-csi-driver/client/apis/xuanwu/v1"
	"huawei-csi-driver/utils/log"
)

type Backend struct {
	// resource of request
	resource *Resource
}

// NewBackend initialize a Backend instance
func NewBackend(resource *Resource) *Backend {
	return &Backend{resource: resource}
}

// Get query backend resources
func (b *Backend) Get() error {
	storageBackendClaimClient := client.NewCommonCallHandler[xuanwuV1.StorageBackendClaim](config.Client)
	claims, err := storageBackendClaimClient.QueryList(b.resource.namespace, b.resource.names...)
	if err != nil {
		return helper.LogErrorf("query sbc resource failed, error: %v", err)
	}

	if len(claims) == 0 && len(b.resource.names) == 0 {
		helper.PrintNoResourceBackend(b.resource.namespace)
		return nil
	}

	notFoundBackends := getNotFoundBackends(claims, b.resource.names)
	if len(claims) == 0 {
		helper.PrintBackend([]BackendShow{}, notFoundBackends, helper.PrintWithTable[BackendShow])
		return nil
	}

	if b.resource.output == "json" || b.resource.output == "yaml" {
		printFunc := helper.GetPrintFunc[xuanwuV1.StorageBackendClaim](b.resource.output)
		helper.PrintBackend(claims, notFoundBackends, printFunc)
		return nil
	}

	wideShows, err := fetchBackendShows(claims, b.resource.namespace)
	if err != nil {
		return helper.LogErrorf("fetch backend shows failed, error: %v", err)
	}

	if b.resource.output == "wide" {
		helper.PrintBackend(wideShows, notFoundBackends, helper.PrintWithTable[BackendShowWide])
		return nil
	}

	backendShow := helper.MapTo(wideShows, func(wide BackendShowWide) BackendShow {
		return BackendShow{
			Namespace:   wide.Namespace,
			Name:        wide.Name,
			Protocol:    wide.Protocol,
			StorageType: wide.StorageType,
			Sn:          wide.Sn,
			Status:      wide.Status,
			Online:      wide.Online,
			Url:         wide.Url,
		}
	})
	helper.PrintBackend(backendShow, notFoundBackends, helper.PrintWithTable[BackendShow])
	return nil
}

func (b *Backend) Delete() error {
	storageBackendClaimClient := client.NewCommonCallHandler[xuanwuV1.StorageBackendClaim](config.Client)
	claims, err := storageBackendClaimClient.QueryList(b.resource.namespace, b.resource.names...)
	if err != nil {
		return err
	}
	if len(claims) == 0 {
		helper.PrintNotFoundBackend(b.resource.names...)
		return nil
	}

	var deleteResult []string
	for _, claim := range claims {
		if err := deleteSbcReferenceResources(claim); err != nil {
			return helper.LogErrorf("delete backend reference resource failed, error: %v", err)
		}

		helper.PrintOperateResult("backend", "deleted", claim.Name)
		deleteResult = append(deleteResult, claim.Name)
	}

	notFoundBackends := getNotFoundBackends(claims, b.resource.names)
	helper.PrintNotFoundBackend(notFoundBackends...)
	return nil
}

// Update update backend
func (b *Backend) Update() error {
	storageBackendClaimClient := client.NewCommonCallHandler[xuanwuV1.StorageBackendClaim](config.Client)
	oldClaim, err := storageBackendClaimClient.QueryByName(b.resource.namespace, b.resource.names[0])
	if err != nil {
		return err
	}

	if reflect.DeepEqual(oldClaim, xuanwuV1.StorageBackendClaim{}) {
		helper.PrintNotFoundBackend(b.resource.names[0])
		return nil
	}

	nameWithUUid := helper.AppendUid(oldClaim.Name, config.DefaultUidLength)
	if err = createSecretWithUid(oldClaim, nameWithUUid); err != nil {
		return err
	}

	secretClient := client.NewCommonCallHandler[corev1.Secret](config.Client)
	newClaim := oldClaim.DeepCopy()
	newClaim.Spec.SecretMeta = k8string.JoinQualifiedName(newClaim.Namespace, nameWithUUid)

	if err = storageBackendClaimClient.Update(*newClaim); err != nil {
		if err := storageBackendClaimClient.Update(oldClaim); err != nil {
			log.Errorf("apply storageBackendClaim failed, error: %v", err)
		}

		if err := secretClient.DeleteByNames(newClaim.Namespace, nameWithUUid); err != nil {
			log.Errorf("delete new created secret failed, error: %v", err)
		}
		return err
	}

	_, oldSecretName := k8string.SplitQualifiedName(oldClaim.Spec.SecretMeta)
	if err := secretClient.DeleteByNames(newClaim.Namespace, oldSecretName); err != nil {
		log.Errorf("delete old created secret failed, error: %v", err)
	}

	// print update result
	helper.PrintOperateResult("backend", "updated", b.resource.names[0])
	return nil
}

func createSecretWithUid(claim xuanwuV1.StorageBackendClaim, uuid string) error {
	backendConfig := &BackendConfiguration{
		Name:      uuid,
		NameSpace: claim.Namespace,
	}

	secretConfig, err := backendConfig.ToSecretConfig()
	if err != nil {
		return err
	}

	secretClient := client.NewCommonCallHandler[corev1.Secret](config.Client)
	return secretClient.Create(secretConfig.ToSecret())
}

func getNotFoundBackends(queryResult []xuanwuV1.StorageBackendClaim, queryNames []string) []string {
	if len(queryNames) == len(queryResult) {
		return []string{}
	}

	notFoundBackends := make([]string, len(queryNames))
	copy(notFoundBackends, queryNames)
	for _, claim := range queryResult {
		notFoundBackends = removeExistBackend(notFoundBackends, claim.Name)
	}
	return notFoundBackends
}

func fetchBackendShows(claims []xuanwuV1.StorageBackendClaim, namespace string) ([]BackendShowWide, error) {
	var contentNames []string
	var configmapNames []string

	for _, claim := range claims {
		if claim.Status != nil {
			contentNames = append(contentNames, claim.Status.BoundContentName)
		}
		_, configName := k8string.SplitQualifiedName(claim.Spec.ConfigMapMeta)
		configmapNames = append(configmapNames, configName)
	}

	storageBackendContentClient := client.NewCommonCallHandler[xuanwuV1.StorageBackendContent](config.Client)
	contentList, err := storageBackendContentClient.QueryList(namespace, contentNames...)
	if err != nil {
		return nil, err
	}

	backendConfigs, err := FetchBackendConfig(namespace, configmapNames...)
	if err != nil {
		return nil, err
	}

	return buildBackendShow(claims, contentList, backendConfigs), nil
}

func buildBackendShow(claims []xuanwuV1.StorageBackendClaim, contentList []xuanwuV1.StorageBackendContent,
	config map[string]*BackendConfiguration) []BackendShowWide {
	var contentMapping = make(map[string]xuanwuV1.StorageBackendContent)
	for _, content := range contentList {
		contentMapping[content.Name] = content
	}

	var result []BackendShowWide
	for _, claim := range claims {
		item := &BackendShowWide{}
		item.ShowWithClaimOption(claim)
		if claim.Status != nil {
			if content, ok := contentMapping[claim.Status.BoundContentName]; ok {
				item.ShowWithContentOption(content)
			}
		}

		_, name := k8string.SplitQualifiedName(claim.Spec.ConfigMapMeta)
		if configuration, ok := config[name]; ok {
			item.ShowWithConfigOption(*configuration)
		}

		result = append(result, *item)
	}

	return result
}

func deleteSbcReferenceResources(claim xuanwuV1.StorageBackendClaim) error {
	_, secretName := k8string.SplitQualifiedName(claim.Spec.SecretMeta)
	_, configmapName := k8string.SplitQualifiedName(claim.Spec.ConfigMapMeta)

	referenceResources := []string{
		k8string.JoinQualifiedName(string(client.Secret), secretName),
		k8string.JoinQualifiedName(string(client.ConfigMap), configmapName),
		k8string.JoinQualifiedName(string(client.Storagebackendclaim), claim.Name),
	}

	_, err := config.Client.DeleteResourceByQualifiedNames(referenceResources, claim.Namespace)
	return err
}

func removeExistBackend(allBackend []string, backendName string) []string {
	index := 0
	for _, name := range allBackend {
		if backendName != name {
			allBackend[index] = name
			index++
		}
	}
	return allBackend[:index]
}

// FetchBackendConfig fetch backend config from configmap
func FetchBackendConfig(namespace string, names ...string) (map[string]*BackendConfiguration, error) {
	configMapClient := client.NewCommonCallHandler[corev1.ConfigMap](config.Client)
	configMapList, err := configMapClient.QueryList(namespace, names...)
	if err != nil {
		return map[string]*BackendConfiguration{}, err
	}

	result := make(map[string]*BackendConfiguration)
	for _, configMap := range configMapList {
		backendConfig, err := LoadBackendsFromConfigMap(configMap)
		if err != nil {
			return result, err
		}
		for _, configuration := range backendConfig {
			configuration.Configured = true
			result[configuration.Name] = configuration
		}
	}
	return result, nil
}

func (b *Backend) Create() error {
	creatingBackends, err := b.LoadBackendFile()
	if err != nil {
		return helper.LogErrorf("load backend failed: error: %v", err)
	}
	notConfiguredBackends, err := b.preProcessBackend(creatingBackends)
	if err != nil {
		return helper.LogErrorf("pre process backend failed: error: %v", err)
	}

	configuredBackends, err := FetchConfiguredBackends(b.resource.namespace)
	if err != nil {
		return helper.LogErrorf("fetch configured backend failed: error: %v", err)
	}

	backends := MergeBackends(notConfiguredBackends, configuredBackends)
	for {
		selectedBackend, err := selectOneBackend(backends)
		if err != nil {
			break
		}

		if selectedBackend.Configured {
			fmt.Printf("backend [%s] has been Configured, please select another\n", selectedBackend.Name)
			continue
		}

		if err := ConfigOneBackend(selectedBackend); err != nil {
			fmt.Printf("failed to configure the backend account. %v\n", err)
			continue
		}

		selectedBackend.Configured = true
	}
	return nil
}

func FetchConfiguredBackends(namespace string) (map[string]*BackendConfiguration, error) {
	storageBackendClaimClient := client.NewCommonCallHandler[xuanwuV1.StorageBackendClaim](config.Client)
	sbcList, err := storageBackendClaimClient.QueryList(namespace)
	if err != nil || len(sbcList) == 0 {
		return map[string]*BackendConfiguration{}, err
	}

	configuredBackend := helper.MapTo(sbcList, func(claim xuanwuV1.StorageBackendClaim) string {
		_, name := k8string.SplitQualifiedName(claim.Spec.ConfigMapMeta)
		return name
	})

	return FetchBackendConfig(namespace, configuredBackend...)
}

func MergeBackends(notConfigured, configured map[string]*BackendConfiguration) []*BackendConfiguration {
	var backends []*BackendConfiguration
	for name, configuration := range configured {
		if _, ok := notConfigured[name]; ok {
			delete(notConfigured, name)
		}
		configuration.Configured = true
		backends = append(backends, configuration)
	}
	for _, configuration := range notConfigured {
		backends = append(backends, configuration)
	}

	return backends
}

// ConfigOneBackend config one backend
func ConfigOneBackend(backendConfig *BackendConfiguration) error {
	claimConfig := backendConfig.ToStorageBackendClaimConfig()
	claim := claimConfig.ToStorageBackendClaim()

	var err error
	defer func() {
		if err != nil {
			// Roll back the remnants of this failed creation
			if err := deleteSbcReferenceResources(claim); err != nil {
				log.Errorf("roll back delete backend reference resource failed, error: %v", err)
			}
		}
	}()

	// remove residual of failed history creation
	if err = deleteSbcReferenceResources(claim); err != nil {
		return helper.LogErrorf("remove residual resource failed, error: %v", err)
	}

	// create configmap resource
	mapConfig, err := backendConfig.ToConfigMapConfig()
	if err != nil {
		return err
	}
	configMapClient := client.NewCommonCallHandler[corev1.ConfigMap](config.Client)
	if err = configMapClient.Create(mapConfig.ToConfigMap()); err != nil {
		return err
	}

	// get input account info
	secretConfig, err := backendConfig.ToSecretConfig()
	if err != nil {
		return err
	}

	// create secret resource
	secretClient := client.NewCommonCallHandler[corev1.Secret](config.Client)
	if err = secretClient.Create(secretConfig.ToSecret()); err != nil {
		return err
	}

	// create storageBackendClaim resource
	storageBackendClaimClient := client.NewCommonCallHandler[xuanwuV1.StorageBackendClaim](config.Client)
	if err = storageBackendClaimClient.Create(claim); err != nil {
		return err
	}

	// out create success tips
	helper.PrintResult(fmt.Sprintf("Backend %s is configured\n", backendConfig.Name))
	return nil
}

func selectOneBackend(backendList []*BackendConfiguration) (*BackendConfiguration, error) {
	printBackendsStatusTable(backendList)
	number, err := helper.GetSelectedNumber("Please enter the backend number to configure "+
		"(Enter 'exit' to exit):", len(backendList))
	if err != nil {
		log.Errorf("failed to get backend number entered by user. %v", err)
		return nil, err
	}

	if number > len(backendList) {
		number = len(backendList)
	}
	return backendList[number-1], nil
}

func (b *Backend) LoadBackendFile() (map[string]*BackendConfiguration, error) {
	data, err := os.ReadFile(b.resource.fileName)
	if err != nil {
		return nil, err
	}

	switch b.resource.fileType {
	case "json":
		return LoadBackendsFromJson(data)
	case "yaml":
		return LoadBackendsFromYaml(data)
	default:
		return nil, errors.New(fmt.Sprintf("file type [%s] is not supported", b.resource.fileType))
	}
}

func (b *Backend) preProcessBackend(backends map[string]*BackendConfiguration) (map[string]*BackendConfiguration,
	error) {
	initFunc := []func(*BackendConfiguration){
		b.setBackendNamespace,
		b.setProvisioner,
		b.setMaxClients,
	}

	result := make(map[string]*BackendConfiguration)
	for _, configuration := range backends {
		mappingName, err := b.processBackendName(configuration)
		if err != nil {
			return map[string]*BackendConfiguration{}, err
		}
		for _, init := range initFunc {
			init(configuration)
		}
		result[mappingName] = configuration
	}

	return result, nil
}
func (b *Backend) processBackendName(configuration *BackendConfiguration) (string, error) {
	if configuration.Name == "" {
		return "", errors.New("backend name cannot be empty, please check your backend file")
	}

	mappingName := configuration.Name
	if b.resource.fileType == "json" {
		config.NotValidateName = true
	}

	isDNSFormat := helper.IsDNSFormat(configuration.Name)
	errFormat := "backend name [%s] must consist of lower case alphanumeric characters or '-'," +
		" and must start and end with an alphanumeric character(e.g. 'backend-nfs'), and must be no more than" +
		" %d characters."
	if !config.NotValidateName && !isDNSFormat {
		errMsg := fmt.Sprintf(errFormat, configuration.Name, helper.BackendNameMaxLength)
		return mappingName, errors.New(errMsg)
	}

	if config.NotValidateName && !isDNSFormat {
		mappingName = helper.BuildBackendName(configuration.Name)
		configuration.Name = mappingName
	}

	if !helper.IsDNSFormat(configuration.Name) {
		errMsg := fmt.Sprintf(errFormat, configuration.Name, helper.BackendNameMaxLength)
		return mappingName, errors.New(errMsg)
	}

	return mappingName, nil
}

func (b *Backend) setBackendNamespace(configuration *BackendConfiguration) {
	// Prioritize the namespace specified by the command
	if config.Namespace != "" {
		configuration.NameSpace = config.Namespace
		return
	}

	// If not set, use the default namespace
	if configuration.NameSpace == "" {
		configuration.NameSpace = config.DefaultNamespace
	}
	b.resource.NamespaceParam(configuration.NameSpace)
}

func (b *Backend) setMaxClients(configuration *BackendConfiguration) {
	// If not set, use the default max clients num
	if configuration.MaxClientThreads == "" {
		configuration.MaxClientThreads = config.DefaultMaxClientThreads
	}
}

func (b *Backend) setProvisioner(configuration *BackendConfiguration) {
	// Prioritize the provisioner specified by the command
	if config.Provisioner != "" {
		configuration.Provisioner = config.Provisioner
		return
	}

	// If not set, use the default provisioner
	if configuration.Provisioner == "" {
		configuration.Provisioner = config.DefaultProvisioner
	}
}

func printBackendsStatusTable(statusList []*BackendConfiguration) {
	var shows []BackendConfigShow
	for i, configuration := range statusList {
		shows = append(shows, BackendConfigShow{
			Number:     strconv.Itoa(i + 1),
			Storage:    configuration.Storage,
			Name:       configuration.Name,
			Urls:       strings.Join(configuration.Urls, ";"),
			Configured: strconv.FormatBool(configuration.Configured),
		})
	}

	helper.PrintWithTable(shows)
}

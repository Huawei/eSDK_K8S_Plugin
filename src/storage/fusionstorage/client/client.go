package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"sync"
	"time"
	"utils"
	"utils/log"

	fusionURL "net/url"
)

const (
	NO_AUTHENTICATED        int64 = 10000003
	VOLUME_NAME_NOT_EXIST   int64 = 50150005
	DELETE_VOLUME_NOT_EXIST int64 = 32150005
	QUERY_VOLUME_NOT_EXIST  int64 = 31000000
	INITIATOR_NOT_EXIST     int64 = 50155103
	HOSTNAME_ALREADY_EXIST  int64 = 50157019
	INITIATOR_ALREADY_EXIST int64 = 50155102
	INITIATOR_ADDED_TO_HOST int64 = 50157021
	OFF_LINE_CODE                 = "1077949069"
	OFF_LINE_CODE_INT       int64 = 1077949069
	CLIENT_ALREADY_EXIST    int64 = 1077939727
	SNAPSHOT_NOT_EXIST      int64 = 50150006
	FILE_SYSTEM_NOT_EXIST   int64 = 33564678
	QUOTA_NOT_EXIST         int64 = 37767685
	DEFAULT_PARALLEL_COUNT  int   = 50
	MAX_PARALLEL_COUNT      int   = 1000
	MIN_PARALLEL_COUNT      int   = 20
)

var (
	LOG_FILTER = map[string]map[string]bool{
		"POST": {
			"/dsware/service/v1.3/sec/login":     true,
			"/dsware/service/v1.3/sec/keepAlive": true,
		},
		"GET": {
			"/dsware/service/v1.3/storagePool": true,
		},
	}
	clientSemaphore *utils.Semaphore
)

func logFilter(method, url string) bool {
	filter, exist := LOG_FILTER[method]
	return exist && filter[url]
}

type Client struct {
	url       string
	user      string
	password  string
	authToken string
	client    *http.Client

	reloginMutex sync.Mutex
}

func NewClient(url, user, password, parallelNum string) *Client {
	var err error
	var parallelCount int

	if len(parallelNum) > 0 {
		parallelCount, err = strconv.Atoi(parallelNum)
		if err != nil || parallelCount > MAX_PARALLEL_COUNT || parallelCount < MIN_PARALLEL_COUNT {
			log.Warningf("The config parallelNum %d is invalid, set it to the default value %d", parallelCount, DEFAULT_PARALLEL_COUNT)
			parallelCount = DEFAULT_PARALLEL_COUNT
		}
	} else {
		parallelCount = DEFAULT_PARALLEL_COUNT
	}

	log.Infof("Init parallel count is %d", parallelCount)
	clientSemaphore = utils.NewSemaphore(parallelCount)
	return &Client{
		url:      url,
		user:     user,
		password: password,
	}
}

func (cli *Client) DuplicateClient() *Client {
	dup := *cli
	dup.client = nil

	return &dup
}

func (cli *Client) Login() error {
	jar, _ := cookiejar.New(nil)
	cli.client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Jar:     jar,
		Timeout: 60 * time.Second,
	}

	log.Infof("Try to login %s.", cli.url)

	data := map[string]interface{}{
		"userName": cli.user,
		"password": cli.password,
	}

	respHeader, resp, err := cli.call("POST", "/dsware/service/v1.3/sec/login", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return fmt.Errorf("login %s error: %+v", cli.url, resp)
	}

	cli.authToken = respHeader["X-Auth-Token"][0]
	log.Infof("Login %s success", cli.url)

	return nil
}

func (cli *Client) Logout() {
	defer func() {
		cli.authToken = ""
		cli.client = nil
	}()

	if cli.client == nil {
		return
	}

	resp, err := cli.post("/dsware/service/v1.3/sec/logout", nil)
	if err != nil {
		log.Warningf("Logout %s error: %v", cli.url, err)
		return
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		log.Warningf("Logout %s error: %d", cli.url, result)
		return
	}

	log.Infof("Logout %s success.", cli.url)
}

func (cli *Client) KeepAlive() {
	_, err := cli.post("/dsware/service/v1.3/sec/keepAlive", nil)
	if err != nil {
		log.Warningf("Keep token alive error: %v", err)
	}
}

func (cli *Client) doCall(method string, url string, data map[string]interface{}) (http.Header, []byte, error) {
	var err error
	var reqUrl string
	var reqBody io.Reader
	var respBody []byte

	if data != nil {
		reqBytes, err := json.Marshal(data)
		if err != nil {
			log.Errorf("json.Marshal data %v error: %v", data, err)
			return nil, nil, err
		}

		reqBody = bytes.NewReader(reqBytes)
	}
	reqUrl = cli.url + url

	req, err := http.NewRequest(method, reqUrl, reqBody)
	if err != nil {
		log.Errorf("Construct http request error: %v", err)
		return nil, nil, err
	}

	req.Header.Set("Referer", cli.url)
	req.Header.Set("Content-Type", "application/json")

	if url != "/dsware/service/v1.3/sec/login" && url != "/dsware/service/v1.3/sec/logout" {
		cli.reloginMutex.Lock()
		if cli.authToken != "" {
			req.Header.Set("X-Auth-Token", cli.authToken)
		}
		cli.reloginMutex.Unlock()
	} else {
		if cli.authToken != "" {
			req.Header.Set("X-Auth-Token", cli.authToken)
		}
	}

	if !logFilter(method, url) {
		log.Infof("Request method: %s, url: %s, body: %v", method, reqUrl, data)
	}

	clientSemaphore.Acquire()
	defer clientSemaphore.Release()

	resp, err := cli.client.Do(req)
	if err != nil {
		log.Errorf("Send request method: %s, url: %s, error: %v", method, reqUrl, err)
		return nil, nil, errors.New("unconnected")
	}

	defer resp.Body.Close()

	respBody, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("Read response data error: %v", err)
		return nil, nil, err
	}

	if !logFilter(method, url) {
		log.Infof("Response method: %s, url: %s, body: %s", method, reqUrl, respBody)
	}

	return resp.Header, respBody, nil
}

func (cli *Client) call(method string, url string, data map[string]interface{}) (http.Header, map[string]interface{}, error) {
	var body map[string]interface{}

	respHeader, respBody, err := cli.doCall(method, url, data)

	if err != nil {
		if err.Error() == "unconnected" {
			goto RETRY
		}

		return nil, nil, err
	}

	err = json.Unmarshal(respBody, &body)
	if err != nil {
		log.Errorf("Unmarshal response body %s error: %v", respBody, err)
		return nil, nil, err
	}

	if errorCode, ok := body["errorCode"].(string); ok && errorCode == OFF_LINE_CODE {
		log.Warningf("User offline, try to relogin %s", cli.url)
		goto RETRY
	}

	// Compatible with int error code 1077949069
	if errorCode, ok := body["errorCode"].(float64); ok && int64(errorCode) == OFF_LINE_CODE_INT {
		log.Warningf("User offline, try to relogin %s", cli.url)
		goto RETRY
	}

	// Compatible with FusionStorage 6.3
	if errorCode, ok := body["errorCode"].(float64); ok && int64(errorCode) == NO_AUTHENTICATED {
		log.Warningf("User offline, try to relogin %s", cli.url)
		goto RETRY
	}

	return respHeader, body, nil

RETRY:
	err = cli.reLogin()
	if err == nil {
		respHeader, respBody, err = cli.doCall(method, url, data)
	}

	if err != nil {
		return nil, nil, err
	}

	err = json.Unmarshal(respBody, &body)
	if err != nil {
		log.Errorf("Unmarshal response body %s error: %v", respBody, err)
		return nil, nil, err
	}

	return respHeader, body, nil
}

func (cli *Client) reLogin() error {
	oldToken := cli.authToken

	cli.reloginMutex.Lock()
	defer cli.reloginMutex.Unlock()
	if cli.authToken != "" && oldToken != cli.authToken {
		// Coming here indicates other thread had already done relogin, so no need to relogin again
		return nil
	} else if cli.authToken != "" {
		cli.Logout()
	}

	err := cli.Login()
	if err != nil {
		log.Errorf("Try to relogin error: %v", err)
		return err
	}

	return nil
}

func (cli *Client) get(url string, data map[string]interface{}) (map[string]interface{}, error) {
	_, body, err := cli.call("GET", url, data)
	return body, err
}

func (cli *Client) post(url string, data map[string]interface{}) (map[string]interface{}, error) {
	_, body, err := cli.call("POST", url, data)
	return body, err
}

func (cli *Client) put(url string, data map[string]interface{}) (map[string]interface{}, error) {
	_, body, err := cli.call("PUT", url, data)
	return body, err
}

func (cli *Client) delete(url string, data map[string]interface{}) (map[string]interface{}, error) {
	_, body, err := cli.call("DELETE", url, data)
	return body, err
}

func (cli *Client) CreateVolume(params map[string]interface{}) error {
	data := map[string]interface{}{
		"volName": params["name"].(string),
		"volSize": params["capacity"].(int64),
		"poolId":  params["poolId"].(int64),
	}

	resp, err := cli.post("/dsware/service/v1.3/volume/create", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		errorCode, _ := resp["errorCode"].(string)
		return fmt.Errorf("Create volume %v error: %s", data, errorCode)
	}

	return nil
}

func (cli *Client) GetVolumeByName(name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/dsware/service/v1.3/volume/queryByName?volName=%s", name)
	resp, err := cli.get(url, nil)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		errorCode, _ := resp["errorCode"].(float64)
		if int64(errorCode) == VOLUME_NAME_NOT_EXIST {
			log.Warningf("Volume of name %s doesn't exist", name)
			return nil, nil
		}

		// Compatible with FusionStorage 6.3
		if int64(errorCode) == QUERY_VOLUME_NOT_EXIST {
			log.Warningf("Volume of name %s doesn't exist", name)
			return nil, nil
		}

		return nil, fmt.Errorf("Get volume by name %s error: %d", name, int64(errorCode))
	}

	lun, ok := resp["lunDetailInfo"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	return lun, nil
}

func (cli *Client) DeleteVolume(name string) error {
	data := map[string]interface{}{
		"volNames": []string{name},
	}

	resp, err := cli.post("/dsware/service/v1.3/volume/delete", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		details, ok := resp["detail"].([]interface{})
		if !ok || len(details) == 0 {
			msg := fmt.Sprintf("There is no detail info in response %v.", resp)
			log.Errorln(msg)
			return errors.New(msg)
		}

		detail, ok := details[0].(map[string]interface{})
		if !ok {
			msg := fmt.Sprintf("The format of detail info %v is not map[string]interface{}.", details)
			log.Errorln(msg)
			return errors.New(msg)
		}

		floatCode, ok := detail["errorCode"].(float64)
		if !ok {
			msg := fmt.Sprintf("There is no error code in detail %v.", detail)
			log.Errorln(msg)
			return errors.New(msg)
		}

		errorCode := int64(floatCode)
		if errorCode == VOLUME_NAME_NOT_EXIST {
			log.Warningf("Volume %s doesn't exist while deleting.", name)
			return nil
		}

		// Compatible with FusionStorage 6.3
		if errorCode == DELETE_VOLUME_NOT_EXIST {
			log.Warningf("Volume %s doesn't exist while deleting.", name)
			return nil
		}

		return fmt.Errorf("Delete volume %s error: %d", name, errorCode)
	}

	return nil
}

func (cli *Client) AttachVolume(name, ip string) error {
	data := map[string]interface{}{
		"volName": []string{name},
		"ipList":  []string{ip},
	}

	resp, err := cli.post("/dsware/service/v1.3/volume/attach", data)
	if err != nil {
		return err
	}

	result, ok := resp[name].([]interface{})
	if !ok || len(result) == 0 {
		return fmt.Errorf("Attach volume %s to %s error", name, ip)
	}

	attachResult, ok := result[0].(map[string]interface{})
	if !ok {
		return fmt.Errorf("attach volume %s to %s error", name, ip)
	}

	errorCode, exist := attachResult["errorCode"].(string)
	if !exist || errorCode != "0" {
		return fmt.Errorf("Attach volume %s to %s error: %s", name, ip, errorCode)
	}

	return nil
}

func (cli *Client) DetachVolume(name, ip string) error {
	data := map[string]interface{}{
		"volName": []string{name},
		"ipList":  []string{ip},
	}

	resp, err := cli.post("/dsware/service/v1.3/volume/detach", data)
	if err != nil {
		return err
	}

	result, ok := resp["volumeInfo"].([]interface{})
	if !ok || len(result) == 0 {
		return fmt.Errorf("Detach volume %s from %s error", name, ip)
	}

	detachResult, ok := result[0].(map[string]interface{})
	if !ok {
		return fmt.Errorf("detach volume %s from %s error", name, ip)
	}

	errorCode, exist := detachResult["errorCode"].(string)
	if !exist || errorCode != "0" {
		return fmt.Errorf("Detach volume %s from %s error: %s", name, ip, errorCode)
	}

	return nil
}

func (cli *Client) GetPoolByName(poolName string) (map[string]interface{}, error) {
	resp, err := cli.get("/dsware/service/v1.3/storagePool", nil)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return nil, fmt.Errorf("Get all pools error: %d", result)
	}

	storagePools, exist := resp["storagePools"].([]interface{})
	if !exist || len(storagePools) <= 0 {
		return nil, nil
	}

	for _, p := range storagePools {
		pool, ok := p.(map[string]interface{})
		if !ok {
			msg := fmt.Sprintf("The pool %v's format is not map[string]interface{}", p)
			log.Errorln(msg)
			return nil, errors.New(msg)
		}

		if pool["poolName"].(string) == poolName {
			return pool, nil
		}
	}

	return nil, nil
}

func (cli *Client) GetPoolById(poolId int64) (map[string]interface{}, error) {
	url := fmt.Sprintf("/dsware/service/v1.3/storagePool?poolId=%d", poolId)
	resp, err := cli.get(url, nil)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return nil, fmt.Errorf("get pool by id %d error: %d", poolId, result)
	}

	storagePools, exist := resp["storagePools"].([]interface{})
	if !exist || len(storagePools) <= 0 {
		return nil, nil
	}

	for _, p := range storagePools {
		pool, ok := p.(map[string]interface{})
		if !ok {
			msg := fmt.Sprintf("The pool %v's format is not map[string]interface{}", p)
			log.Errorln(msg)
			return nil, errors.New(msg)
		}

		if int64(pool["poolId"].(float64)) == poolId {
			return pool, nil
		}
	}

	return nil, nil
}

func (cli *Client) GetAllPools() (map[string]interface{}, error) {
	resp, err := cli.get("/dsware/service/v1.3/storagePool", nil)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return nil, fmt.Errorf("Get all pools error: %d", result)
	}

	storagePools, exist := resp["storagePools"].([]interface{})
	if !exist || len(storagePools) <= 0 {
		return nil, nil
	}

	pools := make(map[string]interface{})

	for _, p := range storagePools {
		pool, ok := p.(map[string]interface{})
		if !ok {
			msg := fmt.Sprintf("The pool %v's format is not map[string]interface{}", p)
			log.Errorln(msg)
			return nil, errors.New(msg)
		}
		name := pool["poolName"].(string)
		pools[name] = pool
	}

	return pools, nil
}

func (cli *Client) CreateSnapshot(snapshotName, volName string) error {
	data := map[string]interface{}{
		"volName":      volName,
		"snapshotName": snapshotName,
	}

	resp, err := cli.post("/dsware/service/v1.3/snapshot/create", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return fmt.Errorf("Create snapshot %s of volume %s error: %d", snapshotName, volName, result)
	}

	return nil
}

func (cli *Client) DeleteSnapshot(snapshotName string) error {
	data := map[string]interface{}{
		"snapshotName": snapshotName,
	}

	resp, err := cli.post("/dsware/service/v1.3/snapshot/delete", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return fmt.Errorf("Delete snapshot %s error: %d", snapshotName, result)
	}

	return nil
}

func (cli *Client) GetSnapshotByName(snapshotName string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/dsware/service/v1.3/snapshot/queryByName?snapshotName=%s", snapshotName)
	resp, err := cli.get(url, nil)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		errorCode, _ := resp["errorCode"].(float64)
		if int64(errorCode) == SNAPSHOT_NOT_EXIST {
			log.Warningf("Snapshot of name %s doesn't exist", snapshotName)
			return nil, nil
		}

		return nil, fmt.Errorf("get snapshot by name %s error: %d", snapshotName, result)
	}

	snapshot, ok := resp["snapshot"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	return snapshot, nil
}

func (cli *Client) CreateVolumeFromSnapshot(volName string, volSize int64, snapshotName string) error {
	data := map[string]interface{}{
		"volName": volName,
		"volSize": volSize,
		"src":     snapshotName,
	}

	resp, err := cli.post("/dsware/service/v1.3/snapshot/volume/create", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return fmt.Errorf("Create volume %s from snapshot %s error: %d", volName, snapshotName, result)
	}

	return nil
}

func (cli *Client) GetHostByName(hostName string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"hostName": hostName,
	}

	resp, err := cli.get("/dsware/service/iscsi/queryAllHost", data)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return nil, fmt.Errorf("Get host of name %s error: %d", hostName, result)
	}

	hostList, exist := resp["hostList"].([]interface{})
	if !exist {
		log.Infof("Host %s does not exist", hostName)
		return nil, nil
	}

	for _, i := range hostList {
		host, ok := i.(map[string]interface{})
		if !ok {
			msg := fmt.Sprintf("The host %v's format is not map[string]interface{}", i)
			log.Errorln(msg)
			return nil, errors.New(msg)
		}
		if host["hostName"] == hostName {
			return host, nil
		}
	}

	return nil, nil
}

func (cli *Client) CreateHost(hostName string, alua map[string]interface{}) error {
	data := map[string]interface{}{
		"hostName": hostName,
	}

	if switchoverMode, ok := alua["switchoverMode"]; ok {
		data["switchoverMode"] = switchoverMode
	}

	if pathType, ok := alua["pathType"]; ok {
		data["pathType"] = pathType
	}

	resp, err := cli.post("/dsware/service/iscsi/createHost", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		if !cli.checkErrorCode(resp, HOSTNAME_ALREADY_EXIST) {
			return fmt.Errorf("Create host %s error", hostName)
		}
	}

	return nil
}

func (cli *Client) UpdateHost(hostName string, alua map[string]interface{}) error {
	data := map[string]interface{}{
		"hostName": hostName,
	}

	if switchoverMode, ok := alua["switchoverMode"]; ok {
		data["switchoverMode"] = switchoverMode
	}

	if pathType, ok := alua["pathType"]; ok {
		data["pathType"] = pathType
	}

	resp, err := cli.post("/dsware/service/iscsi/modifyHost", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return fmt.Errorf("update host %s by %v error", hostName, data)
	}

	return nil
}

func (cli *Client) GetInitiatorByName(name string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"portName": name,
	}

	resp, err := cli.post("/dsware/service/iscsi/queryPortInfo", data)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		if !cli.checkErrorCode(resp, INITIATOR_NOT_EXIST) {
			return nil, fmt.Errorf("Get initiator %s error", name)
		}

		log.Infof("Initiator %s does not exist", name)
		return nil, nil
	}

	portList, exist := resp["portList"].([]interface{})
	if !exist || len(portList) == 0 {
		log.Infof("Initiator %s does not exist", name)
		return nil, nil
	}

	return portList[0].(map[string]interface{}), nil
}

func (cli *Client) QueryHostByPort(port string) (string, error) {
	data := map[string]interface{}{
		"portName": []string{port},
	}

	resp, err := cli.post("/dsware/service/iscsi/queryHostByPort", data)
	if err != nil {
		return "", err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		if !cli.checkErrorCode(resp, INITIATOR_NOT_EXIST) {
			return "", fmt.Errorf("Get host initiator %s belongs error", port)
		}

		log.Infof("Initiator %s does not belong to any host", port)
		return "", nil
	}

	portHostMap, exist := resp["portHostMap"].(map[string]interface{})
	if !exist {
		log.Infof("Initiator %s does not belong to any host", port)
		return "", nil
	}

	hosts, exist := portHostMap[port].([]interface{})
	if !exist || len(hosts) == 0 {
		log.Infof("Initiator %s does not belong to any host", port)
		return "", nil
	}

	return hosts[0].(string), nil
}

func (cli *Client) CreateInitiator(name string) error {
	data := map[string]interface{}{
		"portName": name,
	}

	resp, err := cli.post("/dsware/service/iscsi/createPort", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		if !cli.checkErrorCode(resp, INITIATOR_ALREADY_EXIST) {
			return fmt.Errorf("Create initiator %s error", name)
		}
	}

	return nil
}

func (cli *Client) AddPortToHost(initiatorName, hostName string) error {
	data := map[string]interface{}{
		"hostName":  hostName,
		"portNames": []string{initiatorName},
	}

	resp, err := cli.post("/dsware/service/iscsi/addPortToHost", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		if !cli.checkErrorCode(resp, INITIATOR_ADDED_TO_HOST) {
			return fmt.Errorf("Add initiator %s to host %s error", initiatorName, hostName)
		}
	}

	return nil
}

func (cli *Client) AddLunToHost(lunName, hostName string) error {
	data := map[string]interface{}{
		"hostName": hostName,
		"lunNames": []string{lunName},
	}

	resp, err := cli.post("/dsware/service/iscsi/addLunsToHost", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return fmt.Errorf("Add lun %s to host %s error: %d", lunName, hostName, result)
	}

	return nil
}

func (cli *Client) DeleteLunFromHost(lunName, hostName string) error {
	data := map[string]interface{}{
		"hostName": hostName,
		"lunNames": []string{lunName},
	}

	resp, err := cli.post("/dsware/service/iscsi/deleteLunFromHost", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return fmt.Errorf("Delete lun %s from host %s error: %d", lunName, hostName, result)
	}

	return nil
}

func (cli *Client) QueryIscsiPortal() ([]map[string]interface{}, error) {
	data := make(map[string]interface{})
	resp, err := cli.post("/dsware/service/cluster/dswareclient/queryIscsiPortal", data)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return nil, fmt.Errorf("Query iscsi portal error: %d", result)
	}

	var nodeResultList []map[string]interface{}

	respData, exist := resp["nodeResultList"].([]interface{})
	if exist {
		for _, i := range respData {
			nodeResultList = append(nodeResultList, i.(map[string]interface{}))
		}
	}

	return nodeResultList, nil
}

func (cli *Client) QueryHostOfVolume(lunName string) ([]map[string]interface{}, error) {
	data := map[string]interface{}{
		"lunName": lunName,
	}

	resp, err := cli.post("/dsware/service/iscsi/queryHostFromVolume", data)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return nil, fmt.Errorf("Query hosts which lun %s mapped error: %d", lunName, result)
	}

	var hostList []map[string]interface{}

	respData, exist := resp["hostList"].([]interface{})
	if exist {
		for _, i := range respData {
			hostList = append(hostList, i.(map[string]interface{}))
		}
	}

	return hostList, nil
}

func (cli *Client) checkErrorCode(resp map[string]interface{}, errorCode int64) bool {
	details, exist := resp["detail"].([]interface{})
	if !exist || len(details) == 0 {
		return false
	}

	for _, i := range details {
		detail, ok := i.(map[string]interface{})
		if !ok {
			msg := fmt.Sprintf("The detail %v's format is not map[string]interface{}", i)
			log.Errorln(msg)
			return false
		}
		detailErrorCode := int64(detail["errorCode"].(float64))
		if detailErrorCode != errorCode {
			return false
		}
	}

	return true
}

func (cli *Client) ExtendVolume(lunName string, newCapacity int64) error {
	data := map[string]interface{}{
		"volName":    lunName,
		"newVolSize": newCapacity,
	}

	resp, err := cli.post("/dsware/service/v1.3/volume/expand", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return fmt.Errorf("extend volume capacity to %d error: %d", newCapacity, result)
	}

	return nil
}

func (cli *Client) CreateFileSystem(params map[string]interface{}) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"name":            params["name"].(string),
		"storage_pool_id": params["poolId"].(int64),
		"account_id":      0,
	}
	resp, err := cli.post("/api/v2/converged_service/namespaces", data)
	if err != nil {
		return nil, err
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	errorCode := int64(result["code"].(float64))
	if errorCode != 0 {
		msg := fmt.Sprintf("Create filesystem %v error: %d", data, errorCode)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	respData, ok := resp["data"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The data of response %v's format is not map[string]interface{}", resp)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}
	if respData != nil {
		return respData, nil
	}

	return nil, fmt.Errorf("failed to create filesystem %v", data)
}

func (cli *Client) DeleteFileSystem(id string) error {
	url := fmt.Sprintf("/api/v2/converged_service/namespaces/%s", id)
	resp, err := cli.delete(url, nil)
	if err != nil {
		return err
	}
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
		log.Errorln(msg)
		return errors.New(msg)
	}

	errorCode := int64(result["code"].(float64))
	if errorCode != 0 {
		msg := fmt.Sprintf("Delete filesystem %v error: %d", id, errorCode)
		log.Errorln(msg)
		return errors.New(msg)
	}
	return nil
}

func (cli *Client) GetFileSystemByName(name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/api/v2/converged_service/namespaces?name=%s", name)
	resp, err := cli.get(url, nil)
	if err != nil {
		return nil, err
	}
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}
	errorCode := int64(result["code"].(float64))
	if errorCode == FILE_SYSTEM_NOT_EXIST {
		return nil, nil
	}

	if errorCode != 0 {
		msg := fmt.Sprintf("Get filesystem %v error: %d", name, errorCode)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	respData, ok := resp["data"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The data of response %v's format is not map[string]interface{}", resp)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}
	if respData != nil {
		return respData, nil
	}
	return nil, nil
}

func (cli *Client) CreateNfsShare(params map[string]interface{}) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"share_path":     params["sharepath"].(string),
		"file_system_id": params["fsid"].(string),
		"description":    params["description"].(string),
	}

	resp, err := cli.post("/api/v2/nas_protocol/nfs_share", data)
	if err != nil {
		return nil, err
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	errorCode := int64(result["code"].(float64))
	if errorCode != 0 {
		msg := fmt.Sprintf("Create nfs share %v error: %d", data, errorCode)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	respData, ok := resp["data"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The data of response %v's format is not map[string]interface{}", resp)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	if respData != nil {
		return respData, nil
	}
	return nil, fmt.Errorf("failed to create NFS share %v", data)
}

func (cli *Client) DeleteNfsShare(id string) error {
	url := fmt.Sprintf("/api/v2/nas_protocol/nfs_share?id=%s", id)
	resp, err := cli.delete(url, nil)
	if err != nil {
		return err
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
		log.Errorln(msg)
		return errors.New(msg)
	}

	errorCode := int64(result["code"].(float64))
	if errorCode != 0 {
		msg := fmt.Sprintf("Delete NFS share %v error: %d", id, errorCode)
		log.Errorln(msg)
		return errors.New(msg)
	}
	return nil
}

func (cli *Client) GetNfsShareByPath(path string) (map[string]interface{}, error) {
	bytesPath, err := json.Marshal([]map[string]string{{"share_path": path}})
	if err != nil {
		return nil, err
	}

	sharePath := fusionURL.QueryEscape(fmt.Sprintf("%s", bytesPath))
	url := fmt.Sprintf("/api/v2/nas_protocol/nfs_share_list?filter=%s", sharePath)
	resp, err := cli.get(url, nil)
	if err != nil {
		return nil, err
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	errorCode := int64(result["code"].(float64))
	if errorCode != 0 {
		msg := fmt.Sprintf("Get NFS share path %s error: %d", path, errorCode)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	respData, ok := resp["data"].([]interface{})
	if !ok {
		msg := fmt.Sprintf("There is no data info in response %v.", resp)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	for _, s := range respData {
		share, ok := s.(map[string]interface{})
		if !ok {
			msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
			log.Errorln(msg)
			return nil, errors.New(msg)
		}

		if share["share_path"].(string) == path {
			return share, nil
		}
	}
	return nil, nil
}

func (cli *Client) AllowNfsShareAccess(params map[string]interface{}) error {
	data := map[string]interface{}{
		"access_name":  params["name"].(string),
		"share_id":     params["shareid"].(string),
		"access_value": params["accessval"].(int),
		"sync":         0,
		"all_squash":   1,
		"root_squash":  1,
		"type":         0,
	}

	resp, err := cli.post("/api/v2/nas_protocol/nfs_share_auth_client", data)
	if err != nil {
		return err
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
		log.Errorln(msg)
		return errors.New(msg)
	}

	errorCode := int64(result["code"].(float64))
	if errorCode == CLIENT_ALREADY_EXIST {
		log.Warningf("The nfs share auth client %s is already exist.", params["name"].(string))
		return nil
	} else if errorCode != 0 {
		msg := fmt.Sprintf("Allow nfs share %v access error: %d", data, errorCode)
		log.Errorln(msg)
		return errors.New(msg)
	}
	return nil
}

func (cli *Client) DeleteNfsShareAccess(accessID string) error {
	url := fmt.Sprintf("/api/v2/nas_protocol/nfs_share_auth_client?id=%s", accessID)
	resp, err := cli.delete(url, nil)
	if err != nil {
		return err
	}
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
		log.Errorln(msg)
		return errors.New(msg)
	}

	errorCode := int64(result["code"].(float64))
	if errorCode != 0 {
		msg := fmt.Sprintf("Delete nfs share %v access error: %d", accessID, errorCode)
		log.Errorln(msg)
		return errors.New(msg)
	}
	return nil
}

func (cli *Client) GetNfsShareAccess(shareID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/api/v2/nas_protocol/nfs_share_auth_client_list?filter=share_id::%s", shareID)
	resp, err := cli.get(url, nil)
	if err != nil {
		return nil, err
	}
	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	errorCode := int64(result["code"].(float64))
	if errorCode != 0 {
		msg := fmt.Sprintf("Get nfs share %v access error: %d", shareID, errorCode)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}
	respData, ok := resp["data"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The data of response %v's format is not map[string]interface{}", resp)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}
	if respData != nil {
		return respData, nil
	}
	return nil, err
}

func (cli *Client) CreateQuota(params map[string]interface{}) error {
	resp, err := cli.post("/api/v2/file_service/fs_quota", params)
	if err != nil {
		return err
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
		log.Errorln(msg)
		return errors.New(msg)
	}
	errorCode := int64(result["code"].(float64))
	if errorCode != 0 {
		msg := fmt.Sprintf("Failed to create quota %v, error: %d", params, errorCode)
		log.Errorln(msg)
		return errors.New(msg)
	}

	return nil
}

func (cli *Client) GetQuotaByFileSystem(fsID string) (map[string]interface{}, error) {
	url := "/api/v2/file_service/fs_quota?parent_type=40&parent_id=" + fsID + "&range=%7B%22offset%22%3A0%2C%22limit%22%3A100%7D"
	resp, err := cli.get(url, nil)
	if err != nil {
		return nil, err
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}
	errorCode := int64(result["code"].(float64))
	if errorCode != 0 {
		return nil, fmt.Errorf("get quota by filesystem id %s error: %d", fsID, errorCode)
	}

	fsQuotas, exist := resp["data"].([]interface{})
	if !exist || len(fsQuotas) <= 0 {
		return nil, nil
	}

	for _, q := range fsQuotas {
		quota, ok := q.(map[string]interface{})
		if !ok {
			msg := fmt.Sprintf("The fsQuota %v's format is not map[string]interface{}", q)
			log.Errorln(msg)
			return nil, errors.New(msg)
		}
		return quota, nil
	}
	return nil, nil
}

func (cli *Client) DeleteQuota(quotaID string) error {
	url := fmt.Sprintf("/api/v2/file_service/fs_quota/%s", quotaID)
	resp, err := cli.delete(url, nil)
	if err != nil {
		return err
	}

	result, ok := resp["result"].(map[string]interface{})
	if !ok {
		msg := fmt.Sprintf("The result of response %v's format is not map[string]interface{}", resp)
		log.Errorln(msg)
		return errors.New(msg)
	}
	errorCode := int64(result["code"].(float64))
	if errorCode != 0 {
		if errorCode == QUOTA_NOT_EXIST {
			log.Warningf("Quota %s doesn't exist while deleting.", quotaID)
			return nil
		}
		return fmt.Errorf("delete quota %s error: %d", quotaID, errorCode)
	}

	return nil
}

func (cli *Client) CreateQoS(qosName string, qosData map[string]int) error {
	data := map[string]interface{}{
		"qosName":     qosName,
		"qosSpecInfo": qosData,
	}

	resp, err := cli.post("/dsware/service/v1.3/qos/create", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		errorCode, _ := resp["errorCode"].(string)
		return fmt.Errorf("create QoS %v error: %s", data, errorCode)
	}

	return nil
}

func (cli *Client) DeleteQoS(qosName string) error {
	data := map[string]interface{}{
		"qosNames": []string{qosName},
	}

	resp, err := cli.post("/dsware/service/v1.3/qos/delete", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		errorCode, _ := resp["errorCode"].(string)
		return fmt.Errorf("delete QoS %v error: %s", data, errorCode)
	}

	return nil
}

func (cli *Client) AssociateQoSWithVolume(volName, qosName string) error {
	data := map[string]interface{}{
		"keyNames": []string{volName},
		"qosName":  qosName,
	}

	resp, err := cli.post("/dsware/service/v1.3/qos/volume/associate", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		errorCode, _ := resp["errorCode"].(string)
		return fmt.Errorf("associate QoS %s with volume %s error: %s", qosName, volName, errorCode)
	}

	return nil
}

func (cli *Client) DisassociateQoSWithVolume(volName, qosName string) error {
	data := map[string]interface{}{
		"keyNames": []string{volName},
		"qosName":  qosName,
	}

	resp, err := cli.post("/dsware/service/v1.3/qos/volume/disassociate", data)
	if err != nil {
		return err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		errorCode, _ := resp["errorCode"].(string)
		return fmt.Errorf("disassociate QoS %s with volume %s error: %s", qosName, volName, errorCode)
	}

	return nil
}

func (cli *Client) GetQoSNameByVolume(volName string) (string, error) {
	url := fmt.Sprintf("/dsware/service/v1.3/volume/qos?volName=%s", volName)
	resp, err := cli.get(url, nil)
	if err != nil {
		return "", err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		errorCode, _ := resp["errorCode"].(string)
		return "", fmt.Errorf("get qos by volume %s error: %s", volName, errorCode)
	}

	qosName, exist := resp["qosName"].(string)
	if !exist {
		return "", nil
	}

	return qosName, nil
}

func (cli *Client) getAllPools() ([]interface{}, error) {
	resp, err := cli.get("/dsware/service/v1.3/storagePool", nil)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return nil, fmt.Errorf("get all pools error: %d", result)
	}

	storagePools, exist := resp["storagePools"].([]interface{})
	if !exist || len(storagePools) <= 0 {
		return nil, nil
	}
	return storagePools, nil
}

func (cli *Client) GetAssociateCountOfQoS(qosName string) (int, error) {
	storagePools, err := cli.getAllPools()
	if err != nil {
		return 0, err
	}
	if storagePools == nil {
		return 0, nil
	}

	associatePools, err := cli.getAssociatePoolOfQoS(qosName)
	if err != nil {
		log.Errorf("Get associate snapshot of QoS %s error: %v", qosName, err)
		return 0, err
	}
	pools, ok := associatePools["pools"].([]interface{})
	if !ok {
		msg := fmt.Sprintf("There is no pools info in response %v.", associatePools)
		log.Errorln(msg)
		return 0, errors.New(msg)
	}
	storagePoolsCount := len(pools)

	for _, p := range storagePools {
		pool, ok := p.(map[string]interface{})
		if !ok {
			msg := fmt.Sprintf("The storage pool %v's format is not map[string]interface{}", p)
			log.Errorln(msg)
			return 0, errors.New(msg)
		}
		poolId := int64(pool["poolId"].(float64))
		volumes, err := cli.getAssociateObjOfQoS(qosName, "volume", poolId)
		if err != nil {
			log.Errorf("Get associate volume of QoS %s error: %v", qosName, err)
			return 0, err
		}

		snapshots, err := cli.getAssociateObjOfQoS(qosName, "snapshot", poolId)
		if err != nil {
			log.Errorf("Get associate snapshot of QoS %s error: %v", qosName, err)
			return 0, err
		}

		volumeCount := int(volumes["totalNum"].(float64))
		snapshotCount := int(snapshots["totalNum"].(float64))
		totalCount := volumeCount + snapshotCount + storagePoolsCount
		if totalCount != 0 {
			return totalCount, nil
		}
	}

	return 0, nil
}

func (cli *Client) getAssociateObjOfQoS(qosName, objType string, poolId int64) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"qosName": qosName,
		"poolId":  poolId,
	}

	resp, err := cli.post("/dsware/service/v1.3/qos/volume/list?type=associated", data)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		errorCode, _ := resp["errorCode"].(string)
		return nil, fmt.Errorf("get qos %s associate obj %s error: %s", qosName, objType, errorCode)
	}

	return resp, nil
}

func (cli *Client) getAssociatePoolOfQoS(qosName string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"qosName": qosName,
	}

	resp, err := cli.post("/dsware/service/v1.3/qos/storagePool/list?type=associated", data)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		errorCode, _ := resp["errorCode"].(string)
		return nil, fmt.Errorf("get qos %s associate storagePool error: %s", qosName, errorCode)
	}

	return resp, nil
}

func (cli *Client) GetHostLunId(hostName, lunName string) (string, error) {
	data := map[string]interface{}{
		"hostName": hostName,
	}

	resp, err := cli.post("/dsware/service/v1.3/host/lun/list", data)
	if err != nil {
		return "", err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return "", fmt.Errorf("get hostLun of hostName %s error: %d", hostName, result)
	}

	hostLunList, exist := resp["hostLunList"].([]interface{})
	if !exist {
		log.Infof("Host %s does not exist", hostName)
		return "", nil
	}

	for _, i := range hostLunList {
		hostLun, ok := i.(map[string]interface{})
		if !ok {
			msg := fmt.Sprintf("The hostlun %v's format is not map[string]interface{}", i)
			log.Errorln(msg)
			return "", errors.New(msg)
		}
		if hostLun["lunName"].(string) == lunName {
			return strconv.FormatInt(int64(hostLun["lunId"].(float64)), 10), nil
		}
	}
	return "", nil
}

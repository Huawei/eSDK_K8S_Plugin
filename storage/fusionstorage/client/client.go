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
	"time"
	"utils/log"
)

const (
	NO_AUTHENTICATED               int64 = 10000003
	VOLUME_TO_DELETE_NOT_EXIST     int64 = 32150005
	VOLUME_NAME_TO_QUERY_NOT_EXIST int64 = 31000000
	OFF_LINE_CODE = "1077949069"
)

var (
	LOG_FILTER = map[string]map[string]bool{
		"POST": map[string]bool{
			"/dsware/service/v1.3/sec/login":     true,
			"/dsware/service/v1.3/sec/keepAlive": true,

		},
		"GET": map[string]bool{
			"/dsware/service/v1.3/storagePool": true,
		},
	}
)

func logFilter(method, url string) bool {
	filter, exist := LOG_FILTER[method]
	return exist && filter[url]
}

type Client struct {
	url       string
	user      string
	password  string
	version   string
	authToken string
	client    *http.Client
}


func NewClient(url, user, password string) *Client {
	return &Client{
		url:      url,
		user:     user,
		password: password,
	}
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

	//cli.version = "v1.3"
	cli.authToken = ""

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
		return fmt.Errorf("Login %s error: %d", cli.url, result)
	}
	cli.authToken = respHeader["X-Auth-Token"][0]
	version, err := cli.GetStorageVersion()
	cli.version = version
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

func (cli *Client) GetStorageVersion() (string, error) {
	res, err := cli.get("/dsware/service/v1/version")
	if err != nil {
		return "", err
	}
	version := res["version"].(string)
	return version, nil
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

	if cli.authToken != "" {
		req.Header.Set("X-Auth-Token", cli.authToken)
	}

	if !logFilter(method, url) {
		log.Infof("Request method: %s, url: %s, body: %v", method, reqUrl, data)
	}

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
	var result int64
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
	if ret, ok := body["result"].(float64); ok {
		result = int64(ret)
	} else if ret, ok := body["result"].(map[string]interface{}); ok {
		result = int64(ret["code"].(float64))
	}
	if code, exist := body["errorCode"].(string) ;exist && code == OFF_LINE_CODE{
		goto RETRY
	}

	if result == NO_AUTHENTICATED {
		goto RETRY
	}

	return respHeader, body, nil

RETRY:
	err = cli.Login()
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

func (cli *Client) get(url string) (map[string]interface{}, error) {
	_, body, err := cli.call("GET", url, nil)
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
		return fmt.Errorf("Create volume %v error: %d", data, result)
	}

	return nil
}

func (cli *Client) GetVolumeByName(name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/dsware/service/v1.3/volume/queryByName?volName=%s", name)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		errorCode, exist := resp["errorCode"].(float64)
		if exist && int64(errorCode) == VOLUME_NAME_TO_QUERY_NOT_EXIST {
			log.Warningf("Volume of name %s doesn't exist", name)
			return nil, nil
		}
		if exist && int64(errorCode) == 50150005 {
			log.Warningf("Volume of name %s doesn't exist", name)
			return nil, nil
		}

		return nil, fmt.Errorf("Get volume by name %s error: %d", name, result)
	}
	lun := resp["lunDetailInfo"].(map[string]interface{})
	if lun != nil && len(lun) == 0 {
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
		detailList := resp["detail"].([]interface{})
		detail := detailList[0].(map[string]interface{})

		errorCode := int64(detail["errorCode"].(float64))
		if errorCode == VOLUME_TO_DELETE_NOT_EXIST {
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

	result := int64(resp["result"].(float64))
	if result != 0 {
		return fmt.Errorf("Attach volume %s to %s error: %d", name, ip, result)
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

	result := int64(resp["result"].(float64))
	if result != 0 {
		return fmt.Errorf("Detach volume %s from %s error: %d", name, ip, result)
	}

	return nil
}

func (cli *Client) GetAllServers() ([]map[string]interface{}, error) {
	resp, err := cli.get("/dsware/service/v1.3/server/list")
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		return nil, fmt.Errorf("Get all servers error: %d", result)
	}

	hostInfoList, exist := resp["hostInfoList"].([]interface{})
	if !exist || len(hostInfoList) <= 0 {
		return nil, errors.New("No valid server info returned")
	}

	var servers []map[string]interface{}
	for _, host := range hostInfoList {
		servers = append(servers, host.(map[string]interface{}))
	}

	return servers, nil
}

func (cli *Client) GetPoolByName(poolName string) (map[string]interface{}, error) {
	resp, err := cli.get("/dsware/service/v1.3/storagePool")
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
		pool := p.(map[string]interface{})
		if pool["poolName"].(string) == poolName {
			return pool, nil
		}
	}

	return nil, nil
}

func (cli *Client) GetAllPools() (map[string]interface{}, error) {
	resp, err := cli.get("/dsware/service/v1.3/storagePool")
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
		pool := p.(map[string]interface{})
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


func (cli *Client) GetHostByName(hostName string ) (map[string]interface{},error) {

	resp, err := cli.get("/dsware/service/iscsi/queryAllHost")
	if err != nil {
		return nil, err
	}
	result := int64(resp["result"].(float64))
	if result != 0 {
		msg := fmt.Sprintf("Get hostName error: %d",result)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}
	hostList := resp["hostList"].([]interface{})
	if len(hostList) == 0 {
		return nil, nil
	}
	for _, host := range hostList {
		hostInfo := host.(map[string]interface{})
		if hostInfo["hostName"] == hostName {
			return hostInfo, nil

		}

	}
	return nil, nil
}

func (cli *Client) CreateHost(hostName string ) error {

	data := map[string]interface{}{
		"hostName" : hostName,
	}

	resp, err := cli.post("/dsware/service/iscsi/createHost",data)
	if err != nil {
		return  err
	}
	result := int64(resp["result"].(float64))
	if result != 0 {
		msg := fmt.Sprintf("Create host %s error: %d",hostName, result)
		log.Errorln(msg)
		return errors.New(msg)
	}
	return nil
}
func (cli *Client) GetInitiatorByName(initiatorName string ) (map[string]interface{}, error) {

	data := map[string]interface{}{}

	resp, err := cli.post("/dsware/service/iscsi/queryPortInfo",data)
	if err != nil {
		return  nil, err
	}
	result := int64(resp["result"].(float64))
	if result != 0 {
		msg := fmt.Sprintf("Get initiator %s error: %d",initiatorName, result)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}
	portList := resp["portList"].([]interface{})
	if len(portList) == 0 {
		return nil, nil
	}
	for _, port := range portList{
		portInfo := port.(map[string]interface{})
		if portInfo["portName"].(string) == initiatorName {
			return portInfo ,nil

		}
	}
	return nil, nil
}

func (cli *Client) QueryHostByPort(initiatorName string ) (string, error) {

	data := map[string]interface{}{
		"portName": []string{initiatorName},
	}

	resp, err := cli.post("/dsware/service/iscsi/queryHostByPort",data)
	if err != nil {
		return  "", err
	}
	result := int64(resp["result"].(float64))
	if result !=0 {
		msg := fmt.Sprintf("Get the host of initiator %s error: %d",initiatorName, result)
		log.Errorln(msg)
		return "", errors.New(msg)
	}
	portHostMap := resp["portHostMap"].(map[string]interface{})
	if len(portHostMap) == 0 {
		return "",nil
	}
	hosts := portHostMap[initiatorName].([]interface{})
	return hosts[0].(string) ,nil
}

func (cli *Client) GetParentHostByInitiatorName(initiatorName string ) (map[string]interface{}, error) {

	data := map[string]interface{}{
		"portName" : initiatorName,
	}

	resp, err := cli.post("/dsware/service/iscsi/queryPortInfo",data)
	if err != nil {
		return  nil, err
	}
	result := int64(resp["result"].(float64))
	if result != 0 {
		msg := fmt.Sprintf("Get the parent host of initiator %s error: %d",initiatorName, result)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}
	return resp, nil
}

func (cli *Client) CreateInitiator(initiatorName string ) error {

	data := map[string]interface{}{
		"portName" : initiatorName,
	}

	resp, err := cli.post("/dsware/service/iscsi/createPort",data)
	if err != nil {
		return  err
	}
	result := int64(resp["result"].(float64))
	if result != 0 {
		msg := fmt.Sprintf("Create initiator %s error: %d",initiatorName, result)
		log.Errorln(msg)
		return errors.New(msg)
	}
	return nil
}
func (cli *Client) AddPortToHost(initiatorName, hostName string ) error {

	data := map[string]interface{}{
		"portNames" : []string{initiatorName},
		"hostName": hostName,
	}

	resp, err := cli.post("/dsware/service/iscsi/addPortToHost",data)
	if err != nil {
		return  err
	}
	result := int64(resp["result"].(float64))
	if result != 0 {
		msg := fmt.Sprintf("Add initiator %s to host %s error: %d",initiatorName, hostName, result)
		log.Errorln(msg)
		return errors.New(msg)
	}
	return nil
}

func (cli *Client) AddLunToHost(lunName, hostName string ) error {

	data := map[string]interface{}{
		"lunNames" : []string{lunName},
		"hostName": hostName,
	}

	resp, err := cli.post("/dsware/service/iscsi/addLunsToHost",data)
	if err != nil {
		return  err
	}
	result := int64(resp["result"].(float64))
	if result != 0 {
		msg := fmt.Sprintf("Add lun %s to host %s error: %d",lunName, hostName, result)
		log.Errorln(msg)
		return errors.New(msg)
	}
	return nil
}

func (cli *Client) QueryHostByLun(lunName string) ([]interface{}, error) {
	data := map[string]interface{}{
		"lunName" : lunName,
	}

	resp, err := cli.post("/dsware/service/v1.3/lun/host/list",data)
	if err != nil {
		return nil, err
	}

	result := int64(resp["result"].(float64))
	if result != 0 {
		msg := fmt.Sprintf("Query host by lun %s error: %d", lunName, result)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	hostList := resp["hostList"].([]interface{})

	return hostList, nil
}


func (cli *Client) DeleteLunFromHost(lunName, hostName string ) error {

	data := map[string]interface{}{
		"lunNames" : []string{lunName},
		"hostName": hostName,
	}

	resp, err := cli.post("/dsware/service/iscsi/deleteLunFromHost", data)
	if err != nil {
		return  err
	}
	result := int64(resp["result"].(float64))
	if result != 0 {
		msg := fmt.Sprintf("Unmap lun %s from host %s error: %d", lunName, hostName, result)
		log.Errorln(msg)
		return errors.New(msg)
	}
	return nil
}

func (cli *Client) QueryIscsiPortal() ([]interface{}, error) {
	data := make(map[string]interface{})
	resp, err := cli.post("/dsware/service/cluster/dswareclient/queryIscsiPortal", data)

	if err != nil {
		return  nil, err
	}
	result := int64(resp["result"].(float64))
	if result != 0 {
		msg := fmt.Sprintf("Query iscsi portal error code : %d", result)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}

	nodeResultList := resp["nodeResultList"].([]interface{})

	return nodeResultList, nil
}


func (cli *Client) QueryHostFromVolume(lunName, hostName string) (map[string]interface{}, error) {

	data := map[string]interface{}{
		"lunName":lunName,
	}
	resp, err := cli.post("/dsware/service/iscsi/queryHostFromVolume", data)
	if err != nil {
		return  nil, err
	}
	result := int64(resp["result"].(float64))
	if result != 0 {
		msg := fmt.Sprintf("Query lun %s mapping information code : %d", lunName, result)
		log.Errorln(msg)
		return nil, errors.New(msg)
	}
	hostList := resp["hostList"].([]interface{})
	if len(hostList) ==0 {
		return nil, nil
	}
	for _,host := range hostList {
		hostInfo := host.(map[string]interface{})
		if hostInfo["hostName"] == hostName {
			return hostInfo, nil
		}
	}

	return nil, nil
}





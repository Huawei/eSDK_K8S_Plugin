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
	URL "net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Huawei/eSDK_K8S_Plugin/src/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/src/utils/log"
)

const (
	OBJECT_NOT_EXIST             int64 = 1077948996
	OBJECT_ID_NOT_UNIQUE         int64 = 1077948997
	LUN_ALREADY_IN_GROUP         int64 = 1077936862
	HOSTGROUP_NOT_IN_MAPPING     int64 = 1073804552
	LUNGROUP_NOT_IN_MAPPING      int64 = 1073804554
	HOSTGROUP_ALREADY_IN_MAPPING int64 = 1073804556
	LUNGROUP_ALREADY_IN_MAPPING  int64 = 1073804560
	HOST_ALREADY_IN_HOSTGROUP    int64 = 1077937501
	HOST_NOT_IN_HOSTGROUP        int64 = 1073745412
	OBJECT_NAME_ALREADY_EXIST    int64 = 1077948993
	HOST_NOT_EXIST               int64 = 1077937498
	HOSTGROUP_NOT_EXIST          int64 = 1077937500
	LUN_NOT_EXIST                int64 = 1077936859
	MAPPING_NOT_EXIST            int64 = 1077951819
	FILESYSTEM_NOT_EXIST         int64 = 1073752065
	SHARE_NOT_EXIST              int64 = 1077939717
	SHARE_PATH_INVALID           int64 = 1077939729
	SHARE_ALREADY_EXIST          int64 = 1077939724
	SHARE_PATH_ALREADY_EXIST     int64 = 1077940500
	LUNCOPY_NOT_EXIST            int64 = 1077950183
	CLONEPAIR_NOT_EXIST          int64 = 1073798147
	LUN_SNAPSHOT_NOT_EXIST       int64 = 1077937880
	SNAPSHOT_NOT_ACTIVATED       int64 = 1077937891
	FS_SNAPSHOT_NOT_EXIST        int64 = 1073754118
	REPLICATION_NOT_EXIST        int64 = 1077937923
	HYPERMETRO_NOT_EXIST         int64 = 1077674242
	SNAPSHOT_PARENT_NOT_EXIST    int64 = 1073754117
	DEFAULT_PARALLEL_COUNT       int   = 50
	MAX_PARALLEL_COUNT           int   = 1000
	MIN_PARALLEL_COUNT           int   = 20
)

var (
	LOG_FILTER = map[string]map[string]bool{
		"POST": map[string]bool{
			"/xx/sessions": true,
		},
		"GET": map[string]bool{
			"/storagepool":     true,
			"/license/feature": true,
		},
	}

	LOG_REGEX_FILTER = map[string][]string{
		"GET": {
			`/vstore_pair\?filter=ID`,
		},
	}

	clientSemaphore *utils.Semaphore
)

func logFilter(method, url string) bool {
	if filter, exist := LOG_FILTER[method]; exist && filter[url] {
		return true
	}

	if filter, exist := LOG_REGEX_FILTER[method]; exist {
		for _, k := range filter {
			match, err := regexp.MatchString(k, url)
			if err == nil && match {
				return true
			}
		}
	}

	return false
}

type Client struct {
	url        string
	urls       []string
	user       string
	password   string
	deviceid   string
	token      string
	client     *http.Client
	vstoreName string

	reloginMutex sync.Mutex
}

type Response struct {
	Error map[string]interface{} `json:"error"`
	Data  interface{}            `json:"data,omitempty"`
}

func NewClient(urls []string, user, password, vstoreName, parallelNum string) *Client {
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
		urls:       urls,
		user:       user,
		password:   password,
		vstoreName: vstoreName,
	}
}

func (cli *Client) call(method string, url string, data map[string]interface{}) (Response, error) {
	var r Response
	var err error

	r, err = cli.baseCall(method, url, data)
	if (err != nil && err.Error() == "unconnected") ||
		(r.Error != nil && int64(r.Error["code"].(float64)) == -401) {
		// Current connection fails, try to relogin to other urls if exist,
		// if relogin success, resend the request again.
		log.Infof("Try to relogin and resend request method: %s, url: %s", method, url)

		err = cli.reLogin()
		if err == nil {
			r, err = cli.baseCall(method, url, data)
		}
	}

	return r, err
}

func (cli *Client) getRequest(method string, url string, data map[string]interface{}) (*http.Request, error) {
	var req *http.Request
	var err error

	reqUrl := cli.url
	if cli.deviceid != "" {
		reqUrl += "/" + cli.deviceid
	}
	reqUrl += url

	var reqBody io.Reader

	if data != nil {
		reqBytes, err := json.Marshal(data)
		if err != nil {
			log.Errorf("json.Marshal data %v error: %v", data, err)
			return req, err
		}
		reqBody = bytes.NewReader(reqBytes)
	}

	req, err = http.NewRequest(method, reqUrl, reqBody)
	if err != nil {
		log.Errorf("Construct http request error: %s", err.Error())
		return req, err
	}

	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Content-Type", "application/json")

	if cli.token != "" {
		req.Header.Set("iBaseToken", cli.token)
	}

	return req, nil
}

func (cli *Client) baseCall(method string, url string, data map[string]interface{}) (Response, error) {
	var r Response
	var req *http.Request
	var err error

	reqUrl := cli.url
	reqUrl += url

	if url != "/xx/sessions" && url != "/sessions" {
		cli.reloginMutex.Lock()
		req, err = cli.getRequest(method, url, data)
		cli.reloginMutex.Unlock()
	} else {
		req, err = cli.getRequest(method, url, data)
	}

	if err != nil {
		return r, err
	}

	if !logFilter(method, url) {
		log.Infof("Request method: %s, url: %s, body: %v", method, reqUrl, data)
	}

	clientSemaphore.Acquire()
	defer clientSemaphore.Release()

	resp, err := cli.client.Do(req)
	if err != nil {
		log.Errorf("Send request method: %s, url: %s, error: %v", method, reqUrl, err)
		return r, errors.New("unconnected")
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("Read response data error: %v", err)
		return r, err
	}

	if !logFilter(method, url) {
		log.Infof("Response method: %s, url: %s, body: %s", method, reqUrl, body)
	}

	err = json.Unmarshal(body, &r)
	if err != nil {
		log.Errorf("json.Unmarshal data %s error: %v", body, err)
		return r, err
	}

	return r, nil
}

func (cli *Client) get(url string) (Response, error) {
	return cli.call("GET", url, nil)
}

func (cli *Client) post(url string, data map[string]interface{}) (Response, error) {
	return cli.call("POST", url, data)
}

func (cli *Client) put(url string, data map[string]interface{}) (Response, error) {
	return cli.call("PUT", url, data)
}

func (cli *Client) delete(url string, data map[string]interface{}) (Response, error) {
	return cli.call("DELETE", url, data)
}

func (cli *Client) DuplicateClient() *Client {
	dup := *cli

	dup.urls = make([]string, len(cli.urls))
	copy(dup.urls, cli.urls)

	dup.client = nil

	return &dup
}

func (cli *Client) Login() error {
	var resp Response
	var err error

	jar, _ := cookiejar.New(nil)
	cli.client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Jar:     jar,
		Timeout: 60 * time.Second,
	}

	data := map[string]interface{}{
		"username": cli.user,
		"password": cli.password,
		"scope":    "0",
	}

	if len(cli.vstoreName) > 0 {
		data["vstorename"] = cli.vstoreName
	}

	cli.deviceid = ""
	cli.token = ""
	for i, url := range cli.urls {
		cli.url = url

		log.Infof("Try to login %s", cli.url)
		resp, err = cli.baseCall("POST", "/xx/sessions", data)
		if err == nil {
			/* Sort the login url to the last slot of san addresses, so that
			   if this connection error, next time will try other url first. */
			cli.urls[i], cli.urls[len(cli.urls)-1] = cli.urls[len(cli.urls)-1], cli.urls[i]
			break
		} else if err.Error() != "unconnected" {
			log.Errorf("Login %s error", cli.url)
			break
		}

		log.Warningf("Login %s error due to connection failure, gonna try another url", cli.url)
	}

	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Login %s error: %d", cli.url, code)
		return errors.New(msg)
	}

	respData := resp.Data.(map[string]interface{})
	cli.deviceid = respData["deviceid"].(string)
	cli.token = respData["iBaseToken"].(string)

	log.Infof("Login %s success", cli.url)
	return nil
}

func (cli *Client) Logout() {
	resp, err := cli.baseCall("DELETE", "/sessions", nil)
	if err != nil {
		log.Warningf("Logout %s error: %v", cli.url, err)
		return
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		log.Warningf("Logout %s error: %d", cli.url, code)
		return
	}

	log.Infof("Logout %s success", cli.url)
}

func (cli *Client) reLogin() error {
	oldToken := cli.token

	cli.reloginMutex.Lock()
	defer cli.reloginMutex.Unlock()

	if cli.token != "" && oldToken != cli.token {
		// Coming here indicates other thread had already done relogin, so no need to relogin again
		return nil
	} else if cli.token != "" {
		cli.Logout()
	}

	err := cli.Login()
	if err != nil {
		log.Errorf("Try to relogin error: %v", err)
		return err
	}

	return nil
}

func (cli *Client) GetvStoreName() string {
	return cli.vstoreName
}

func (cli *Client) GetLunByName(name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/lun?filter=NAME::%s&range=[0-100]", name)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get lun %s info error: %d", name, code)
		return nil, errors.New(msg)
	}

	if resp.Data == nil {
		log.Infof("Lun %s does not exist", name)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) <= 0 {
		log.Infof("Lun %s does not exist", name)
		return nil, nil
	}

	lun := respData[0].(map[string]interface{})
	return lun, nil
}

func (cli *Client) GetLunByID(id string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/lun/%s", id)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get lun %s info error: %d", id, code)
		return nil, errors.New(msg)
	}

	lun := resp.Data.(map[string]interface{})
	return lun, nil
}

func (cli *Client) AddLunToGroup(lunID string, groupID string) error {
	data := map[string]interface{}{
		"ID":               groupID,
		"ASSOCIATEOBJTYPE": "11",
		"ASSOCIATEOBJID":   lunID,
	}

	resp, err := cli.post("/lungroup/associate", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == OBJECT_ID_NOT_UNIQUE || code == LUN_ALREADY_IN_GROUP {
		log.Warningf("Lun %s is already in group %s", lunID, groupID)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Add lun %s to group %s error: %d", lunID, groupID, code)
		return errors.New(msg)
	}

	return nil
}

func (cli *Client) RemoveLunFromGroup(lunID, groupID string) error {
	data := map[string]interface{}{
		"ID":               groupID,
		"ASSOCIATEOBJTYPE": "11",
		"ASSOCIATEOBJID":   lunID,
	}

	resp, err := cli.delete("/lungroup/associate", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == OBJECT_NOT_EXIST {
		log.Warningf("LUN %s is not in lungroup %s", lunID, groupID)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Remove lun %s from group %s error: %d", lunID, groupID, code)
		return errors.New(msg)
	}

	return nil
}

func (cli *Client) GetLunGroupByName(name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/lungroup?filter=NAME::%s", name)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get lungroup %s info error: %d", name, code)
		return nil, errors.New(msg)
	}

	if resp.Data == nil {
		log.Infof("Lungroup %s does not exist", name)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) <= 0 {
		log.Infof("Lungroup %s does not exist", name)
		return nil, nil
	}

	group := respData[0].(map[string]interface{})
	return group, nil
}

func (cli *Client) CreateLunGroup(name string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME":    name,
		"APPTYPE": 0,
	}
	resp, err := cli.post("/lungroup", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == OBJECT_NAME_ALREADY_EXIST {
		log.Infof("Lungroup %s already exists", name)
		return cli.GetLunGroupByName(name)
	}
	if code != 0 {
		msg := fmt.Sprintf("Create lungroup %s error: %d", name, code)
		return nil, errors.New(msg)
	}

	lunGroup := resp.Data.(map[string]interface{})
	return lunGroup, nil
}

func (cli *Client) DeleteLunGroup(id string) error {
	url := fmt.Sprintf("/lungroup/%s", id)
	resp, err := cli.delete(url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == OBJECT_NOT_EXIST {
		log.Infof("Lungroup %s does not exist while deleting", id)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Delete lungroup %s error: %d", id, code)
		return errors.New(msg)
	}

	return nil
}

func (cli *Client) QueryAssociateLunGroup(objType int, objID string) ([]interface{}, error) {
	url := fmt.Sprintf("/lungroup/associate?ASSOCIATEOBJTYPE=%d&ASSOCIATEOBJID=%s", objType, objID)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Associate query lungroup by obj %s of type %d error: %d", objID, objType, code)
	}

	if resp.Data == nil {
		log.Infof("Obj %s of type %d doesn't associate to any lungroup", objID, objType)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	return respData, nil
}

func (cli *Client) CreateLun(params map[string]interface{}) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME":        params["name"].(string),
		"PARENTID":    params["parentid"].(string),
		"CAPACITY":    params["capacity"].(int64),
		"DESCRIPTION": params["description"].(string),
		"ALLOCTYPE":   params["alloctype"].(int),
	}

	resp, err := cli.post("/lun", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Create volume %v error: %d", data, code)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}

func (cli *Client) DeleteLun(id string) error {
	url := fmt.Sprintf("/lun/%s", id)
	resp, err := cli.delete(url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == LUN_NOT_EXIST {
		log.Infof("Lun %s does not exist while deleting", id)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Delete lun %s error: %d", id, code)
		return errors.New(msg)
	}

	return nil
}

func (cli *Client) GetPoolByName(name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/storagepool?filter=NAME::%s&range=[0-100]", name)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get pool %s info error: %d", name, code)
		return nil, errors.New(msg)
	}

	if resp.Data == nil {
		log.Infof("Pool %s does not exist", name)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) <= 0 {
		log.Infof("Pool %s does not exist", name)
		return nil, nil
	}

	pool := respData[0].(map[string]interface{})
	return pool, nil
}

func (cli *Client) GetAllPools() (map[string]interface{}, error) {
	resp, err := cli.get("/storagepool")
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get all pools info error: %d", code)
		return nil, errors.New(msg)
	}

	if resp.Data == nil {
		log.Infof("There's no pools exist")
		return nil, nil
	}

	pools := make(map[string]interface{})

	respData := resp.Data.([]interface{})
	for _, p := range respData {
		pool := p.(map[string]interface{})
		name := pool["NAME"].(string)
		pools[name] = pool
	}

	return pools, nil
}

func (cli *Client) CreateHost(name string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME":            name,
		"OPERATIONSYSTEM": 0,
	}

	resp, err := cli.post("/host", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == OBJECT_NAME_ALREADY_EXIST {
		log.Infof("Host %s already exists", name)
		return cli.GetHostByName(name)
	}
	if code != 0 {
		msg := fmt.Sprintf("Create host %s error: %d", name, code)
		return nil, errors.New(msg)
	}

	host := resp.Data.(map[string]interface{})
	return host, nil
}

func (cli *Client) UpdateHost(id string, alua map[string]interface{}) error {
	url := fmt.Sprintf("/host/%s", id)
	data := map[string]interface{}{}

	if accessMode, ok := alua["accessMode"]; ok {
		data["accessMode"] = accessMode
	}

	if hyperMetroPathOptimized, ok := alua["hyperMetroPathOptimized"]; ok {
		data["hyperMetroPathOptimized"] = hyperMetroPathOptimized
	}

	resp, err := cli.put(url, data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("update host %s by %v error: %d", id, data, code)
	}

	return nil
}

func (cli *Client) GetHostByName(name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/host?filter=NAME::%s&range=[0-100]", name)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get host %s error: %d", name, code)
		return nil, errors.New(msg)
	}

	if resp.Data == nil {
		log.Infof("Host %s does not exist", name)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) <= 0 {
		log.Infof("Host %s does not exist", name)
		return nil, nil
	}

	host := respData[0].(map[string]interface{})
	return host, nil
}

func (cli *Client) DeleteHost(id string) error {
	url := fmt.Sprintf("/host/%s", id)
	resp, err := cli.delete(url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == HOST_NOT_EXIST {
		log.Infof("Host %s does not exist", id)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Delete host %s error: %d", id, code)
		return errors.New(msg)
	}

	return nil
}

func (cli *Client) CreateHostGroup(name string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME": name,
	}
	resp, err := cli.post("/hostgroup", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == OBJECT_NAME_ALREADY_EXIST {
		log.Infof("Hostgroup %s already exists", name)
		return cli.GetHostGroupByName(name)
	}
	if code != 0 {
		msg := fmt.Sprintf("Create hostgroup %s error: %d", name, code)
		return nil, errors.New(msg)
	}

	hostGroup := resp.Data.(map[string]interface{})
	return hostGroup, nil
}

func (cli *Client) GetHostGroupByName(name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/hostgroup?filter=NAME::%s", name)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get hostgroup %s error: %d", name, code)
		return nil, errors.New(msg)
	}

	if resp.Data == nil {
		log.Infof("Hostgroup %s does not exist", name)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) <= 0 {
		log.Infof("Hostgroup %s does not exist", name)
		return nil, nil
	}

	hostGroup := respData[0].(map[string]interface{})
	return hostGroup, nil
}

func (cli *Client) DeleteHostGroup(id string) error {
	url := fmt.Sprintf("/hostgroup/%s", id)
	resp, err := cli.delete(url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == HOSTGROUP_NOT_EXIST {
		log.Infof("Hostgroup %s does not exist while deleting", id)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Delete hostgroup %s error: %d", id, code)
		return errors.New(msg)
	}

	return nil
}

func (cli *Client) CreateMapping(name string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME": name,
	}
	resp, err := cli.post("/mappingview", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == OBJECT_NAME_ALREADY_EXIST {
		log.Infof("Mapping %s already exists", name)
		return cli.GetMappingByName(name)
	}
	if code != 0 {
		msg := fmt.Sprintf("Create mapping %s error: %d", name, code)
		return nil, errors.New(msg)
	}

	mapping := resp.Data.(map[string]interface{})
	return mapping, nil
}

func (cli *Client) GetMappingByName(name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/mappingview?filter=NAME::%s", name)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get mapping %s error: %d", name, code)
		return nil, errors.New(msg)
	}

	if resp.Data == nil {
		log.Infof("Mapping %s does not exist", name)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) <= 0 {
		log.Infof("Mapping %s does not exist", name)
		return nil, nil
	}

	mapping := respData[0].(map[string]interface{})
	return mapping, nil
}

func (cli *Client) DeleteMapping(id string) error {
	url := fmt.Sprintf("/mappingview/%s", id)
	resp, err := cli.delete(url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == MAPPING_NOT_EXIST {
		log.Infof("Mapping %s does not exist while deleting", id)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Delete mapping %s error: %d", id, code)
		return errors.New(msg)
	}

	return nil
}

func (cli *Client) AddHostToGroup(hostID, hostGroupID string) error {
	data := map[string]interface{}{
		"ID":               hostGroupID,
		"ASSOCIATEOBJTYPE": 21,
		"ASSOCIATEOBJID":   hostID,
	}
	resp, err := cli.post("/hostgroup/associate", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == HOST_ALREADY_IN_HOSTGROUP {
		log.Infof("Host %s is already in hostgroup %s", hostID, hostGroupID)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Add host %s to hostgroup %s error: %d", hostID, hostGroupID, code)
		return errors.New(msg)
	}

	return nil
}

func (cli *Client) RemoveHostFromGroup(hostID, hostGroupID string) error {
	data := map[string]interface{}{
		"ID":               hostGroupID,
		"ASSOCIATEOBJTYPE": 21,
		"ASSOCIATEOBJID":   hostID,
	}
	resp, err := cli.delete("/host/associate", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == HOST_NOT_IN_HOSTGROUP {
		log.Infof("Host %s is not in hostgroup %s", hostID, hostGroupID)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Remove host %s from hostgroup %s error: %d", hostID, hostGroupID, code)
		return errors.New(msg)
	}

	return nil
}

func (cli *Client) QueryAssociateHostGroup(objType int, objID string) ([]interface{}, error) {
	url := fmt.Sprintf("/hostgroup/associate?ASSOCIATEOBJTYPE=%d&ASSOCIATEOBJID=%s", objType, objID)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Associate query hostgroup by obj %s of type %d error: %d", objID, objType, code)
	}

	if resp.Data == nil {
		log.Infof("Obj %s of type %d doesn't associate to any hostgroup", objID, objType)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	return respData, nil
}

func (cli *Client) AddIscsiInitiator(initiator string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"ID": initiator,
	}

	resp, err := cli.post("/iscsi_initiator", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == OBJECT_ID_NOT_UNIQUE {
		log.Infof("Iscsi initiator %s already exists", initiator)
		return cli.GetIscsiInitiatorByID(initiator)
	}
	if code != 0 {
		msg := fmt.Sprintf("Add iscsi initiator %s error: %d", initiator, code)
		return nil, errors.New(msg)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}

func (cli *Client) UpdateIscsiInitiator(initiator string, alua map[string]interface{}) error {
	url := fmt.Sprintf("/iscsi_initiator/%s", initiator)
	data := map[string]interface{}{}

	if multiPathType, ok := alua["MULTIPATHTYPE"]; ok {
		data["MULTIPATHTYPE"] = multiPathType
	}

	if failoverMode, ok := alua["FAILOVERMODE"]; ok {
		data["FAILOVERMODE"] = failoverMode
	}

	if specialModeType, ok := alua["SPECIALMODETYPE"]; ok {
		data["SPECIALMODETYPE"] = specialModeType
	}

	if pathType, ok := alua["PATHTYPE"]; ok {
		data["PATHTYPE"] = pathType
	}

	resp, err := cli.put(url, data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("update iscsi initiator %s by %v error: %d", initiator, data, code)
	}

	return nil
}

func (cli *Client) AddIscsiInitiatorToHost(initiator, hostID string) error {
	url := fmt.Sprintf("/iscsi_initiator/%s", initiator)
	data := map[string]interface{}{
		"PARENTTYPE": 21,
		"PARENTID":   hostID,
	}
	resp, err := cli.put(url, data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Add iscsi initiator %s to host %s error: %d", initiator, hostID, code)
		return errors.New(msg)
	}

	return nil
}

func (cli *Client) AddGroupToMapping(groupType int, groupID, mappingID string) error {
	data := map[string]interface{}{
		"ID":               mappingID,
		"ASSOCIATEOBJTYPE": groupType,
		"ASSOCIATEOBJID":   groupID,
	}
	resp, err := cli.put("/mappingview/create_associate", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == HOSTGROUP_ALREADY_IN_MAPPING || code == LUNGROUP_ALREADY_IN_MAPPING {
		log.Infof("Group %s of type %d is already in mapping %s", groupID, groupType, mappingID)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Add group %s of type %d to mapping %s error: %d", groupID, groupType, mappingID, code)
		return errors.New(msg)
	}

	return nil
}

func (cli *Client) RemoveGroupFromMapping(groupType int, groupID, mappingID string) error {
	data := map[string]interface{}{
		"ID":               mappingID,
		"ASSOCIATEOBJTYPE": groupType,
		"ASSOCIATEOBJID":   groupID,
	}
	resp, err := cli.put("/mappingview/remove_associate", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == HOSTGROUP_NOT_IN_MAPPING ||
		code == LUNGROUP_NOT_IN_MAPPING {
		log.Infof("Group %s of type %d is not in mapping %s", groupID, groupType, mappingID)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Remove group %s of type %d from mapping %s error: %d", groupID, groupType, mappingID, code)
		return errors.New(msg)
	}

	return nil
}

func (cli *Client) GetLunCountOfHost(hostID string) (int64, error) {
	url := fmt.Sprintf("/lun/count?ASSOCIATEOBJTYPE=21&ASSOCIATEOBJID=%s", hostID)
	resp, err := cli.get(url)
	if err != nil {
		return 0, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get mapped lun count of host %s error: %d", hostID, code)
		return 0, errors.New(msg)
	}

	respData := resp.Data.(map[string]interface{})
	countStr := respData["COUNT"].(string)
	count, _ := strconv.ParseInt(countStr, 10, 64)
	return count, nil
}

func (cli *Client) GetLunCountOfMapping(mappingID string) (int64, error) {
	url := fmt.Sprintf("/lun/count?ASSOCIATEOBJTYPE=245&ASSOCIATEOBJID=%s", mappingID)
	resp, err := cli.get(url)
	if err != nil {
		return 0, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get mapped lun count of mapping %s error: %d", mappingID, code)
		return 0, errors.New(msg)
	}

	respData := resp.Data.(map[string]interface{})
	countStr := respData["COUNT"].(string)
	count, _ := strconv.ParseInt(countStr, 10, 64)
	return count, nil
}

func (cli *Client) CreateFileSystem(params map[string]interface{}) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME":          params["name"].(string),
		"PARENTID":      params["parentid"].(string),
		"CAPACITY":      params["capacity"].(int64),
		"DESCRIPTION":   params["description"].(string),
		"ALLOCTYPE":     params["alloctype"].(int),
		"ISSHOWSNAPDIR": false,
	}

	resp, err := cli.post("/filesystem", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Create filesystem %v error: %d", data, code)
		return nil, errors.New(msg)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}

func (cli *Client) DeleteFileSystem(id string) error {
	url := fmt.Sprintf("/filesystem/%s", id)
	resp, err := cli.delete(url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == FILESYSTEM_NOT_EXIST {
		log.Infof("Filesystem %s does not exist while deleting", id)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Delete filesystem %s error: %d", id, code)
		return errors.New(msg)
	}

	return nil
}

func (cli *Client) GetFileSystemByName(name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/filesystem?filter=NAME::%s&range=[0-100]", name)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get filesystem %s error: %d", name, code)
		return nil, errors.New(msg)
	}

	if resp.Data == nil {
		log.Infof("Filesystem %s does not exist", name)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) == 0 {
		return nil, nil
	}

	fs := respData[0].(map[string]interface{})
	return fs, nil
}

func (cli *Client) GetFileSystemByID(id string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/filesystem/%s", id)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get filesystem of ID %s error: %d", id, code)
		return nil, errors.New(msg)
	}

	fs := resp.Data.(map[string]interface{})
	return fs, nil
}

func (cli *Client) CreateNfsShare(params map[string]interface{}) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"SHAREPATH":   params["sharepath"].(string),
		"FSID":        params["fsid"].(string),
		"DESCRIPTION": params["description"].(string),
	}

	resp, err := cli.post("/NFSHARE", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == SHARE_ALREADY_EXIST || code == SHARE_PATH_ALREADY_EXIST {
		sharePath := params["sharepath"].(string)
		log.Infof("Nfs share %s already exists while creating", sharePath)

		share, err := cli.GetNfsShareByPath(sharePath)
		return share, err
	}
	if code != 0 {
		return nil, fmt.Errorf("Create nfs share %v error: %d", data, code)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}

func (cli *Client) DeleteNfsShare(id string) error {
	url := fmt.Sprintf("/NFSHARE/%s", id)
	resp, err := cli.delete(url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == SHARE_NOT_EXIST {
		log.Infof("Nfs share %s does not exist while deleting", id)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Delete nfs share %s error: %d", id, code)
		return errors.New(msg)
	}

	return nil
}

func (cli *Client) GetNfsShareByPath(path string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/NFSHARE?filter=SHAREPATH::%s&range=[0-100]", path)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == SHARE_PATH_INVALID {
		log.Infof("Nfs share of path %s does not exist", path)
		return nil, nil
	}
	if code != 0 {
		return nil, fmt.Errorf("Get nfs share of path %s error: %d", path, code)
	}

	if resp.Data == nil {
		log.Infof("Nfs share of path %s does not exist", path)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) == 0 {
		log.Infof("Nfs share of path %s does not exist", path)
		return nil, nil
	}

	share := respData[0].(map[string]interface{})
	return share, nil
}

func (cli *Client) GetNfsShareAccess(parentID, name string) (map[string]interface{}, error) {
	count, err := cli.GetNfsShareAccessCount(parentID)
	if err != nil {
		return nil, err
	}

	var i int64
	for i = 0; i < count; i += 100 { // Query per page 100
		clients, err := cli.GetNfsShareAccessRange(parentID, i, i+100)
		if err != nil {
			return nil, err
		}

		if clients == nil {
			return nil, nil
		}

		for _, ac := range clients {
			access := ac.(map[string]interface{})
			if access["NAME"].(string) == name {
				return access, nil
			}
		}
	}

	return nil, nil
}

func (cli *Client) GetNfsShareAccessCount(parentID string) (int64, error) {
	url := fmt.Sprintf("/NFS_SHARE_AUTH_CLIENT/count?filter=PARENTID::%s", parentID)
	resp, err := cli.get(url)
	if err != nil {
		return 0, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return 0, fmt.Errorf("Get nfs share access count of %s error: %d", parentID, code)
	}

	respData := resp.Data.(map[string]interface{})
	countStr := respData["COUNT"].(string)
	count, _ := strconv.ParseInt(countStr, 10, 64)

	return count, nil
}

func (cli *Client) GetNfsShareAccessRange(parentID string, startRange, endRange int64) ([]interface{}, error) {
	url := fmt.Sprintf("/NFS_SHARE_AUTH_CLIENT?filter=PARENTID::%s&range=[%d-%d]", parentID, startRange, endRange)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get nfs share access of %s error: %d", parentID, code)
	}

	if resp.Data == nil {
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	return respData, nil
}

func (cli *Client) AllowNfsShareAccess(params map[string]interface{}) error {
	resp, err := cli.post("/NFS_SHARE_AUTH_CLIENT", params)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Allow nfs share %v access error: %d", params, code)
	}

	return nil
}

func (cli *Client) DeleteNfsShareAccess(accessID string) error {
	url := fmt.Sprintf("/NFS_SHARE_AUTH_CLIENT/%s", accessID)

	resp, err := cli.delete(url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Delete nfs share %s access error: %d", accessID, code)
	}

	return nil
}

func (cli *Client) GetFCInitiator(wwn string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/fc_initiator?filter=ID::%s", wwn)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get fc initiator %s error: %d", wwn, code)
	}

	if resp.Data == nil {
		log.Infof("FC initiator %s does not exist", wwn)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	initiator := respData[0].(map[string]interface{})
	return initiator, nil
}

func (cli *Client) GetFCInitiatorByID(wwn string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/fc_initiator/%s", wwn)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get fc initiator by ID %s error: %d", wwn, code)
		return nil, errors.New(msg)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}

func (cli *Client) UpdateFCInitiator(wwn string, alua map[string]interface{}) error {
	url := fmt.Sprintf("/fc_initiator/%s", wwn)
	data := map[string]interface{}{}

	if multiPathType, ok := alua["MULTIPATHTYPE"]; ok {
		data["MULTIPATHTYPE"] = multiPathType
	}

	if failoverMode, ok := alua["FAILOVERMODE"]; ok {
		data["FAILOVERMODE"] = failoverMode
	}

	if specialModeType, ok := alua["SPECIALMODETYPE"]; ok {
		data["SPECIALMODETYPE"] = specialModeType
	}

	if pathType, ok := alua["PATHTYPE"]; ok {
		data["PATHTYPE"] = pathType
	}

	resp, err := cli.put(url, data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("update fc initiator %s by %v error: %d", wwn, data, code)
	}

	return nil
}

func (cli *Client) QueryFCInitiatorByHost(hostID string) ([]interface{}, error) {
	url := fmt.Sprintf("/fc_initiator?PARENTID=%s", hostID)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Query fc initiator of host %s error: %d", hostID, code)
		return nil, errors.New(msg)
	}
	if resp.Data == nil {
		log.Infof("No fc initiator associated to host %s", hostID)
		return nil, nil
	}

	initiators := resp.Data.([]interface{})
	return initiators, nil
}

func (cli *Client) AddFCInitiatorToHost(initiator, hostID string) error {
	url := fmt.Sprintf("/fc_initiator/%s", initiator)
	data := map[string]interface{}{
		"PARENTTYPE": 21,
		"PARENTID":   hostID,
	}
	resp, err := cli.put(url, data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Add FC initiator %s to host %s error: %d", initiator, hostID, code)
		return errors.New(msg)
	}

	return nil
}

func (cli *Client) GetIscsiInitiator(initiator string) (map[string]interface{}, error) {
	id := strings.Replace(initiator, ":", "\\:", -1)
	url := fmt.Sprintf("/iscsi_initiator?filter=ID::%s", id)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get ISCSI initiator %s error: %d", initiator, code)
		return nil, errors.New(msg)
	}

	if resp.Data == nil {
		log.Infof("ISCSI initiator %s does not exist", initiator)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	ini := respData[0].(map[string]interface{})
	return ini, nil
}

func (cli *Client) GetIscsiInitiatorByID(initiator string) (map[string]interface{}, error) {
	id := strings.Replace(initiator, ":", "\\:", -1)
	url := fmt.Sprintf("/iscsi_initiator/%s", id)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get ISCSI initiator by ID %s error: %d", initiator, code)
		return nil, errors.New(msg)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}

func (cli *Client) GetLicenseFeature() (map[string]int, error) {
	resp, err := cli.get("/license/feature")
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get license feature error: %d", code)
		return nil, errors.New(msg)
	}

	result := map[string]int{}

	if resp.Data == nil {
		return result, nil
	}

	respData := resp.Data.([]interface{})
	for _, i := range respData {
		feature := i.(map[string]interface{})
		for k, v := range feature {
			result[k] = int(v.(float64))
		}
	}
	return result, nil
}

func (cli *Client) GetSystem() (map[string]interface{}, error) {
	resp, err := cli.get("/system/")
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get system info error: %d", code)
		return nil, errors.New(msg)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}

func (cli *Client) UpdateLun(lunID string, params map[string]interface{}) error {
	url := fmt.Sprintf("/lun/%s", lunID)
	resp, err := cli.put(url, params)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Update LUN %s by params %v error: %d", lunID, params, code)
		return errors.New(msg)
	}

	return nil
}

func (cli *Client) UpdateFileSystem(fsID string, params map[string]interface{}) error {
	url := fmt.Sprintf("/filesystem/%s", fsID)
	resp, err := cli.put(url, params)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Update filesystem %s by params %v error: %d", fsID, params, code)
	}

	return nil
}

func (cli *Client) CreateQos(name, objID, objType string, params map[string]int) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME":              name,
		"SCHEDULEPOLICY":    1,
		"SCHEDULESTARTTIME": 1410969600,
		"STARTTIME":         "00:00",
		"DURATION":          86400,
	}

	if objType == "fs" {
		data["FSLIST"] = []string{objID}
	} else {
		data["LUNLIST"] = []string{objID}
	}

	for k, v := range params {
		data[k] = v
	}

	resp, err := cli.post("/ioclass", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Create qos %v error: %d", data, code)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}

func (cli *Client) ActivateQos(qosID string) error {
	data := map[string]interface{}{
		"ID":           qosID,
		"ENABLESTATUS": "true",
	}

	resp, err := cli.put("/ioclass/active", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Activate qos %s error: %d", qosID, code)
	}

	return nil
}

func (cli *Client) DeactivateQos(qosID string) error {
	data := map[string]interface{}{
		"ID":           qosID,
		"ENABLESTATUS": "false",
	}

	resp, err := cli.put("/ioclass/active", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Deactivate qos %s error: %d", qosID, code)
	}

	return nil
}

func (cli *Client) DeleteQos(qosID string) error {
	url := fmt.Sprintf("/ioclass/%s", qosID)

	resp, err := cli.delete(url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Delete qos %s error: %d", qosID, code)
	}

	return nil
}

func (cli *Client) GetQosByName(name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/ioclass?filter=NAME::%s", name)

	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get qos by name %s error: %d", name, code)
	}

	if resp.Data == nil {
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) <= 0 {
		return nil, nil
	}

	qos := respData[0].(map[string]interface{})
	return qos, nil
}

func (cli *Client) GetQosByID(qosID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/ioclass/%s", qosID)

	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get qos by ID %s error: %d", qosID, code)
	}

	qos := resp.Data.(map[string]interface{})
	return qos, nil
}

func (cli *Client) UpdateQos(qosID string, params map[string]interface{}) error {
	url := fmt.Sprintf("/ioclass/%s", qosID)
	resp, err := cli.put(url, params)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Update qos %s to %v error: %d", qosID, params, code)
	}

	return nil
}

func (cli *Client) GetIscsiTgtPort() ([]interface{}, error) {
	resp, err := cli.get("/iscsi_tgt_port")
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get ISCSI tgt port error: %d", code)
	}

	if resp.Data == nil {
		log.Infof("ISCSI tgt port does not exist")
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	return respData, nil
}

func (cli *Client) GetFCHostLink(hostID string) ([]interface{}, error) {
	url := fmt.Sprintf("/host_link?INITIATOR_TYPE=223&PARENTID=%s", hostID)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get FC host link of host %s error: %d", hostID, code)
	}

	if resp.Data == nil {
		log.Infof("There is no FC host link of host %s", hostID)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	return respData, nil
}

func (cli *Client) GetISCSIHostLink(hostID string) ([]interface{}, error) {
	url := fmt.Sprintf("/host_link?INITIATOR_TYPE=222&PARENTID=%s", hostID)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get ISCSI host link of host %s error: %d", hostID, code)
	}

	if resp.Data == nil {
		log.Infof("There is no ISCSI host link of host %s", hostID)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	return respData, nil
}

func (cli *Client) CreateLunSnapshot(name, lunID string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME":        name,
		"DESCRIPTION": "Created from Kubernetes",
		"PARENTID":    lunID,
	}

	resp, err := cli.post("/snapshot", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Create snapshot %s for lun %s error: %d", name, lunID, code)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}

func (cli *Client) GetLunSnapshotByName(name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/snapshot?filter=NAME::%s&range=[0-100]", name)

	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get snapshot by name %s error: %d", name, code)
	}

	if resp.Data == nil {
		log.Infof("Snapshot %s does not exist", name)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) <= 0 {
		return nil, nil
	}

	snapshot := respData[0].(map[string]interface{})
	return snapshot, nil
}

func (cli *Client) DeleteLunSnapshot(snapshotID string) error {
	url := fmt.Sprintf("/snapshot/%s", snapshotID)
	resp, err := cli.delete(url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == LUN_SNAPSHOT_NOT_EXIST {
		log.Infof("Lun snapshot %s does not exist while deleting", snapshotID)
		return nil
	}
	if code != 0 {
		return fmt.Errorf("Delete snapshot %s error: %d", snapshotID, code)
	}

	return nil
}

func (cli *Client) ActivateLunSnapshot(snapshotID string) error {
	data := map[string]interface{}{
		"SNAPSHOTLIST": []string{snapshotID},
	}

	resp, err := cli.post("/snapshot/activate", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Activate snapshot %s error: %d", snapshotID, code)
	}

	return nil
}

func (cli *Client) DeactivateLunSnapshot(snapshotID string) error {
	data := map[string]interface{}{
		"ID": snapshotID,
	}

	resp, err := cli.put("/snapshot/stop", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == SNAPSHOT_NOT_ACTIVATED {
		return nil
	}
	if code != 0 {
		return fmt.Errorf("Deactivate snapshot %s error: %d", snapshotID, code)
	}

	return nil
}

func (cli *Client) CreateLunCopy(name, srcLunID, dstLunID string, copySpeed int) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME":      name,
		"COPYSPEED": copySpeed,
		"SOURCELUN": fmt.Sprintf("INVALID;%s;INVALID;INVALID;INVALID", srcLunID),
		"TARGETLUN": fmt.Sprintf("INVALID;%s;INVALID;INVALID;INVALID", dstLunID),
	}

	resp, err := cli.post("/LUNCOPY", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Create luncopy from %s to %s error: %d", srcLunID, dstLunID, code)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}

func (cli *Client) GetLunCopyByID(lunCopyID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/LUNCOPY/%s", lunCopyID)

	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get luncopy %s error: %d", lunCopyID, code)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}

func (cli *Client) GetLunCopyByName(name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/LUNCOPY?filter=NAME::%s", name)

	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get luncopy by name %s error: %d", name, code)
	}

	if resp.Data == nil {
		log.Infof("Luncopy %s does not exist", name)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) <= 0 {
		log.Infof("Luncopy %s does not exist", name)
		return nil, nil
	}

	luncopy := respData[0].(map[string]interface{})
	return luncopy, nil
}

func (cli *Client) StartLunCopy(lunCopyID string) error {
	data := map[string]interface{}{
		"ID": lunCopyID,
	}

	resp, err := cli.put("/LUNCOPY/start", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Start luncopy %s error: %d", lunCopyID, code)
	}

	return nil
}

func (cli *Client) StopLunCopy(lunCopyID string) error {
	data := map[string]interface{}{
		"ID": lunCopyID,
	}

	resp, err := cli.put("/LUNCOPY/stop", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Stop luncopy %s error: %d", lunCopyID, code)
	}

	return nil
}

func (cli *Client) DeleteLunCopy(lunCopyID string) error {
	url := fmt.Sprintf("/LUNCOPY/%s", lunCopyID)

	resp, err := cli.delete(url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == LUNCOPY_NOT_EXIST {
		log.Infof("Luncopy %s does not exist while deleting", lunCopyID)
		return nil
	}
	if code != 0 {
		return fmt.Errorf("Delete luncopy %s error: %d", lunCopyID, code)
	}

	return nil
}

func (cli *Client) CreateFSSnapshot(name, parentID string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME":        name,
		"DESCRIPTION": "Created from Kubernetes",
		"PARENTID":    parentID,
		"PARENTTYPE":  "40",
	}

	resp, err := cli.post("/FSSNAPSHOT", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Create snapshot %s for FS %s error: %d", name, parentID, code)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}

func (cli *Client) DeleteFSSnapshot(snapshotID string) error {
	url := fmt.Sprintf("/FSSNAPSHOT/%s", snapshotID)
	resp, err := cli.delete(url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == FS_SNAPSHOT_NOT_EXIST {
		log.Infof("FS Snapshot %s does not exist while deleting", snapshotID)
		return nil
	}
	if code != 0 {
		return fmt.Errorf("Delete FS snapshot %s error: %d", snapshotID, code)
	}

	return nil
}

func (cli *Client) GetFSSnapshotByName(parentID, snapshotName string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/FSSNAPSHOT?PARENTID=%s&filter=NAME::%s", parentID, snapshotName)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		if code == SNAPSHOT_PARENT_NOT_EXIST {
			log.Infof("The parent filesystem %s of snapshot %s does not exist", parentID, snapshotName)
			return nil, nil
		}

		return nil, fmt.Errorf("failed to get filesystem snapshot %s, error is %d", snapshotName, code)
	}

	if resp.Data == nil {
		log.Infof("Filesystem snapshot %s does not exist", snapshotName)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) <= 0 {
		return nil, nil
	}

	snapshot := respData[0].(map[string]interface{})
	return snapshot, nil
}

func (cli *Client) GetFSSnapshotCountByParentId(ParentId string) (int, error) {
	url := fmt.Sprintf("/FSSNAPSHOT/count?PARENTID=%s", ParentId)
	resp, err := cli.get(url)
	if err != nil {
		return 0, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("failed to get snapshot count of filesystem %s, error is %d", ParentId, code)
		return 0, errors.New(msg)
	}

	respData := resp.Data.(map[string]interface{})
	countStr := respData["COUNT"].(string)
	count, _ := strconv.Atoi(countStr)
	return count, nil
}

func (cli *Client) CloneFileSystem(name string, allocType int, parentID, parentSnapshotID string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME":               name,
		"ALLOCTYPE":          allocType,
		"DESCRIPTION":        "Created from Kubernetes",
		"PARENTFILESYSTEMID": parentID,
	}

	if parentSnapshotID != "" {
		data["PARENTSNAPSHOTID"] = parentSnapshotID
	}

	resp, err := cli.post("/filesystem", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Clone FS from %s error: %d", parentID, code)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}

func (cli *Client) SplitCloneFS(fsID string, splitSpeed int, isDeleteParentSnapshot bool) error {
	data := map[string]interface{}{
		"ID":                     fsID,
		"SPLITENABLE":            true,
		"SPLITSPEED":             splitSpeed,
		"ISDELETEPARENTSNAPSHOT": isDeleteParentSnapshot,
	}

	resp, err := cli.put("/filesystem_split_switch", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Split FS %s error: %d", fsID, code)
	}

	return nil
}

func (cli *Client) StopCloneFSSplit(fsID string) error {
	data := map[string]interface{}{
		"ID":          fsID,
		"SPLITENABLE": false,
	}

	resp, err := cli.put("/filesystem_split_switch", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Stop FS %s splitting error: %d", fsID, code)
	}

	return nil
}

func (cli *Client) ExtendFileSystem(fsID string, newCapacity int64) error {
	url := fmt.Sprintf("/filesystem/%s", fsID)
	data := map[string]interface{}{
		"CAPACITY": newCapacity,
	}

	resp, err := cli.put(url, data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Extend FS capacity to %d error: %d", newCapacity, code)
	}

	return nil
}

func (cli *Client) ExtendLun(lunID string, newCapacity int64) error {
	data := map[string]interface{}{
		"CAPACITY": newCapacity,
		"ID":       lunID,
	}

	resp, err := cli.put("/lun/expand", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Extend LUN capacity to %d error: %d", newCapacity, code)
	}

	return nil
}

func (cli *Client) GetHyperMetroDomainByName(name string) (map[string]interface{}, error) {
	resp, err := cli.get("/HyperMetroDomain?range=[0-100]")
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get HyperMetroDomain of name %s error: %d", name, code)
	}
	if resp.Data == nil {
		log.Infof("No HyperMetroDomain %s exist", name)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	for _, i := range respData {
		domain := i.(map[string]interface{})
		if domain["NAME"].(string) == name {
			return domain, nil
		}
	}

	return nil, nil
}

func (cli *Client) GetHyperMetroDomain(domainID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/HyperMetroDomain/%s", domainID)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get HyperMetroDomain %s error: %d", domainID, code)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}

func (cli *Client) CreateHyperMetroPair(data map[string]interface{}) (map[string]interface{}, error) {
	resp, err := cli.post("/HyperMetroPair", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Create hypermetro %v error: %d", data, code)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}

func (cli *Client) SyncHyperMetroPair(pairID string) error {
	data := map[string]interface{}{
		"ID": pairID,
	}

	resp, err := cli.put("/HyperMetroPair/synchronize_hcpair", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Sync hypermetro %s error: %d", pairID, code)
	}

	return nil
}

func (cli *Client) StopHyperMetroPair(pairID string) error {
	data := map[string]interface{}{
		"ID": pairID,
	}

	resp, err := cli.put("/HyperMetroPair/disable_hcpair", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Stop hypermetro %s error: %d", pairID, code)
	}

	return nil
}

func (cli *Client) GetHyperMetroPair(pairID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/HyperMetroPair?filter=ID::%s", pairID)

	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get hypermetro %s error: %d", pairID, code)
	}

	if resp.Data == nil {
		log.Infof("Hypermetro %s does not exist", pairID)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) <= 0 {
		log.Infof("Hypermetro %s does not exist", pairID)
		return nil, nil
	}

	pair := respData[0].(map[string]interface{})
	return pair, nil
}

func (cli *Client) DeleteHyperMetroPair(pairID string) error {
	url := fmt.Sprintf("/HyperMetroPair/%s", pairID)

	resp, err := cli.delete(url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == HYPERMETRO_NOT_EXIST {
		log.Infof("Hypermetro %s to delete does not exist", pairID)
		return nil
	} else if code != 0 {
		return fmt.Errorf("Delete hypermetro %s error: %d", pairID, code)
	}

	return nil
}

func (cli *Client) GetHyperMetroPairByLocalObjID(objID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/HyperMetroPair?filter=LOCALOBJID::%s", objID)

	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get hypermetro of local obj %s error: %d", objID, code)
	}

	if resp.Data == nil {
		log.Infof("Hypermetro of local obj %s does not exist", objID)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	for _, i := range respData {
		pair := i.(map[string]interface{})
		if pair["LOCALOBJID"] == objID {
			return pair, nil
		}
	}

	log.Infof("Hypermetro of local obj %s does not exist", objID)
	return nil, nil
}

func (cli *Client) CreateClonePair(srcLunID, dstLunID string, cloneSpeed int) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"copyRate":          cloneSpeed,
		"sourceID":          srcLunID,
		"targetID":          dstLunID,
		"isNeedSynchronize": "0",
	}

	resp, err := cli.post("/clonepair/relation", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Create ClonePair from %s to %s, error: %d", srcLunID, dstLunID, code)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}

func (cli *Client) SyncClonePair(clonePairID string) error {
	data := map[string]interface{}{
		"ID":         clonePairID,
		"copyAction": 0,
	}

	resp, err := cli.put("/clonepair/synchronize", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Sync ClonePair %s error: %d", clonePairID, code)
	}

	return nil
}

func (cli *Client) DeleteClonePair(clonePairID string) error {
	data := map[string]interface{}{
		"ID":             clonePairID,
		"isDeleteDstLun": false,
	}

	resp, err := cli.delete("/clonepair", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == CLONEPAIR_NOT_EXIST {
		log.Infof("ClonePair %s does not exist while deleting", clonePairID)
		return nil
	}
	if code != 0 {
		return fmt.Errorf("Delete ClonePair %s error: %d", clonePairID, code)
	}

	return nil
}

func (cli *Client) GetClonePairInfo(clonePairID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/clonepair?filter=ID::%s", clonePairID)

	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get ClonePair info %s error: %d", clonePairID, code)
	}

	if resp.Data == nil {
		log.Infof("clonePair %s does not exist", clonePairID)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) <= 0 {
		log.Infof("clonePair %s does not exist", clonePairID)
		return nil, nil
	}

	clonePair := respData[0].(map[string]interface{})
	return clonePair, nil
}

func (cli *Client) GetRemoteDeviceBySN(sn string) (map[string]interface{}, error) {
	resp, err := cli.get("/remote_device")
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get remote device %s error: %d", sn, code)
	}

	if resp.Data == nil {
		log.Infof("Remote device %s does not exist", sn)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	for _, i := range respData {
		device := i.(map[string]interface{})
		if device["SN"] == sn {
			return device, nil
		}
	}

	return nil, nil
}

func (cli *Client) CreateReplicationPair(data map[string]interface{}) (map[string]interface{}, error) {
	resp, err := cli.post("/REPLICATIONPAIR", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Create replication %v error: %d", data, code)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}

func (cli *Client) SplitReplicationPair(pairID string) error {
	data := map[string]interface{}{
		"ID": pairID,
	}

	resp, err := cli.put("/REPLICATIONPAIR/split", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Split replication pair %s error: %d", pairID, code)
	}

	return nil
}

func (cli *Client) SyncReplicationPair(pairID string) error {
	data := map[string]interface{}{
		"ID": pairID,
	}

	resp, err := cli.put("/REPLICATIONPAIR/sync", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Sync replication pair %s error: %d", pairID, code)
	}

	return nil
}

func (cli *Client) DeleteReplicationPair(pairID string) error {
	url := fmt.Sprintf("/REPLICATIONPAIR/%s", pairID)
	resp, err := cli.delete(url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == REPLICATION_NOT_EXIST {
		log.Infof("Replication pair %s does not exist while deleting", pairID)
		return nil
	}
	if code != 0 {
		return fmt.Errorf("Delete replication pair %s error: %d", pairID, code)
	}

	return nil
}

func (cli *Client) GetReplicationPairByResID(resID string, resType int) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("/REPLICATIONPAIR/associate?ASSOCIATEOBJTYPE=%d&ASSOCIATEOBJID=%s", resType, resID)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("get replication pairs resource %s associated error: %d", resID, code)
	}

	if resp.Data == nil {
		log.Infof("Replication pairs resource %s associated does not exist", resID)
		return nil, nil
	}

	var pairs []map[string]interface{}

	respData := resp.Data.([]interface{})
	for _, i := range respData {
		pairs = append(pairs, i.(map[string]interface{}))
	}

	return pairs, nil
}

func (cli *Client) GetReplicationPairByID(pairID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/REPLICATIONPAIR/%s", pairID)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get replication pair %s error: %d", pairID, code)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}

func (cli *Client) GetReplicationvStorePairCount() (int64, error) {
	resp, err := cli.get("/replication_vstorepair/count")
	if err != nil {
		return 0, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return 0, fmt.Errorf("Get replication vstore pair count error: %d", code)
	}

	respData := resp.Data.(map[string]interface{})
	countStr := respData["COUNT"].(string)
	count, _ := strconv.ParseInt(countStr, 10, 64)

	return count, nil
}

func (cli *Client) GetReplicationvStorePairRange(startRange, endRange int64) ([]interface{}, error) {
	url := fmt.Sprintf("/replication_vstorepair?range=[%d-%d]", startRange, endRange)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get replication vstore pairs error: %d", code)
	}

	if resp.Data == nil {
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	return respData, nil
}

func (cli *Client) GetReplicationvStorePairByvStore(vStoreID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/replication_vstorepair/associate?ASSOCIATEOBJTYPE=16442&ASSOCIATEOBJID=%s", vStoreID)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get replication vstore pair by vstore %s error: %d", vStoreID, code)
	}
	if resp.Data == nil {
		log.Infof("Replication vstore pair of vstore %s does not exist", vStoreID)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) == 0 {
		log.Infof("Replication vstore pair of vstore %s does not exist", vStoreID)
		return nil, nil
	}

	pair := respData[0].(map[string]interface{})
	return pair, nil
}

func (cli *Client) GetvStoreByName(name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/vstore?filter=NAME::%s", name)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get vstore %s error: %d", name, code)
	}
	if resp.Data == nil {
		log.Infof("vstore %s does not exist", name)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) == 0 {
		log.Infof("vstore %s does not exist", name)
		return nil, nil
	}

	vstore := respData[0].(map[string]interface{})
	return vstore, nil
}

func (cli *Client) GetvStorePairByID(pairID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/vstore_pair?filter=ID::%s", pairID)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get vstore pair by ID %s error: %d", pairID, code)
	}
	if resp.Data == nil {
		log.Infof("vstore pair %s does not exist", pairID)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) == 0 {
		log.Infof("vstore pair %s does not exist", pairID)
		return nil, nil
	}

	pair := respData[0].(map[string]interface{})
	return pair, nil
}

func (cli *Client) GetRoCEInitiator(initiator string) (map[string]interface{}, error) {
	id := URL.QueryEscape(strings.Replace(initiator, ":", "\\:", -1))
	url := fmt.Sprintf("/NVMe_over_RoCE_initiator?filter=ID::%s", id)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("get RoCE initiator %s error: %d", initiator, code)
	}

	if resp.Data == nil {
		log.Infof("RoCE initiator %s does not exist", initiator)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) == 0 {
		return nil, nil
	}
	ini := respData[0].(map[string]interface{})
	return ini, nil
}

func (cli *Client) GetRoCEInitiatorByID(initiator string) (map[string]interface{}, error) {
	id := URL.QueryEscape(strings.Replace(initiator, ":", "\\:", -1))
	url := fmt.Sprintf("/NVMe_over_RoCE_initiator/%s", id)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("get RoCE initiator by ID %s error: %d", initiator, code)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}

func (cli *Client) AddRoCEInitiator(initiator string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"ID": initiator,
	}

	resp, err := cli.post("/NVMe_over_RoCE_initiator", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == OBJECT_ID_NOT_UNIQUE {
		log.Infof("RoCE initiator %s already exists", initiator)
		return cli.GetRoCEInitiatorByID(initiator)
	}
	if code != 0 {
		return nil, fmt.Errorf("add RoCE initiator %s error: %d", initiator, code)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}

func (cli *Client) AddRoCEInitiatorToHost(initiator, hostID string) error {
	data := map[string]interface{}{
		"ID":               hostID,
		"ASSOCIATEOBJTYPE": 57870,
		"ASSOCIATEOBJID":   initiator,
	}
	resp, err := cli.put("/host/create_associate", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("add RoCE initiator %s to host %s error: %d", initiator, hostID, code)
	}

	return nil
}

func (cli *Client) GetRoCEPortalByIP(tgtPortal string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/lif?filter=IPV4ADDR::%s", tgtPortal)
	resp, err := cli.get(url)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("get RoCE by IP %s error: %d", tgtPortal, code)
	}
	if resp.Data == nil {
		log.Infof("RoCE portal %s does not exist", tgtPortal)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) == 0 {
		log.Infof("RoCE portal %s does not exist", tgtPortal)
		return nil, nil
	}

	portal := respData[0].(map[string]interface{})
	return portal, nil
}

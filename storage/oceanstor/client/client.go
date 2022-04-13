package client

import (
	"bytes"
	"context"
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

	"huawei-csi-driver/utils"
	"huawei-csi-driver/utils/log"
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
	SNAPSHOT_PARENT_NOT_EXIST_V3 int64 = 1073754117
	SNAPSHOT_PARENT_NOT_EXIST_V6 int64 = 1073754136
	SMARTQOS_ALREADY_EXIST       int64 = 1077948993
	SYSTEM_BUSY                  int64 = 1077949006
	MSG_TIME_OUT                 int64 = 1077949001
	DEFAULT_PARALLEL_COUNT       int   = 50
	MAX_PARALLEL_COUNT           int   = 1000
	MIN_PARALLEL_COUNT           int   = 20
	GET_INFO_WAIT_INTERNAL             = 10

	exceedFSCapacityUpper int64 = 1073844377
	lessFSCapacityLower   int64 = 1073844376
	parameterIncorrect    int64 = 50331651

	localFilesystem      string = "0"
	hyperMetroFilesystem string = "1"
)

var (
	loggerFilter = map[string]map[string]bool{
		"POST": map[string]bool{
			"/xx/sessions": true,
		},
		"GET": map[string]bool{
			"/storagepool":     true,
			"/license/feature": true,
		},
	}

	logRegexFilter = map[string][]string{
		"GET": {
			`/vstore_pair\?filter=ID`,
			`/FsHyperMetroDomain\?RUNNINGSTATUS=0`,
		},
	}

	clientSemaphore *utils.Semaphore
)

func logFilter(method, url string) bool {
	if filter, exist := loggerFilter[method]; exist && filter[url] {
		return true
	}

	if filter, exist := logRegexFilter[method]; exist {
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
	client     HTTPClient
	vstoreName string

	reloginMutex sync.Mutex
}

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

var newHTTPClient = func() HTTPClient {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
		Jar:     jar,
		Timeout: 60 * time.Second,
	}
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
			log.Warningf("The config parallelNum %d is invalid, set it to the default value %d",
				parallelCount, DEFAULT_PARALLEL_COUNT)
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
		client:     newHTTPClient(),
	}
}

func (cli *Client) call(ctx context.Context,
	method string, url string,
	data map[string]interface{}) (Response, error) {
	var r Response
	var err error

	r, err = cli.baseCall(ctx, method, url, data)
	if (err != nil && err.Error() == "unconnected") ||
		(r.Error != nil && int64(r.Error["code"].(float64)) == -401) {
		// Current connection fails, try to relogin to other urls if exist,
		// if relogin success, resend the request again.
		log.AddContext(ctx).Infof("Try to relogin and resend request method: %s, url: %s", method, url)

		err = cli.reLogin(ctx)
		if err == nil {
			r, err = cli.baseCall(ctx, method, url, data)
		}
	}

	return r, err
}

func (cli *Client) getRequest(ctx context.Context,
	method string, url string,
	data map[string]interface{}) (*http.Request, error) {
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
			log.AddContext(ctx).Errorf("json.Marshal data %v error: %v", data, err)
			return req, err
		}
		reqBody = bytes.NewReader(reqBytes)
	}

	req, err = http.NewRequest(method, reqUrl, reqBody)
	if err != nil {
		log.AddContext(ctx).Errorf("Construct http request error: %s", err.Error())
		return req, err
	}

	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Content-Type", "application/json")

	if cli.token != "" {
		req.Header.Set("iBaseToken", cli.token)
	}

	return req, nil
}

func (cli *Client) baseCall(ctx context.Context,
	method string, url string,
	data map[string]interface{}) (Response, error) {
	var r Response
	var req *http.Request
	var err error

	reqUrl := cli.url
	reqUrl += url

	if url != "/xx/sessions" && url != "/sessions" {
		cli.reloginMutex.Lock()
		req, err = cli.getRequest(ctx, method, url, data)
		cli.reloginMutex.Unlock()
	} else {
		req, err = cli.getRequest(ctx, method, url, data)
	}

	if err != nil {
		return r, err
	}

	if !logFilter(method, url) {
		log.AddContext(ctx).Infof("Request method: %s, url: %s, body: %v", method, reqUrl, data)
	}

	clientSemaphore.Acquire()
	defer clientSemaphore.Release()

	resp, err := cli.client.Do(req)
	if err != nil {
		log.AddContext(ctx).Errorf("Send request method: %s, url: %s, error: %v", method, reqUrl, err)
		return r, errors.New("unconnected")
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.AddContext(ctx).Errorf("Read response data error: %v", err)
		return r, err
	}

	if !logFilter(method, url) {
		log.AddContext(ctx).Infof("Response method: %s, url: %s, body: %s", method, reqUrl, body)
	}

	err = json.Unmarshal(body, &r)
	if err != nil {
		log.AddContext(ctx).Errorf("json.Unmarshal data %s error: %v", body, err)
		return r, err
	}

	return r, nil
}

func (cli *Client) get(ctx context.Context, url string, data map[string]interface{}) (Response, error) {
	return cli.call(ctx, "GET", url, data)
}

func (cli *Client) post(ctx context.Context, url string, data map[string]interface{}) (Response, error) {
	return cli.call(ctx, "POST", url, data)
}

func (cli *Client) put(ctx context.Context, url string, data map[string]interface{}) (Response, error) {
	return cli.call(ctx, "PUT", url, data)
}

func (cli *Client) delete(ctx context.Context, url string, data map[string]interface{}) (Response, error) {
	return cli.call(ctx, "DELETE", url, data)
}

func (cli *Client) DuplicateClient() *Client {
	dup := *cli

	dup.urls = make([]string, len(cli.urls))
	copy(dup.urls, cli.urls)

	dup.client = nil

	return &dup
}

func (cli *Client) Login(ctx context.Context) error {
	var resp Response
	var err error

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
		cli.url = url + "/deviceManager/rest"

		log.AddContext(ctx).Infof("Try to login %s", cli.url)
		resp, err = cli.baseCall(context.Background(), "POST", "/xx/sessions", data)
		if err == nil {
			/* Sort the login url to the last slot of san addresses, so that
			   if this connection error, next time will try other url first. */
			cli.urls[i], cli.urls[len(cli.urls)-1] = cli.urls[len(cli.urls)-1], cli.urls[i]
			break
		} else if err.Error() != "unconnected" {
			log.AddContext(ctx).Errorf("Login %s error", cli.url)
			break
		}

		log.AddContext(ctx).Warningf("Login %s error due to connection failure, gonna try another url",
			cli.url)
	}

	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Login %s error: %+v", cli.url, resp)
		return errors.New(msg)
	}

	respData := resp.Data.(map[string]interface{})
	cli.deviceid = respData["deviceid"].(string)
	cli.token = respData["iBaseToken"].(string)

	log.AddContext(ctx).Infof("Login %s success", cli.url)
	return nil
}

func (cli *Client) Logout(ctx context.Context) {
	resp, err := cli.baseCall(ctx, "DELETE", "/sessions", nil)
	if err != nil {
		log.AddContext(ctx).Warningf("Logout %s error: %v", cli.url, err)
		return
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		log.AddContext(ctx).Warningf("Logout %s error: %d", cli.url, code)
		return
	}

	log.AddContext(ctx).Infof("Logout %s success", cli.url)
}

func (cli *Client) reLogin(ctx context.Context) error {
	oldToken := cli.token

	cli.reloginMutex.Lock()
	defer cli.reloginMutex.Unlock()

	if cli.token != "" && oldToken != cli.token {
		// Coming here indicates other thread had already done relogin, so no need to relogin again
		return nil
	} else if cli.token != "" {
		cli.Logout(ctx)
	}

	err := cli.Login(ctx)
	if err != nil {
		log.AddContext(ctx).Errorf("Try to relogin error: %v", err)
		return err
	}

	return nil
}

func (cli *Client) GetvStoreName() string {
	return cli.vstoreName
}

func (cli *Client) GetLunByName(ctx context.Context, name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/lun?filter=NAME::%s&range=[0-100]", name)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get lun %s info error: %d", name, code)
		return nil, errors.New(msg)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Lun %s does not exist", name)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) <= 0 {
		log.AddContext(ctx).Infof("Lun %s does not exist", name)
		return nil, nil
	}

	lun := respData[0].(map[string]interface{})
	return lun, nil
}

func (cli *Client) GetLunByID(ctx context.Context, id string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/lun/%s", id)
	resp, err := cli.get(ctx, url, nil)
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

func (cli *Client) AddLunToGroup(ctx context.Context, lunID string, groupID string) error {
	data := map[string]interface{}{
		"ID":               groupID,
		"ASSOCIATEOBJTYPE": "11",
		"ASSOCIATEOBJID":   lunID,
	}

	resp, err := cli.post(ctx, "/lungroup/associate", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == OBJECT_ID_NOT_UNIQUE || code == LUN_ALREADY_IN_GROUP {
		log.AddContext(ctx).Warningf("Lun %s is already in group %s", lunID, groupID)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Add lun %s to group %s error: %d", lunID, groupID, code)
		return errors.New(msg)
	}

	return nil
}

func (cli *Client) RemoveLunFromGroup(ctx context.Context, lunID, groupID string) error {
	data := map[string]interface{}{
		"ID":               groupID,
		"ASSOCIATEOBJTYPE": "11",
		"ASSOCIATEOBJID":   lunID,
	}

	resp, err := cli.delete(ctx, "/lungroup/associate", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == OBJECT_NOT_EXIST {
		log.AddContext(ctx).Warningf("LUN %s is not in lungroup %s", lunID, groupID)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Remove lun %s from group %s error: %d", lunID, groupID, code)
		return errors.New(msg)
	}

	return nil
}

func (cli *Client) GetLunGroupByName(ctx context.Context, name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/lungroup?filter=NAME::%s", name)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get lungroup %s info error: %d", name, code)
		return nil, errors.New(msg)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Lungroup %s does not exist", name)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) <= 0 {
		log.AddContext(ctx).Infof("Lungroup %s does not exist", name)
		return nil, nil
	}

	group := respData[0].(map[string]interface{})
	return group, nil
}

func (cli *Client) CreateLunGroup(ctx context.Context, name string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME":    name,
		"APPTYPE": 0,
	}
	resp, err := cli.post(ctx, "/lungroup", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == OBJECT_NAME_ALREADY_EXIST {
		log.AddContext(ctx).Infof("Lungroup %s already exists", name)
		return cli.GetLunGroupByName(ctx, name)
	}
	if code != 0 {
		msg := fmt.Sprintf("Create lungroup %s error: %d", name, code)
		return nil, errors.New(msg)
	}

	lunGroup := resp.Data.(map[string]interface{})
	return lunGroup, nil
}

func (cli *Client) DeleteLunGroup(ctx context.Context, id string) error {
	url := fmt.Sprintf("/lungroup/%s", id)
	resp, err := cli.delete(ctx, url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == OBJECT_NOT_EXIST {
		log.AddContext(ctx).Infof("Lungroup %s does not exist while deleting", id)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Delete lungroup %s error: %d", id, code)
		return errors.New(msg)
	}

	return nil
}

func (cli *Client) QueryAssociateLunGroup(ctx context.Context, objType int, objID string) ([]interface{}, error) {
	url := fmt.Sprintf("/lungroup/associate?ASSOCIATEOBJTYPE=%d&ASSOCIATEOBJID=%s", objType, objID)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("associate query lungroup by obj %s of type %d error: %d", objID, objType, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("obj %s of type %d doesn't associate to any lungroup", objID, objType)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	return respData, nil
}

func (cli *Client) CreateLun(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME":        params["name"].(string),
		"PARENTID":    params["parentid"].(string),
		"CAPACITY":    params["capacity"].(int64),
		"DESCRIPTION": params["description"].(string),
		"ALLOCTYPE":   params["alloctype"].(int),
	}
	if val, ok := params["workloadTypeID"].(string); ok {
		data["WORKLOADTYPEID"] = val
	}

	resp, err := cli.post(ctx, "/lun", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == parameterIncorrect {
		return nil, fmt.Errorf("create Lun error. ErrorCode: %d. Reason: The input parameter is incorrect. "+
			"Suggestion: delete current PVC and check the parameter of the storageClass and PVC and try again", code)
	}

	if code != 0 {
		return nil, fmt.Errorf("create volume %v error: %d", data, code)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}

func (cli *Client) DeleteLun(ctx context.Context, id string) error {
	url := fmt.Sprintf("/lun/%s", id)
	resp, err := cli.delete(ctx, url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == LUN_NOT_EXIST {
		log.AddContext(ctx).Infof("Lun %s does not exist while deleting", id)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Delete lun %s error: %d", id, code)
		return errors.New(msg)
	}

	return nil
}

func (cli *Client) GetPoolByName(ctx context.Context, name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/storagepool?filter=NAME::%s&range=[0-100]", name)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get pool %s info error: %d", name, code)
		return nil, errors.New(msg)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Pool %s does not exist", name)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) <= 0 {
		log.AddContext(ctx).Infof("Pool %s does not exist", name)
		return nil, nil
	}

	pool := respData[0].(map[string]interface{})
	return pool, nil
}

func (cli *Client) GetAllPools(ctx context.Context) (map[string]interface{}, error) {
	resp, err := cli.get(ctx, "/storagepool", nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get all pools info error: %d", code)
		return nil, errors.New(msg)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("There's no pools exist")
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

func (cli *Client) CreateHost(ctx context.Context, name string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME":            name,
		"OPERATIONSYSTEM": 0,
	}

	resp, err := cli.post(ctx, "/host", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == OBJECT_NAME_ALREADY_EXIST {
		log.AddContext(ctx).Infof("Host %s already exists", name)
		return cli.GetHostByName(ctx, name)
	}
	if code != 0 {
		msg := fmt.Sprintf("Create host %s error: %d", name, code)
		return nil, errors.New(msg)
	}

	host := resp.Data.(map[string]interface{})
	return host, nil
}

func (cli *Client) UpdateHost(ctx context.Context, id string, alua map[string]interface{}) error {
	url := fmt.Sprintf("/host/%s", id)
	data := map[string]interface{}{}

	if accessMode, ok := alua["accessMode"]; ok {
		data["accessMode"] = accessMode
	}

	if hyperMetroPathOptimized, ok := alua["hyperMetroPathOptimized"]; ok {
		data["hyperMetroPathOptimized"] = hyperMetroPathOptimized
	}

	resp, err := cli.put(ctx, url, data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("update host %s by %v error: %d", id, data, code)
	}

	return nil
}

func (cli *Client) GetHostByName(ctx context.Context, name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/host?filter=NAME::%s&range=[0-100]", name)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get host %s error: %d", name, code)
		return nil, errors.New(msg)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Host %s does not exist", name)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) <= 0 {
		log.AddContext(ctx).Infof("Host %s does not exist", name)
		return nil, nil
	}

	host := respData[0].(map[string]interface{})
	return host, nil
}

func (cli *Client) DeleteHost(ctx context.Context, id string) error {
	url := fmt.Sprintf("/host/%s", id)
	resp, err := cli.delete(ctx, url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == HOST_NOT_EXIST {
		log.AddContext(ctx).Infof("Host %s does not exist", id)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Delete host %s error: %d", id, code)
		return errors.New(msg)
	}

	return nil
}

func (cli *Client) CreateHostGroup(ctx context.Context, name string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME": name,
	}
	resp, err := cli.post(ctx, "/hostgroup", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == OBJECT_NAME_ALREADY_EXIST {
		log.AddContext(ctx).Infof("Hostgroup %s already exists", name)
		return cli.GetHostGroupByName(ctx, name)
	}
	if code != 0 {
		msg := fmt.Sprintf("Create hostgroup %s error: %d", name, code)
		return nil, errors.New(msg)
	}

	hostGroup := resp.Data.(map[string]interface{})
	return hostGroup, nil
}

func (cli *Client) GetHostGroupByName(ctx context.Context, name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/hostgroup?filter=NAME::%s", name)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get hostgroup %s error: %d", name, code)
		return nil, errors.New(msg)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Hostgroup %s does not exist", name)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) <= 0 {
		log.AddContext(ctx).Infof("Hostgroup %s does not exist", name)
		return nil, nil
	}

	hostGroup := respData[0].(map[string]interface{})
	return hostGroup, nil
}

func (cli *Client) DeleteHostGroup(ctx context.Context, id string) error {
	url := fmt.Sprintf("/hostgroup/%s", id)
	resp, err := cli.delete(ctx, url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == HOSTGROUP_NOT_EXIST {
		log.AddContext(ctx).Infof("Hostgroup %s does not exist while deleting", id)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Delete hostgroup %s error: %d", id, code)
		return errors.New(msg)
	}

	return nil
}

func (cli *Client) CreateMapping(ctx context.Context, name string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME": name,
	}
	resp, err := cli.post(ctx, "/mappingview", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == OBJECT_NAME_ALREADY_EXIST {
		log.AddContext(ctx).Infof("Mapping %s already exists", name)
		return cli.GetMappingByName(ctx, name)
	}
	if code != 0 {
		msg := fmt.Sprintf("Create mapping %s error: %d", name, code)
		return nil, errors.New(msg)
	}

	mapping := resp.Data.(map[string]interface{})
	return mapping, nil
}

func (cli *Client) GetMappingByName(ctx context.Context, name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/mappingview?filter=NAME::%s", name)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get mapping %s error: %d", name, code)
		return nil, errors.New(msg)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Mapping %s does not exist", name)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) <= 0 {
		log.AddContext(ctx).Infof("Mapping %s does not exist", name)
		return nil, nil
	}

	mapping := respData[0].(map[string]interface{})
	return mapping, nil
}

func (cli *Client) DeleteMapping(ctx context.Context, id string) error {
	url := fmt.Sprintf("/mappingview/%s", id)
	resp, err := cli.delete(ctx, url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == MAPPING_NOT_EXIST {
		log.AddContext(ctx).Infof("Mapping %s does not exist while deleting", id)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Delete mapping %s error: %d", id, code)
		return errors.New(msg)
	}

	return nil
}

func (cli *Client) AddHostToGroup(ctx context.Context, hostID, hostGroupID string) error {
	data := map[string]interface{}{
		"ID":               hostGroupID,
		"ASSOCIATEOBJTYPE": 21,
		"ASSOCIATEOBJID":   hostID,
	}
	resp, err := cli.post(ctx, "/hostgroup/associate", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == HOST_ALREADY_IN_HOSTGROUP {
		log.AddContext(ctx).Infof("Host %s is already in hostgroup %s", hostID, hostGroupID)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Add host %s to hostgroup %s error: %d", hostID, hostGroupID, code)
		return errors.New(msg)
	}

	return nil
}

func (cli *Client) RemoveHostFromGroup(ctx context.Context, hostID, hostGroupID string) error {
	data := map[string]interface{}{
		"ID":               hostGroupID,
		"ASSOCIATEOBJTYPE": 21,
		"ASSOCIATEOBJID":   hostID,
	}
	resp, err := cli.delete(ctx, "/host/associate", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == HOST_NOT_IN_HOSTGROUP {
		log.AddContext(ctx).Infof("Host %s is not in hostgroup %s", hostID, hostGroupID)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Remove host %s from hostgroup %s error: %d", hostID, hostGroupID, code)
		return errors.New(msg)
	}

	return nil
}

func (cli *Client) QueryAssociateHostGroup(ctx context.Context,
	objType int, objID string) ([]interface{}, error) {
	url := fmt.Sprintf("/hostgroup/associate?ASSOCIATEOBJTYPE=%d&ASSOCIATEOBJID=%s", objType, objID)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("associate query hostgroup by obj %s of type %d error: %d", objID, objType, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("obj %s of type %d doesn't associate to any hostgroup", objID, objType)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	return respData, nil
}

func (cli *Client) AddIscsiInitiator(ctx context.Context, initiator string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"ID": initiator,
	}

	resp, err := cli.post(ctx, "/iscsi_initiator", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == OBJECT_ID_NOT_UNIQUE {
		log.AddContext(ctx).Infof("Iscsi initiator %s already exists", initiator)
		return cli.GetIscsiInitiatorByID(ctx, initiator)
	}
	if code != 0 {
		msg := fmt.Sprintf("Add iscsi initiator %s error: %d", initiator, code)
		return nil, errors.New(msg)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}

func (cli *Client) UpdateIscsiInitiator(ctx context.Context, initiator string, alua map[string]interface{}) error {
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

	resp, err := cli.put(ctx, url, data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("update iscsi initiator %s by %v error: %d", initiator, data, code)
	}

	return nil
}

func (cli *Client) AddIscsiInitiatorToHost(ctx context.Context, initiator, hostID string) error {
	url := fmt.Sprintf("/iscsi_initiator/%s", initiator)
	data := map[string]interface{}{
		"PARENTTYPE": 21,
		"PARENTID":   hostID,
	}
	resp, err := cli.put(ctx, url, data)
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

func (cli *Client) AddGroupToMapping(ctx context.Context, groupType int, groupID, mappingID string) error {
	data := map[string]interface{}{
		"ID":               mappingID,
		"ASSOCIATEOBJTYPE": groupType,
		"ASSOCIATEOBJID":   groupID,
	}
	resp, err := cli.put(ctx, "/mappingview/create_associate", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == HOSTGROUP_ALREADY_IN_MAPPING || code == LUNGROUP_ALREADY_IN_MAPPING {
		log.AddContext(ctx).Infof("Group %s of type %d is already in mapping %s",
			groupID, groupType, mappingID)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Add group %s of type %d to mapping %s error: %d", groupID, groupType, mappingID, code)
		return errors.New(msg)
	}

	return nil
}

func (cli *Client) RemoveGroupFromMapping(ctx context.Context,
	groupType int,
	groupID, mappingID string) error {
	data := map[string]interface{}{
		"ID":               mappingID,
		"ASSOCIATEOBJTYPE": groupType,
		"ASSOCIATEOBJID":   groupID,
	}
	resp, err := cli.put(ctx, "/mappingview/remove_associate", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == HOSTGROUP_NOT_IN_MAPPING ||
		code == LUNGROUP_NOT_IN_MAPPING {
		log.AddContext(ctx).Infof("Group %s of type %d is not in mapping %s",
			groupID, groupType, mappingID)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Remove group %s of type %d from mapping %s error: %d", groupID, groupType, mappingID, code)
		return errors.New(msg)
	}

	return nil
}

func (cli *Client) GetLunCountOfHost(ctx context.Context, hostID string) (int64, error) {
	url := fmt.Sprintf("/lun/count?ASSOCIATEOBJTYPE=21&ASSOCIATEOBJID=%s", hostID)
	resp, err := cli.get(ctx, url, nil)
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

func (cli *Client) GetLunCountOfMapping(ctx context.Context, mappingID string) (int64, error) {
	url := fmt.Sprintf("/lun/count?ASSOCIATEOBJTYPE=245&ASSOCIATEOBJID=%s", mappingID)
	resp, err := cli.get(ctx, url, nil)
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

func (cli *Client) GetFileSystemByName(ctx context.Context, name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/filesystem?filter=NAME::%s&range=[0-100]", name)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get filesystem %s error: %d", name, code)
		return nil, errors.New(msg)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Filesystem %s does not exist", name)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) == 0 {
		return nil, nil
	}

	fs := respData[0].(map[string]interface{})
	return fs, nil
}

func (cli *Client) GetFileSystemByID(ctx context.Context, id string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/filesystem/%s", id)
	resp, err := cli.get(ctx, url, nil)
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

func (cli *Client) CreateNfsShare(ctx context.Context,
	params map[string]interface{}) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"SHAREPATH":   params["sharepath"].(string),
		"FSID":        params["fsid"].(string),
		"DESCRIPTION": params["description"].(string),
	}

	vStoreID, _ := params["vStoreID"].(string)
	if vStoreID != "" {
		data["vstoreId"] = vStoreID
	}

	resp, err := cli.post(ctx, "/NFSHARE", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == SHARE_ALREADY_EXIST || code == SHARE_PATH_ALREADY_EXIST {
		sharePath := params["sharepath"].(string)
		log.AddContext(ctx).Infof("Nfs share %s already exists while creating", sharePath)

		share, err := cli.GetNfsShareByPath(ctx, sharePath, vStoreID)
		return share, err
	}

	if code == SYSTEM_BUSY || code == MSG_TIME_OUT {
		for i := 0; i < 10; i++ {
			time.Sleep(time.Second * GET_INFO_WAIT_INTERNAL)
			log.AddContext(ctx).Infof("Create nfs share timeout, try to get info. The %d time", i+1)
			share, err := cli.GetNfsShareByPath(ctx, params["sharepath"].(string), vStoreID)
			if err != nil || share == nil {
				log.AddContext(ctx).Warningf("get nfs share error, share: %v, error: %v", share, err)
				continue
			}
			return share, nil
		}
	}

	if code != 0 {
		return nil, fmt.Errorf("create nfs share %v error: %d", data, code)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}

func (cli *Client) DeleteNfsShare(ctx context.Context, id, vStoreID string) error {
	url := fmt.Sprintf("/NFSHARE/%s", id)
	var data = make(map[string]interface{})
	if vStoreID != "" {
		data["vstoreId"] = vStoreID
	}

	resp, err := cli.delete(ctx, url, data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == SHARE_NOT_EXIST {
		log.AddContext(ctx).Infof("Nfs share %s does not exist while deleting", id)
		return nil
	}
	if code != 0 {
		msg := fmt.Sprintf("Delete nfs share %s error: %d", id, code)
		return errors.New(msg)
	}

	return nil
}

func (cli *Client) GetNfsShareByPath(ctx context.Context, path, vStoreID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/NFSHARE?filter=SHAREPATH::%s&range=[0-100]", path)
	var data = make(map[string]interface{})
	if vStoreID != "" {
		data["vstoreId"] = vStoreID
	}

	resp, err := cli.get(ctx, url, data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == SHARE_PATH_INVALID {
		log.AddContext(ctx).Infof("Nfs share of path %s does not exist", path)
		return nil, nil
	}
	if code != 0 {
		return nil, fmt.Errorf("get nfs share of path %s error: %d", path, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Nfs share of path %s does not exist", path)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) == 0 {
		log.AddContext(ctx).Infof("Nfs share of path %s does not exist", path)
		return nil, nil
	}

	share := respData[0].(map[string]interface{})
	return share, nil
}

func (cli *Client) GetNfsShareAccess(ctx context.Context,
	parentID, name, vStoreID string) (map[string]interface{}, error) {
	count, err := cli.GetNfsShareAccessCount(ctx, parentID, vStoreID)
	if err != nil {
		return nil, err
	}

	var i int64
	for i = 0; i < count; i += 100 { // Query per page 100
		clients, err := cli.GetNfsShareAccessRange(ctx, parentID, vStoreID, i, i+100)
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

func (cli *Client) GetNfsShareAccessCount(ctx context.Context, parentID, vStoreID string) (int64, error) {
	url := fmt.Sprintf("/NFS_SHARE_AUTH_CLIENT/count?filter=PARENTID::%s", parentID)
	var data = make(map[string]interface{})
	if vStoreID != "" {
		data["vstoreId"] = vStoreID
	}
	resp, err := cli.get(ctx, url, data)
	if err != nil {
		return 0, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return 0, fmt.Errorf("get nfs share access count of %s error: %d", parentID, code)
	}

	respData := resp.Data.(map[string]interface{})
	countStr := respData["COUNT"].(string)
	count, _ := strconv.ParseInt(countStr, 10, 64)

	return count, nil
}

func (cli *Client) GetNfsShareAccessRange(ctx context.Context,
	parentID, vStoreID string,
	startRange, endRange int64) ([]interface{}, error) {
	url := fmt.Sprintf("/NFS_SHARE_AUTH_CLIENT?filter=PARENTID::%s&range=[%d-%d]", parentID, startRange, endRange)
	var data = make(map[string]interface{})
	if vStoreID != "" {
		data["vstoreId"] = vStoreID
	}
	resp, err := cli.get(ctx, url, data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("get nfs share access of %s error: %d", parentID, code)
	}

	if resp.Data == nil {
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	return respData, nil
}

func (cli *Client) AllowNfsShareAccess(ctx context.Context, params map[string]interface{}) error {
	resp, err := cli.post(ctx, "/NFS_SHARE_AUTH_CLIENT", params)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("allow nfs share %v access error: %d", params, code)
	}

	return nil
}

func (cli *Client) DeleteNfsShareAccess(ctx context.Context, accessID, vStoreID string) error {
	url := fmt.Sprintf("/NFS_SHARE_AUTH_CLIENT/%s", accessID)
	var data = make(map[string]interface{})
	if vStoreID != "" {
		data["vstoreId"] = vStoreID
	}
	resp, err := cli.delete(ctx, url, data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Delete nfs share %s access error: %d", accessID, code)
	}

	return nil
}

func (cli *Client) GetFCInitiator(ctx context.Context, wwn string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/fc_initiator?filter=ID::%s", wwn)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get fc initiator %s error: %d", wwn, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("FC initiator %s does not exist", wwn)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	initiator := respData[0].(map[string]interface{})
	return initiator, nil
}

func (cli *Client) GetFCInitiatorByID(ctx context.Context, wwn string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/fc_initiator/%s", wwn)
	resp, err := cli.get(ctx, url, nil)
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

func (cli *Client) UpdateFCInitiator(ctx context.Context, wwn string, alua map[string]interface{}) error {
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

	resp, err := cli.put(ctx, url, data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("update fc initiator %s by %v error: %d", wwn, data, code)
	}

	return nil
}

func (cli *Client) QueryFCInitiatorByHost(ctx context.Context, hostID string) ([]interface{}, error) {
	url := fmt.Sprintf("/fc_initiator?PARENTID=%s", hostID)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Query fc initiator of host %s error: %d", hostID, code)
		return nil, errors.New(msg)
	}
	if resp.Data == nil {
		log.AddContext(ctx).Infof("No fc initiator associated to host %s", hostID)
		return nil, nil
	}

	initiators := resp.Data.([]interface{})
	return initiators, nil
}

func (cli *Client) AddFCInitiatorToHost(ctx context.Context, initiator, hostID string) error {
	url := fmt.Sprintf("/fc_initiator/%s", initiator)
	data := map[string]interface{}{
		"PARENTTYPE": 21,
		"PARENTID":   hostID,
	}
	resp, err := cli.put(ctx, url, data)
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

func (cli *Client) GetIscsiInitiator(ctx context.Context, initiator string) (map[string]interface{}, error) {
	id := strings.Replace(initiator, ":", "\\:", -1)
	url := fmt.Sprintf("/iscsi_initiator?filter=ID::%s", id)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		msg := fmt.Sprintf("Get ISCSI initiator %s error: %d", initiator, code)
		return nil, errors.New(msg)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("ISCSI initiator %s does not exist", initiator)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	ini := respData[0].(map[string]interface{})
	return ini, nil
}

func (cli *Client) GetIscsiInitiatorByID(ctx context.Context, initiator string) (map[string]interface{}, error) {
	id := strings.Replace(initiator, ":", "\\:", -1)
	url := fmt.Sprintf("/iscsi_initiator/%s", id)
	resp, err := cli.get(ctx, url, nil)
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

func (cli *Client) GetLicenseFeature(ctx context.Context) (map[string]int, error) {
	resp, err := cli.get(ctx, "/license/feature", nil)
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

// GetApplicationTypeByName function to get the Application type ID to set the I/O size
// while creating Volume
func (cli *Client) GetApplicationTypeByName(ctx context.Context, appType string) (string, error) {
	result := ""
	appType = URL.QueryEscape(appType)
	url := fmt.Sprintf("/workload_type?filter=NAME::%s", appType)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return result, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return result, fmt.Errorf("Get application types returned error: %d", code)
	}

	if resp.Data == nil {
		return result, nil
	}
	respData, ok := resp.Data.([]interface{})
	if !ok {
		return result, errors.New("application types response is not valid")
	}
	// This should be just one elem. But the return is an array with single value
	for _, i := range respData {
		applicationTypes, ok := i.(map[string]interface{})
		if !ok {
			return result, errors.New("Data in response is not valid")
		}
		// From the map we need the application type ID
		// This will be used for param to create LUN
		val, ok := applicationTypes["ID"].(string)
		if !ok {
			return result, fmt.Errorf("application type is not valid")
		}
		result = val
	}
	return result, nil
}

func (cli *Client) GetSystem(ctx context.Context) (map[string]interface{}, error) {
	resp, err := cli.get(ctx, "/system/", nil)
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

func (cli *Client) UpdateLun(ctx context.Context, lunID string, params map[string]interface{}) error {
	url := fmt.Sprintf("/lun/%s", lunID)
	resp, err := cli.put(ctx, url, params)
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

func (cli *Client) UpdateFileSystem(ctx context.Context, fsID string, params map[string]interface{}) error {
	url := fmt.Sprintf("/filesystem/%s", fsID)
	resp, err := cli.put(ctx, url, params)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Update filesystem %s by params %v error: %d", fsID, params, code)
	}

	return nil
}

func (cli *Client) CreateQos(ctx context.Context,
	name, objID, objType, vStoreID string,
	params map[string]int) (map[string]interface{}, error) {
	utcTime, err := cli.getSystemUTCTime(ctx)
	if err != nil {
		return nil, err
	}

	days := time.Unix(utcTime, 0).Format("2006-01-02")
	utcZeroTime, err := time.ParseInLocation("2006-01-02", days, time.UTC)
	if err != nil {
		return nil, err
	}

	data := map[string]interface{}{
		"NAME":              name,
		"SCHEDULEPOLICY":    1,
		"SCHEDULESTARTTIME": utcZeroTime.Unix(),
		"STARTTIME":         "00:00",
		"DURATION":          86400,
	}

	if objType == "fs" {
		data["FSLIST"] = []string{objID}
	} else {
		data["LUNLIST"] = []string{objID}
	}

	if vStoreID != "" {
		data["vstoreId"] = vStoreID
	}

	for k, v := range params {
		data[k] = v
	}

	resp, err := cli.post(ctx, "/ioclass", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == SMARTQOS_ALREADY_EXIST {
		log.AddContext(ctx).Warningf("The QoS %s is already exist.", name)
		return cli.GetQosByName(ctx, name, vStoreID)
	} else if code != 0 {
		return nil, fmt.Errorf("Create qos %v error: %d", data, code)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}

func (cli *Client) ActivateQos(ctx context.Context, qosID, vStoreID string) error {
	data := map[string]interface{}{
		"ID":           qosID,
		"ENABLESTATUS": "true",
	}

	if vStoreID != "" {
		data["vstoreId"] = vStoreID
	}

	resp, err := cli.put(ctx, "/ioclass/active", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Activate qos %s error: %d", qosID, code)
	}

	return nil
}

func (cli *Client) DeactivateQos(ctx context.Context, qosID, vStoreID string) error {
	data := map[string]interface{}{
		"ID":           qosID,
		"ENABLESTATUS": "false",
	}

	if vStoreID != "" {
		data["vstoreId"] = vStoreID
	}

	resp, err := cli.put(ctx, "/ioclass/active", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Deactivate qos %s error: %d", qosID, code)
	}

	return nil
}

func (cli *Client) DeleteQos(ctx context.Context, qosID, vStoreID string) error {
	url := fmt.Sprintf("/ioclass/%s", qosID)
	var data = make(map[string]interface{})
	if vStoreID != "" {
		data["vstoreId"] = vStoreID
	}

	resp, err := cli.delete(ctx, url, data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Delete qos %s error: %d", qosID, code)
	}

	return nil
}

func (cli *Client) GetQosByName(ctx context.Context, name, vStoreID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/ioclass?filter=NAME::%s", name)
	var data = make(map[string]interface{})
	if vStoreID != "" {
		data["vstoreId"] = vStoreID
	}

	resp, err := cli.get(ctx, url, data)
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

func (cli *Client) GetQosByID(ctx context.Context, qosID, vStoreID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/ioclass/%s", qosID)
	var data = make(map[string]interface{})
	if vStoreID != "" {
		data["vstoreId"] = vStoreID
	}
	resp, err := cli.get(ctx, url, data)
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

func (cli *Client) UpdateQos(ctx context.Context, qosID, vStoreID string, params map[string]interface{}) error {
	url := fmt.Sprintf("/ioclass/%s", qosID)
	if vStoreID != "" {
		params["vstoreId"] = vStoreID
	}
	resp, err := cli.put(ctx, url, params)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Update qos %s to %v error: %d", qosID, params, code)
	}

	return nil
}

func (cli *Client) GetIscsiTgtPort(ctx context.Context) ([]interface{}, error) {
	resp, err := cli.get(ctx, "/iscsi_tgt_port", nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get ISCSI tgt port error: %d", code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("ISCSI tgt port does not exist")
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	return respData, nil
}

func (cli *Client) GetFCHostLink(ctx context.Context, hostID string) ([]interface{}, error) {
	url := fmt.Sprintf("/host_link?INITIATOR_TYPE=223&PARENTID=%s", hostID)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get FC host link of host %s error: %d", hostID, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("There is no FC host link of host %s", hostID)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	return respData, nil
}

func (cli *Client) GetISCSIHostLink(ctx context.Context, hostID string) ([]interface{}, error) {
	url := fmt.Sprintf("/host_link?INITIATOR_TYPE=222&PARENTID=%s", hostID)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get ISCSI host link of host %s error: %d", hostID, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("There is no ISCSI host link of host %s", hostID)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	return respData, nil
}

func (cli *Client) CreateLunSnapshot(ctx context.Context, name, lunID string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME":        name,
		"DESCRIPTION": "Created from Kubernetes",
		"PARENTID":    lunID,
	}

	resp, err := cli.post(ctx, "/snapshot", data)
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

func (cli *Client) GetLunSnapshotByName(ctx context.Context, name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/snapshot?filter=NAME::%s&range=[0-100]", name)

	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get snapshot by name %s error: %d", name, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Snapshot %s does not exist", name)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) <= 0 {
		return nil, nil
	}

	snapshot := respData[0].(map[string]interface{})
	return snapshot, nil
}

func (cli *Client) DeleteLunSnapshot(ctx context.Context, snapshotID string) error {
	url := fmt.Sprintf("/snapshot/%s", snapshotID)
	resp, err := cli.delete(ctx, url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == LUN_SNAPSHOT_NOT_EXIST {
		log.AddContext(ctx).Infof("Lun snapshot %s does not exist while deleting", snapshotID)
		return nil
	}
	if code != 0 {
		return fmt.Errorf("Delete snapshot %s error: %d", snapshotID, code)
	}

	return nil
}

func (cli *Client) ActivateLunSnapshot(ctx context.Context, snapshotID string) error {
	data := map[string]interface{}{
		"SNAPSHOTLIST": []string{snapshotID},
	}

	resp, err := cli.post(ctx, "/snapshot/activate", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Activate snapshot %s error: %d", snapshotID, code)
	}

	return nil
}

func (cli *Client) DeactivateLunSnapshot(ctx context.Context, snapshotID string) error {
	data := map[string]interface{}{
		"ID": snapshotID,
	}

	resp, err := cli.put(ctx, "/snapshot/stop", data)
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

func (cli *Client) CreateLunCopy(ctx context.Context,
	name, srcLunID, dstLunID string,
	copySpeed int) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME":      name,
		"COPYSPEED": copySpeed,
		"SOURCELUN": fmt.Sprintf("INVALID;%s;INVALID;INVALID;INVALID", srcLunID),
		"TARGETLUN": fmt.Sprintf("INVALID;%s;INVALID;INVALID;INVALID", dstLunID),
	}

	resp, err := cli.post(ctx, "/LUNCOPY", data)
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

func (cli *Client) GetLunCopyByID(ctx context.Context, lunCopyID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/LUNCOPY/%s", lunCopyID)

	resp, err := cli.get(ctx, url, nil)
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

func (cli *Client) GetLunCopyByName(ctx context.Context, name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/LUNCOPY?filter=NAME::%s", name)

	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get luncopy by name %s error: %d", name, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Luncopy %s does not exist", name)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) <= 0 {
		log.AddContext(ctx).Infof("Luncopy %s does not exist", name)
		return nil, nil
	}

	luncopy := respData[0].(map[string]interface{})
	return luncopy, nil
}

func (cli *Client) StartLunCopy(ctx context.Context, lunCopyID string) error {
	data := map[string]interface{}{
		"ID": lunCopyID,
	}

	resp, err := cli.put(ctx, "/LUNCOPY/start", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Start luncopy %s error: %d", lunCopyID, code)
	}

	return nil
}

func (cli *Client) StopLunCopy(ctx context.Context, lunCopyID string) error {
	data := map[string]interface{}{
		"ID": lunCopyID,
	}

	resp, err := cli.put(ctx, "/LUNCOPY/stop", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Stop luncopy %s error: %d", lunCopyID, code)
	}

	return nil
}

func (cli *Client) DeleteLunCopy(ctx context.Context, lunCopyID string) error {
	url := fmt.Sprintf("/LUNCOPY/%s", lunCopyID)

	resp, err := cli.delete(ctx, url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == LUNCOPY_NOT_EXIST {
		log.AddContext(ctx).Infof("Luncopy %s does not exist while deleting", lunCopyID)
		return nil
	}
	if code != 0 {
		return fmt.Errorf("Delete luncopy %s error: %d", lunCopyID, code)
	}

	return nil
}

func (cli *Client) CreateFSSnapshot(ctx context.Context, name, parentID string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME":        name,
		"DESCRIPTION": "Created from Kubernetes",
		"PARENTID":    parentID,
		"PARENTTYPE":  "40",
	}

	resp, err := cli.post(ctx, "/FSSNAPSHOT", data)
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

func (cli *Client) DeleteFSSnapshot(ctx context.Context, snapshotID string) error {
	url := fmt.Sprintf("/FSSNAPSHOT/%s", snapshotID)
	resp, err := cli.delete(ctx, url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == FS_SNAPSHOT_NOT_EXIST {
		log.AddContext(ctx).Infof("FS Snapshot %s does not exist while deleting", snapshotID)
		return nil
	}
	if code != 0 {
		return fmt.Errorf("Delete FS snapshot %s error: %d", snapshotID, code)
	}

	return nil
}

func (cli *Client) GetFSSnapshotByName(ctx context.Context,
	parentID, snapshotName string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/FSSNAPSHOT?PARENTID=%s&filter=NAME::%s", parentID, snapshotName)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		if code == SNAPSHOT_PARENT_NOT_EXIST_V3 || code == SNAPSHOT_PARENT_NOT_EXIST_V6 {
			log.AddContext(ctx).Infof("The parent filesystem %s of snapshot %s does not exist",
				parentID, snapshotName)
			return nil, nil
		}

		return nil, fmt.Errorf("failed to get filesystem snapshot %s, error is %d", snapshotName, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Filesystem snapshot %s does not exist", snapshotName)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) <= 0 {
		return nil, nil
	}

	snapshot := respData[0].(map[string]interface{})
	return snapshot, nil
}

func (cli *Client) GetFSSnapshotCountByParentId(ctx context.Context, ParentId string) (int, error) {
	url := fmt.Sprintf("/FSSNAPSHOT/count?PARENTID=%s", ParentId)
	resp, err := cli.get(ctx, url, nil)
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

func (cli *Client) CloneFileSystem(ctx context.Context,
	name string,
	allocType int,
	parentID, parentSnapshotID string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"NAME":               name,
		"ALLOCTYPE":          allocType,
		"DESCRIPTION":        "Created from Kubernetes",
		"PARENTFILESYSTEMID": parentID,
	}

	if parentSnapshotID != "" {
		data["PARENTSNAPSHOTID"] = parentSnapshotID
	}

	resp, err := cli.post(ctx, "/filesystem", data)
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

func (cli *Client) SplitCloneFS(ctx context.Context,
	fsID string,
	splitSpeed int,
	isDeleteParentSnapshot bool) error {
	data := map[string]interface{}{
		"ID":                     fsID,
		"SPLITENABLE":            true,
		"SPLITSPEED":             splitSpeed,
		"ISDELETEPARENTSNAPSHOT": isDeleteParentSnapshot,
	}

	resp, err := cli.put(ctx, "/filesystem_split_switch", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Split FS %s error: %d", fsID, code)
	}

	return nil
}

func (cli *Client) StopCloneFSSplit(ctx context.Context, fsID string) error {
	data := map[string]interface{}{
		"ID":          fsID,
		"SPLITENABLE": false,
	}

	resp, err := cli.put(ctx, "/filesystem_split_switch", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Stop FS %s splitting error: %d", fsID, code)
	}

	return nil
}

func (cli *Client) ExtendFileSystem(ctx context.Context, fsID string, newCapacity int64) error {
	url := fmt.Sprintf("/filesystem/%s", fsID)
	data := map[string]interface{}{
		"CAPACITY": newCapacity,
	}

	resp, err := cli.put(ctx, url, data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Extend FS capacity to %d error: %d", newCapacity, code)
	}

	return nil
}

func (cli *Client) ExtendLun(ctx context.Context, lunID string, newCapacity int64) error {
	data := map[string]interface{}{
		"CAPACITY": newCapacity,
		"ID":       lunID,
	}

	resp, err := cli.put(ctx, "/lun/expand", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Extend LUN capacity to %d error: %d", newCapacity, code)
	}

	return nil
}

func (cli *Client) GetHyperMetroDomainByName(ctx context.Context, name string) (map[string]interface{}, error) {
	resp, err := cli.get(ctx, "/HyperMetroDomain?range=[0-100]", nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get HyperMetroDomain of name %s error: %d", name, code)
	}
	if resp.Data == nil {
		log.AddContext(ctx).Infof("No HyperMetroDomain %s exist", name)
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

func (cli *Client) GetHyperMetroDomain(ctx context.Context, domainID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/HyperMetroDomain/%s", domainID)
	resp, err := cli.get(ctx, url, nil)
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

func (cli *Client) CreateHyperMetroPair(ctx context.Context,
	data map[string]interface{}) (map[string]interface{}, error) {
	resp, err := cli.post(ctx, "/HyperMetroPair", data)
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

func (cli *Client) SyncHyperMetroPair(ctx context.Context, pairID string) error {
	data := map[string]interface{}{
		"ID": pairID,
	}

	resp, err := cli.put(ctx, "/HyperMetroPair/synchronize_hcpair", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Sync hypermetro %s error: %d", pairID, code)
	}

	return nil
}

func (cli *Client) StopHyperMetroPair(ctx context.Context, pairID string) error {
	data := map[string]interface{}{
		"ID": pairID,
	}

	resp, err := cli.put(ctx, "/HyperMetroPair/disable_hcpair", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Stop hypermetro %s error: %d", pairID, code)
	}

	return nil
}

func (cli *Client) GetHyperMetroPair(ctx context.Context, pairID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/HyperMetroPair?filter=ID::%s", pairID)

	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get hypermetro %s error: %d", pairID, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Hypermetro %s does not exist", pairID)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) <= 0 {
		log.AddContext(ctx).Infof("Hypermetro %s does not exist", pairID)
		return nil, nil
	}

	pair := respData[0].(map[string]interface{})
	return pair, nil
}

func (cli *Client) DeleteHyperMetroPair(ctx context.Context, pairID string, onlineDelete bool) error {
	url := fmt.Sprintf("/HyperMetroPair/%s", pairID)
	if !onlineDelete {
		url += "?isOnlineDeleting=0"
	}

	resp, err := cli.delete(ctx, url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == HYPERMETRO_NOT_EXIST {
		log.AddContext(ctx).Infof("Hypermetro %s to delete does not exist", pairID)
		return nil
	} else if code != 0 {
		return fmt.Errorf("Delete hypermetro %s error: %d", pairID, code)
	}

	return nil
}

func (cli *Client) GetHyperMetroPairByLocalObjID(ctx context.Context,
	objID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/HyperMetroPair?filter=LOCALOBJID::%s", objID)

	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get hypermetro of local obj %s error: %d", objID, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Hypermetro of local obj %s does not exist", objID)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	for _, i := range respData {
		pair := i.(map[string]interface{})
		if pair["LOCALOBJID"] == objID {
			return pair, nil
		}
	}

	log.AddContext(ctx).Infof("Hypermetro of local obj %s does not exist", objID)
	return nil, nil
}

func (cli *Client) CreateClonePair(ctx context.Context,
	srcLunID, dstLunID string,
	cloneSpeed int) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"copyRate":          cloneSpeed,
		"sourceID":          srcLunID,
		"targetID":          dstLunID,
		"isNeedSynchronize": "0",
	}

	resp, err := cli.post(ctx, "/clonepair/relation", data)
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

func (cli *Client) SyncClonePair(ctx context.Context, clonePairID string) error {
	data := map[string]interface{}{
		"ID":         clonePairID,
		"copyAction": 0,
	}

	resp, err := cli.put(ctx, "/clonepair/synchronize", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Sync ClonePair %s error: %d", clonePairID, code)
	}

	return nil
}

func (cli *Client) DeleteClonePair(ctx context.Context, clonePairID string) error {
	data := map[string]interface{}{
		"ID":             clonePairID,
		"isDeleteDstLun": false,
	}

	resp, err := cli.delete(ctx, "/clonepair", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == CLONEPAIR_NOT_EXIST {
		log.AddContext(ctx).Infof("ClonePair %s does not exist while deleting", clonePairID)
		return nil
	}
	if code != 0 {
		return fmt.Errorf("Delete ClonePair %s error: %d", clonePairID, code)
	}

	return nil
}

func (cli *Client) GetClonePairInfo(ctx context.Context, clonePairID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/clonepair?filter=ID::%s", clonePairID)

	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get ClonePair info %s error: %d", clonePairID, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("clonePair %s does not exist", clonePairID)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) <= 0 {
		log.AddContext(ctx).Infof("clonePair %s does not exist", clonePairID)
		return nil, nil
	}

	clonePair := respData[0].(map[string]interface{})
	return clonePair, nil
}

func (cli *Client) GetRemoteDeviceBySN(ctx context.Context, sn string) (map[string]interface{}, error) {
	resp, err := cli.get(ctx, "/remote_device", nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get remote device %s error: %d", sn, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Remote device %s does not exist", sn)
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

func (cli *Client) CreateReplicationPair(ctx context.Context,
	data map[string]interface{}) (map[string]interface{}, error) {
	resp, err := cli.post(ctx, "/REPLICATIONPAIR", data)
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

func (cli *Client) SplitReplicationPair(ctx context.Context, pairID string) error {
	data := map[string]interface{}{
		"ID": pairID,
	}

	resp, err := cli.put(ctx, "/REPLICATIONPAIR/split", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Split replication pair %s error: %d", pairID, code)
	}

	return nil
}

func (cli *Client) SyncReplicationPair(ctx context.Context, pairID string) error {
	data := map[string]interface{}{
		"ID": pairID,
	}

	resp, err := cli.put(ctx, "/REPLICATIONPAIR/sync", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("Sync replication pair %s error: %d", pairID, code)
	}

	return nil
}

func (cli *Client) DeleteReplicationPair(ctx context.Context, pairID string) error {
	url := fmt.Sprintf("/REPLICATIONPAIR/%s", pairID)
	resp, err := cli.delete(ctx, url, nil)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code == REPLICATION_NOT_EXIST {
		log.AddContext(ctx).Infof("Replication pair %s does not exist while deleting", pairID)
		return nil
	}
	if code != 0 {
		return fmt.Errorf("Delete replication pair %s error: %d", pairID, code)
	}

	return nil
}

func (cli *Client) GetReplicationPairByResID(ctx context.Context,
	resID string,
	resType int) ([]map[string]interface{}, error) {
	url := fmt.Sprintf("/REPLICATIONPAIR/associate?ASSOCIATEOBJTYPE=%d&ASSOCIATEOBJID=%s", resType, resID)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("get replication pairs resource %s associated error: %d", resID, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("Replication pairs resource %s associated does not exist", resID)
		return nil, nil
	}

	var pairs []map[string]interface{}

	respData := resp.Data.([]interface{})
	for _, i := range respData {
		pairs = append(pairs, i.(map[string]interface{}))
	}

	return pairs, nil
}

func (cli *Client) GetReplicationPairByID(ctx context.Context, pairID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/REPLICATIONPAIR/%s", pairID)
	resp, err := cli.get(ctx, url, nil)
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

func (cli *Client) GetReplicationvStorePairCount(ctx context.Context) (int64, error) {
	resp, err := cli.get(ctx, "/replication_vstorepair/count", nil)
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

func (cli *Client) GetReplicationvStorePairRange(ctx context.Context,
	startRange, endRange int64) ([]interface{}, error) {
	url := fmt.Sprintf("/replication_vstorepair?range=[%d-%d]", startRange, endRange)
	resp, err := cli.get(ctx, url, nil)
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

func (cli *Client) GetReplicationvStorePairByvStore(ctx context.Context,
	vStoreID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/replication_vstorepair/associate?ASSOCIATEOBJTYPE=16442&ASSOCIATEOBJID=%s", vStoreID)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get replication vstore pair by vstore %s error: %d", vStoreID, code)
	}
	if resp.Data == nil {
		log.AddContext(ctx).Infof("Replication vstore pair of vstore %s does not exist", vStoreID)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) == 0 {
		log.AddContext(ctx).Infof("Replication vstore pair of vstore %s does not exist", vStoreID)
		return nil, nil
	}

	pair := respData[0].(map[string]interface{})
	return pair, nil
}

func (cli *Client) GetvStoreByName(ctx context.Context, name string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/vstore?filter=NAME::%s", name)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get vstore %s error: %d", name, code)
	}
	if resp.Data == nil {
		log.AddContext(ctx).Infof("vstore %s does not exist", name)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) == 0 {
		log.AddContext(ctx).Infof("vstore %s does not exist", name)
		return nil, nil
	}

	vstore := respData[0].(map[string]interface{})
	return vstore, nil
}

func (cli *Client) GetvStorePairByID(ctx context.Context, pairID string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/vstore_pair?filter=ID::%s", pairID)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("Get vstore pair by ID %s error: %d", pairID, code)
	}
	if resp.Data == nil {
		log.AddContext(ctx).Infof("vstore pair %s does not exist", pairID)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) == 0 {
		log.AddContext(ctx).Infof("vstore pair %s does not exist", pairID)
		return nil, nil
	}

	pair := respData[0].(map[string]interface{})
	return pair, nil
}

func (cli *Client) GetFSHyperMetroDomain(ctx context.Context, domainName string) (map[string]interface{}, error) {
	url := "/FsHyperMetroDomain?RUNNINGSTATUS=0"
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("get filesystem hyperMetro domain %s error: %d", domainName, code)
	}
	if resp.Data == nil {
		log.AddContext(ctx).Infof("hyperMetro domain %s does not exist", domainName)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	for _, d := range respData {
		domain := d.(map[string]interface{})
		if domain["NAME"].(string) == domainName {
			return domain, nil
		}
	}

	log.AddContext(ctx).Infof("FileSystem hyperMetro domain %s does not exist or is not normal", domainName)
	return nil, nil
}

func (cli *Client) GetRoCEInitiator(ctx context.Context, initiator string) (map[string]interface{}, error) {
	id := URL.QueryEscape(strings.Replace(initiator, ":", "\\:", -1))
	url := fmt.Sprintf("/NVMe_over_RoCE_initiator?filter=ID::%s", id)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("get RoCE initiator %s error: %d", initiator, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("RoCE initiator %s does not exist", initiator)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) == 0 {
		return nil, nil
	}
	ini := respData[0].(map[string]interface{})
	return ini, nil
}

func (cli *Client) GetRoCEInitiatorByID(ctx context.Context, initiator string) (map[string]interface{}, error) {
	id := URL.QueryEscape(strings.Replace(initiator, ":", "\\:", -1))
	url := fmt.Sprintf("/NVMe_over_RoCE_initiator/%s", id)
	resp, err := cli.get(ctx, url, nil)
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

func (cli *Client) AddRoCEInitiator(ctx context.Context, initiator string) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"ID": initiator,
	}

	resp, err := cli.post(ctx, "/NVMe_over_RoCE_initiator", data)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code == OBJECT_ID_NOT_UNIQUE {
		log.AddContext(ctx).Infof("RoCE initiator %s already exists", initiator)
		return cli.GetRoCEInitiatorByID(ctx, initiator)
	}
	if code != 0 {
		return nil, fmt.Errorf("add RoCE initiator %s error: %d", initiator, code)
	}

	respData := resp.Data.(map[string]interface{})
	return respData, nil
}

func (cli *Client) AddRoCEInitiatorToHost(ctx context.Context, initiator, hostID string) error {
	data := map[string]interface{}{
		"ID":               hostID,
		"ASSOCIATEOBJTYPE": 57870,
		"ASSOCIATEOBJID":   initiator,
	}
	resp, err := cli.put(ctx, "/host/create_associate", data)
	if err != nil {
		return err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return fmt.Errorf("add RoCE initiator %s to host %s error: %d", initiator, hostID, code)
	}

	return nil
}

func (cli *Client) GetRoCEPortalByIP(ctx context.Context, tgtPortal string) (map[string]interface{}, error) {
	url := fmt.Sprintf("/lif?filter=IPV4ADDR::%s", tgtPortal)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("get RoCE by IP %s error: %d", tgtPortal, code)
	}
	if resp.Data == nil {
		log.AddContext(ctx).Infof("RoCE portal %s does not exist", tgtPortal)
		return nil, nil
	}

	respData := resp.Data.([]interface{})
	if len(respData) == 0 {
		log.AddContext(ctx).Infof("RoCE portal %s does not exist", tgtPortal)
		return nil, nil
	}

	portal := respData[0].(map[string]interface{})
	return portal, nil
}

func (cli *Client) GetHostLunId(ctx context.Context, hostID, lunID string) (string, error) {
	hostLunId := "1"
	url := fmt.Sprintf("/lun/associate?TYPE=11&ASSOCIATEOBJTYPE=21&ASSOCIATEOBJID=%s", hostID)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return "", err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return "", fmt.Errorf("get hostLunId of host %s, lun %s error: %d", hostID, lunID, code)
	}

	respData := resp.Data.([]interface{})
	for _, i := range respData {
		hostLunInfo := i.(map[string]interface{})
		if hostLunInfo["ID"].(string) == lunID {
			var associateData map[string]interface{}
			associateDataBytes := []byte(hostLunInfo["ASSOCIATEMETADATA"].(string))
			err := json.Unmarshal(associateDataBytes, &associateData)
			if err != nil {
				return "", nil
			}
			hostLunId = strconv.FormatInt(int64(associateData["HostLUNID"].(float64)), 10)
			break
		}
	}

	return hostLunId, nil
}

func (cli *Client) GetFCTargetWWNs(ctx context.Context, initiatorWWN string) ([]string, error) {
	url := fmt.Sprintf("/host_link?INITIATOR_TYPE=223&INITIATOR_PORT_WWN=%s", initiatorWWN)
	resp, err := cli.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}

	code := int64(resp.Error["code"].(float64))
	if code != 0 {
		return nil, fmt.Errorf("get FC target wwns of initiator %s error: %d", initiatorWWN, code)
	}

	if resp.Data == nil {
		log.AddContext(ctx).Infof("There is no FC target wwn of host initiator wwn %s", initiatorWWN)
		return nil, nil
	}

	var tgtWWNs []string
	respData := resp.Data.([]interface{})
	for _, tgt := range respData {
		tgtPort := tgt.(map[string]interface{})
		tgtWWNs = append(tgtWWNs, tgtPort["TARGET_PORT_WWN"].(string))
	}

	return tgtWWNs, nil
}

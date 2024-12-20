/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2024. All rights reserved.
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

package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"sync"

	xuanwuV1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/constants"
	pkgVolume "github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/volume"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/proto"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/attacher"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/client"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/storage/oceanstorage/oceanstor/volume"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	hyperMetroPairRunningStatusNormal = "1"
	hyperMetroPairRunningStatusPause  = "41"
	reflectResultLength               = 2
	csiInvoker                        = "csi"
)

// OceanstorSanPlugin implements storage StoragePlugin interface
type OceanstorSanPlugin struct {
	OceanstorPlugin
	protocol string
	portals  []string
	alua     map[string]interface{}

	replicaRemotePlugin *OceanstorSanPlugin
	metroRemotePlugin   *OceanstorSanPlugin
	storageOnline       bool
	clientCount         int
	clientMutex         sync.Mutex
}

type handlerRequest struct {
	localCli   client.OceanstorClientInterface
	metroCli   client.OceanstorClientInterface
	lun        map[string]interface{}
	parameters map[string]interface{}
	method     string
}

func init() {
	RegPlugin("oceanstor-san", &OceanstorSanPlugin{})
}

// NewPlugin used to create new plugin
func (p *OceanstorSanPlugin) NewPlugin() StoragePlugin {
	return &OceanstorSanPlugin{}
}

// Init used to init the plugin
func (p *OceanstorSanPlugin) Init(ctx context.Context, config map[string]interface{},
	parameters map[string]interface{}, keepLogin bool) error {
	protocol, exist := parameters["protocol"].(string)
	if !exist || (protocol != "iscsi" && protocol != "fc" && protocol != "roce" && protocol != "fc-nvme") {
		return errors.New("protocol must be provided as 'iscsi', 'fc', " +
			"'roce' or 'fc-nvme' for oceanstor-san backend")
	}

	p.alua, _ = parameters["ALUA"].(map[string]interface{})

	if protocol == "iscsi" || protocol == "roce" {
		portals, exist := parameters["portals"].([]interface{})
		if !exist {
			return errors.New("portals are required to configure for iSCSI or RoCE backend")
		}

		IPs, err := proto.VerifyIscsiPortals(ctx, portals)
		if err != nil {
			return err
		}

		p.portals = IPs
	}

	err := p.init(ctx, config, keepLogin)
	if err != nil {
		return err
	}

	if (protocol == "roce" || protocol == "fc-nvme") && !p.product.IsDoradoV6OrV7() {
		p.Logout(ctx)

		msg := fmt.Sprintf("The storage backend %s does not support NVME protocol", p.product)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	p.protocol = protocol
	p.storageOnline = true

	return nil
}

func (p *OceanstorSanPlugin) getSanObj() *volume.SAN {
	var metroRemoteCli client.OceanstorClientInterface
	var replicaRemoteCli client.OceanstorClientInterface

	if p.metroRemotePlugin != nil {
		metroRemoteCli = p.metroRemotePlugin.cli
	}
	if p.replicaRemotePlugin != nil {
		replicaRemoteCli = p.replicaRemotePlugin.cli
	}

	return volume.NewSAN(p.cli, metroRemoteCli, replicaRemoteCli, p.product)
}

// CreateVolume used to create volume
func (p *OceanstorSanPlugin) CreateVolume(ctx context.Context,
	name string, parameters map[string]interface{}) (utils.Volume, error) {

	params := getParams(ctx, name, parameters)
	san := p.getSanObj()

	volObj, err := san.Create(ctx, params)
	if err != nil {
		return nil, err
	}
	return volObj, nil
}

// QueryVolume used to query volume
func (p *OceanstorSanPlugin) QueryVolume(ctx context.Context, name string, params map[string]interface{}) (
	utils.Volume, error) {
	san := p.getSanObj()
	return san.Query(ctx, name)
}

// DeleteVolume used to delete volume
func (p *OceanstorSanPlugin) DeleteVolume(ctx context.Context, name string) error {
	san := p.getSanObj()
	return san.Delete(ctx, name)
}

// ExpandVolume used to expand volume
func (p *OceanstorSanPlugin) ExpandVolume(ctx context.Context, name string, size int64) (bool, error) {
	san := p.getSanObj()
	return san.Expand(ctx, name, size)
}

func (p *OceanstorSanPlugin) isHyperMetro(ctx context.Context, lun map[string]interface{}) bool {
	rssStr, ok := lun["HASRSSOBJECT"].(string)
	if !ok {
		log.AddContext(ctx).Errorf("get lun HASRSSOBJECT failed, lun[\"HASRSSOBJECT\"]:%v", lun["HASRSSOBJECT"])
		return false
	}

	var rss map[string]string
	if err := json.Unmarshal([]byte(rssStr), &rss); err != nil {
		log.AddContext(ctx).Errorf("unmarshal lun HASRSSOBJECT failed, lun[\"HASRSSOBJECT\"]:%s", rssStr)
		return false
	}

	if hyperMetro, ok := rss["HyperMetro"]; ok && hyperMetro == "TRUE" {
		return true
	}
	return false
}

func (p *OceanstorSanPlugin) metroHandler(ctx context.Context, req handlerRequest) ([]reflect.Value, error) {
	localLunID, ok := req.lun["ID"].(string)
	if !ok {
		log.AddContext(ctx).Warningf("req.lun[\"ID\"] is not string")
	}
	pair, err := req.localCli.GetHyperMetroPairByLocalObjID(ctx, localLunID)
	if err != nil {
		return nil, err
	}
	if pair == nil {
		return nil, fmt.Errorf("hypermetro pair of LUN %s doesn't exist", localLunID)
	}

	if req.method == "ControllerDetach" || req.method == "NodeUnstage" {
		if pair["RUNNINGSTATUS"] != hyperMetroPairRunningStatusNormal &&
			pair["RUNNINGSTATUS"] != hyperMetroPairRunningStatusPause {
			log.AddContext(ctx).Warningf("hypermetro pair status of LUN %s is not normal or pause",
				localLunID)
		}
	} else {
		if pair["RUNNINGSTATUS"] != hyperMetroPairRunningStatusNormal {
			log.AddContext(ctx).Warningf("hypermetro pair status of LUN %s is not normal", localLunID)
		}
	}

	localAttacher := attacher.NewAttacher(attacher.VolumeAttacherConfig{
		Product:  p.product,
		Cli:      req.localCli,
		Protocol: p.protocol,
		Invoker:  "csi",
		Portals:  p.portals,
		Alua:     p.alua,
	})
	remoteAttacher := attacher.NewAttacher(attacher.VolumeAttacherConfig{
		Product:  p.metroRemotePlugin.product,
		Cli:      req.metroCli,
		Protocol: p.metroRemotePlugin.protocol,
		Invoker:  "csi",
		Portals:  p.metroRemotePlugin.portals,
		Alua:     p.metroRemotePlugin.alua,
	})

	metroAttacher := attacher.NewMetroAttacher(localAttacher, remoteAttacher, p.protocol)
	lunName, ok := req.lun["NAME"].(string)
	if !ok {
		log.AddContext(ctx).Warningf("req.lun[\"NAME\"] is not string")
	}
	out := utils.ReflectCall(metroAttacher, req.method, ctx, lunName, req.parameters)

	return out, nil
}

func (p *OceanstorSanPlugin) commonHandler(ctx context.Context,
	plugin *OceanstorSanPlugin, lun, parameters map[string]any, method string) ([]reflect.Value, error) {
	commonAttacher := attacher.NewAttacher(attacher.VolumeAttacherConfig{
		Product:  plugin.product,
		Cli:      plugin.cli,
		Protocol: plugin.protocol,
		Invoker:  "csi",
		Portals:  plugin.portals,
		Alua:     plugin.alua,
	})

	lunName, ok := lun["NAME"].(string)
	if !ok {
		return nil, errors.New("there is no NAME in lun info")
	}
	out := utils.ReflectCall(commonAttacher, method, ctx, lunName, parameters)
	return out, nil
}

func (p *OceanstorSanPlugin) handler(ctx context.Context, req handlerRequest) ([]reflect.Value, error) {
	var out []reflect.Value
	var err error

	if !p.isHyperMetro(ctx, req.lun) {
		return p.commonHandler(ctx, p, req.lun, req.parameters, req.method)
	}

	if p.storageOnline && p.metroRemotePlugin != nil && p.metroRemotePlugin.storageOnline {
		out, err = p.metroHandler(ctx, req)
	} else if p.storageOnline {
		log.AddContext(ctx).Warningf("the lun %s is hyperMetro, but just the local storage is online",
			req.lun["NAME"].(string))
		out, err = p.commonHandler(ctx, p, req.lun, req.parameters, req.method)
	} else if p.metroRemotePlugin != nil && p.metroRemotePlugin.storageOnline {
		log.AddContext(ctx).Warningf("the lun %s is hyperMetro, but just the remote storage is online",
			req.lun["NAME"].(string))
		out, err = p.commonHandler(ctx, p.metroRemotePlugin, req.lun, req.parameters, req.method)
	}

	return out, err
}

// AttachVolume attach volume to node,return storage mapping info.
func (p *OceanstorSanPlugin) AttachVolume(ctx context.Context, name string,
	parameters map[string]interface{}) (map[string]interface{}, error) {
	var localCli, metroCli client.OceanstorClientInterface
	if p.storageOnline {
		localCli = p.cli
	}

	if p.metroRemotePlugin != nil && p.metroRemotePlugin.storageOnline {
		metroCli = p.metroRemotePlugin.cli
	}

	lunName := p.cli.MakeLunName(name)
	lun, err := p.getLunInfo(ctx, localCli, metroCli, lunName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get lun %s error: %v", lunName, err)
		return nil, err
	}
	if lun == nil {
		return nil, utils.Errorf(ctx, "Get empty lun info, lunName: %v", lunName)
	}

	var out []reflect.Value
	out, err = p.handler(ctx, handlerRequest{localCli: localCli, metroCli: metroCli,
		lun: lun, parameters: parameters, method: "ControllerAttach"})
	if err != nil {
		return nil, utils.Errorf(ctx, "Storage connect for volume %s error: %v", lunName, err)
	}

	if len(out) != reflectResultLength {
		return nil, utils.Errorf(ctx, "attach volume %s error", lunName)
	}

	result := out[1].Interface()
	if result != nil {
		return nil, result.(error)
	}

	connectInfo, ok := out[0].Interface().(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("controller attach volume %s error", lunName)
	}
	return connectInfo, nil
}

// DetachVolume used to detach volume from node
func (p *OceanstorSanPlugin) DetachVolume(ctx context.Context, name string, parameters map[string]interface{}) error {
	var localCli, metroCli client.OceanstorClientInterface
	if p.storageOnline {
		localCli = p.cli
	}

	if p.metroRemotePlugin != nil && p.metroRemotePlugin.storageOnline {
		metroCli = p.metroRemotePlugin.cli
	}

	lunName := p.cli.MakeLunName(name)
	lun, err := p.getLunInfo(ctx, localCli, metroCli, lunName)
	if err != nil {
		log.AddContext(ctx).Errorf("Get lun %s error: %v", lunName, err)
		return err
	}
	if lun == nil {
		log.AddContext(ctx).Warningf("LUN %s to detach doesn't exist", lunName)
		return nil
	}

	var out []reflect.Value
	out, err = p.handler(ctx, handlerRequest{localCli: localCli, metroCli: metroCli,
		lun: lun, parameters: parameters, method: "ControllerDetach"})
	if err != nil {
		return err
	}
	if len(out) != reflectResultLength {
		return fmt.Errorf("detach volume %s error", lunName)
	}

	result := out[1].Interface()
	if result != nil {
		return result.(error)
	}

	return nil
}

func (p *OceanstorSanPlugin) mutexReleaseClient(ctx context.Context,
	plugin *OceanstorSanPlugin,
	cli client.OceanstorClientInterface) {
	plugin.clientMutex.Lock()
	defer plugin.clientMutex.Unlock()
	plugin.clientCount--
	if plugin.clientCount == 0 {
		cli.Logout(ctx)
		plugin.storageOnline = false
	}
}

func (p *OceanstorSanPlugin) releaseClient(ctx context.Context, cli, metroCli client.OceanstorClientInterface) {
	if p.storageOnline {
		p.mutexReleaseClient(ctx, p, cli)
	}

	if p.metroRemotePlugin != nil && p.metroRemotePlugin.storageOnline {
		p.mutexReleaseClient(ctx, p.metroRemotePlugin, metroCli)
	}
}

// UpdatePoolCapabilities used to update pool capabilities
func (p *OceanstorSanPlugin) UpdatePoolCapabilities(ctx context.Context, poolNames []string) (map[string]interface{},
	error) {
	vStoreQuotaMap, err := p.getVstoreCapacity(ctx)
	if err != nil {
		log.AddContext(ctx).Debugf("get vstore capacity failed, err: %v", err)
		vStoreQuotaMap = map[string]interface{}{}
	}

	return p.updatePoolCapabilities(ctx, poolNames, vStoreQuotaMap, "1")
}

func (p *OceanstorSanPlugin) getVstoreCapacity(ctx context.Context) (map[string]interface{}, error) {
	// only Dorado V6 6.1.5 and later versions need to get vStore's capacity.
	if !p.product.IsDoradoV6OrV7() ||
		(p.product.IsDoradoV6() && p.cli.GetStorageVersion() < constants.DoradoV615) ||
		p.cli.GetvStoreName() == "" {
		return map[string]interface{}{}, nil
	}
	vStore, err := p.cli.GetvStoreByName(ctx, p.cli.GetvStoreName())
	if err != nil {
		return nil, err
	}
	if vStore == nil {
		return nil, fmt.Errorf("not find vstore by name, name: %s", p.cli.GetvStoreName())
	}

	var sanCapacityQuota, sanFreeCapacityQuota int64

	if totalStr, ok := vStore["sanCapacityQuota"].(string); ok {
		sanCapacityQuota, err = strconv.ParseInt(totalStr, constants.DefaultIntBase, constants.DefaultIntBitSize)
	}
	if freeStr, ok := vStore["sanFreeCapacityQuota"].(string); ok {
		sanFreeCapacityQuota, err = strconv.ParseInt(freeStr, constants.DefaultIntBase, constants.DefaultIntBitSize)
	}
	if err != nil {
		log.AddContext(ctx).Warningf("parse vstore quota failed, error: %v", err)
		return nil, err
	}

	// if not set quota, sanCapacityQuota is 0, sanFreeCapacityQuota is -1
	if sanCapacityQuota == 0 || sanFreeCapacityQuota == -1 {
		return map[string]interface{}{}, nil
	}

	return map[string]interface{}{
		string(xuanwuV1.FreeCapacity):  sanFreeCapacityQuota * constants.AllocationUnitBytes,
		string(xuanwuV1.TotalCapacity): sanCapacityQuota * constants.AllocationUnitBytes,
		string(xuanwuV1.UsedCapacity):  (sanCapacityQuota - sanFreeCapacityQuota) * constants.AllocationUnitBytes,
	}, nil
}

// UpdateMetroRemotePlugin used to convert metroRemotePlugin to OceanstorSanPlugin
func (p *OceanstorSanPlugin) UpdateMetroRemotePlugin(ctx context.Context, remote StoragePlugin) {
	var ok bool
	p.metroRemotePlugin, ok = remote.(*OceanstorSanPlugin)
	if !ok {
		log.AddContext(ctx).Warningf("convert metroRemotePlugin to OceanstorSanPlugin failed, data: %v", remote)
	}
}

// CreateSnapshot used to create snapshot
func (p *OceanstorSanPlugin) CreateSnapshot(ctx context.Context,
	lunName, snapshotName string) (map[string]interface{}, error) {
	san := p.getSanObj()

	snapshotName = utils.GetSnapshotName(snapshotName)
	snapshot, err := san.CreateSnapshot(ctx, lunName, snapshotName)
	if err != nil {
		return nil, err
	}

	return snapshot, nil
}

// DeleteSnapshot used to delete snapshot
func (p *OceanstorSanPlugin) DeleteSnapshot(ctx context.Context,
	snapshotParentID, snapshotName string) error {
	san := p.getSanObj()

	snapshotName = utils.GetSnapshotName(snapshotName)
	err := san.DeleteSnapshot(ctx, snapshotName)
	if err != nil {
		return err
	}

	return nil
}

func (p *OceanstorSanPlugin) mutexGetClient(ctx context.Context) (client.OceanstorClientInterface, error) {
	p.clientMutex.Lock()
	defer p.clientMutex.Unlock()
	var err error
	if !p.storageOnline || p.clientCount == 0 {
		err = p.cli.Login(ctx)
		p.storageOnline = err == nil
		if err == nil {
			p.clientCount++
		}
	} else {
		p.clientCount++
	}

	return p.cli, err
}

func (p *OceanstorSanPlugin) getClient(ctx context.Context) (client.OceanstorClientInterface,
	client.OceanstorClientInterface, error) {
	cli, locErr := p.mutexGetClient(ctx)
	var metroCli client.OceanstorClientInterface
	var rmtErr error
	if p.metroRemotePlugin != nil {
		metroCli, rmtErr = p.metroRemotePlugin.mutexGetClient(ctx)
		if locErr != nil && rmtErr != nil {
			return nil, nil, errors.New("local and remote storage can not login")
		}
	} else {
		if locErr != nil {
			return nil, nil, errors.New("local storage can not login")
		}
	}
	return cli, metroCli, nil
}

func (p *OceanstorSanPlugin) getLunInfo(ctx context.Context, localCli, remoteCli client.OceanstorClientInterface,
	lunName string) (map[string]interface{}, error) {
	var lun map[string]interface{}
	var err error
	if p.storageOnline {
		lun, err = localCli.GetLunByName(ctx, lunName)
	} else if p.metroRemotePlugin != nil && p.metroRemotePlugin.storageOnline {
		lun, err = remoteCli.GetLunByName(ctx, lunName)
	} else {
		return nil, errors.New("both the local and remote storage are not online")
	}

	return lun, err
}

// UpdateBackendCapabilities to update the block storage capabilities
func (p *OceanstorSanPlugin) UpdateBackendCapabilities(ctx context.Context) (map[string]interface{},
	map[string]interface{}, error) {
	capabilities, specifications, err := p.OceanstorPlugin.UpdateBackendCapabilities(ctx)
	if err != nil {
		p.storageOnline = false
		return nil, nil, err
	}

	p.storageOnline = true
	p.updateHyperMetroCapability(capabilities)
	p.updateReplicaCapability(capabilities)
	return capabilities, specifications, nil
}

func (p *OceanstorSanPlugin) updateHyperMetroCapability(capabilities map[string]interface{}) {
	if metroSupport, exist := capabilities["SupportMetro"]; !exist || metroSupport == false {
		return
	}

	capabilities["SupportMetro"] = p.metroRemotePlugin != nil &&
		p.storageOnline && p.metroRemotePlugin.storageOnline
}

func (p *OceanstorSanPlugin) updateReplicaCapability(capabilities map[string]interface{}) {
	if metroReplica, exist := capabilities["SupportReplication"]; !exist || metroReplica == false {
		return
	}

	capabilities["SupportReplication"] = p.replicaRemotePlugin != nil
}

// Validate used to validate OceanstorSanPlugin parameters
func (p *OceanstorSanPlugin) Validate(ctx context.Context, param map[string]interface{}) error {
	log.AddContext(ctx).Infoln("Start to validate OceanstorSanPlugin parameters.")

	err := p.verifyOceanstorSanParam(ctx, param)
	if err != nil {
		return err
	}

	clientConfig, err := getNewClientConfig(ctx, param)
	if err != nil {
		return err
	}

	// Login verification
	cli, err := client.NewClient(ctx, clientConfig)
	if err != nil {
		return err
	}

	err = cli.ValidateLogin(ctx)
	if err != nil {
		return err
	}
	cli.Logout(ctx)

	return nil
}

func (p *OceanstorSanPlugin) verifyOceanstorSanParam(ctx context.Context, config map[string]interface{}) error {
	parameters, exist := config["parameters"].(map[string]interface{})
	if !exist {
		msg := fmt.Sprintf("Verify parameters: [%v] failed. \nparameters must be provided", config["parameters"])
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	protocol, exist := parameters["protocol"].(string)
	if !exist || (protocol != "iscsi" && protocol != "fc" && protocol != "roce" && protocol != "fc-nvme") {
		msg := fmt.Sprintf("Verify protocol: [%v] failed. \nprotocol must be provided and be one of "+
			"[iscsi, fc, roce, fc-nvme] for oceanstor-san backend\n", parameters["protocol"])
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	if protocol == "iscsi" || protocol == "roce" {
		portals, exist := parameters["portals"].([]interface{})
		if !exist {
			msg := fmt.Sprintf("Verify portals: [%v] failed. \nportals are required to configure for "+
				"iscsi or roce for oceanstor-san backend\n", parameters["portals"])
			log.AddContext(ctx).Errorln(msg)
			return errors.New(msg)
		}

		_, err := proto.VerifyIscsiPortals(ctx, portals)
		if err != nil {
			return err
		}
	}

	return nil
}

// DeleteDTreeVolume used to delete DTree volume
func (p *OceanstorSanPlugin) DeleteDTreeVolume(ctx context.Context, m map[string]interface{}) error {
	return errors.New("not implement")
}

// ExpandDTreeVolume used to expand DTree volume
func (p *OceanstorSanPlugin) ExpandDTreeVolume(context.Context, string, string, int64) (bool, error) {
	return false, errors.New("not implement")
}

// ModifyVolume used to modify volume hyperMetro status
func (p *OceanstorSanPlugin) ModifyVolume(ctx context.Context, volumeName string,
	modifyType pkgVolume.ModifyVolumeType, param map[string]string) error {

	return errors.New("not implement")
}

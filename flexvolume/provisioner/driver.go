package main

import (
	"errors"
	"flexvolume/config"
	"fmt"
	"runtime/debug"
	"storage/oceanstor/client"
	osVol "storage/oceanstor/volume"
	"strings"
	"utils"
	"utils/log"

	"github.com/kubernetes-sigs/sig-storage-lib-external-provisioner/controller"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	volUtil "k8s.io/kubernetes/pkg/volume/util"
)

type FlexProvisioner struct {
	backends map[string]*config.Backend
}

type Handler struct {
	backend          string
	cli              *client.Client
	remoteCli        *client.Client
	hyperMetroDomain string
	options          map[string]string
}

func NewProvisioner() *FlexProvisioner {
	return &FlexProvisioner{
		backends: config.Backends,
	}
}

func (p *FlexProvisioner) newHandler(backend string) (*Handler, error) {
	backendConf, exist := p.backends[backend]
	if !exist {
		msg := fmt.Sprintf("backend %s is not configured", backend)
		log.Errorln(msg)
		return nil, &(controller.IgnoredError{
			Reason: msg,
		})
	}

	urlsCopy := make([]string, len(backendConf.Urls))
	copy(urlsCopy, backendConf.Urls)

	cli := client.NewClient(urlsCopy, backendConf.User, backendConf.Password, backendConf.VstoreName)
	err := cli.Login()
	if err != nil {
		log.Errorf("Login backend %s error: %v", backend, err)
		return nil, err
	}

	var remoteCli *client.Client

	if backendConf.HyperMetroDomain != "" {
		for name, b := range p.backends {
			if name != backend && b.HyperMetroDomain == backendConf.HyperMetroDomain {
				urlsCopy := make([]string, len(b.Urls))
				copy(urlsCopy, b.Urls)

				remoteCli = client.NewClient(urlsCopy, b.User, b.Password, b.VstoreName)
				break
			}
		}
	}

	handler := &Handler{
		backend:          backend,
		cli:              cli,
		remoteCli:        remoteCli,
		hyperMetroDomain: backendConf.HyperMetroDomain,
	}

	return handler, nil
}

func (p *FlexProvisioner) freeHandler(handler *Handler) {
	handler.cli.Logout()
	if handler.remoteCli != nil {
		handler.remoteCli.Logout()
	}
}

func (p *FlexProvisioner) Provision(options controller.VolumeOptions) (volume *v1.PersistentVolume, err error) {
	backend, _ := options.Parameters["backend"]

	cloneFrom, exist := options.Parameters["cloneFrom"]
	if exist && cloneFrom != "" {
		backend, _ = utils.GetBackendAndVolume(cloneFrom)
	}

	if backend == "" {
		msg := "Cannot get the backend to create volume"
		log.Errorln(msg)
		err = errors.New(msg)
		return
	}

	handler, err := p.newHandler(backend)
	if err != nil {
		log.Errorf("New handler for provision error: %v", err)
		return
	}

	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprintf("Runtime error caught: %v", r)
			err = errors.New(msg)
			log.Errorln(msg)
			log.Errorf("%s", debug.Stack())
		}
		if handler != nil {
			p.freeHandler(handler)
		}

		log.Flush()
	}()

	volume, err = handler.createVolume(options)
	return
}

func (p *FlexProvisioner) Delete(volume *v1.PersistentVolume) (err error) {
	backend, volName := utils.GetBackendAndVolume(volume.Name)

	handler, err := p.newHandler(backend)
	if err != nil {
		log.Errorf("New handler for delete error: %v", err)
		return
	}

	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprintf("Runtime error caught: %v", r)
			err = errors.New(msg)
			log.Errorln(msg)
			log.Errorf("%s", debug.Stack())
		}
		if handler != nil {
			p.freeHandler(handler)
		}

		log.Flush()
	}()

	err = handler.deleteVolume(volume, volName)
	return
}

func (p *Handler) getParams(options controller.VolumeOptions) map[string]interface{} {
	params := map[string]interface{}{
		"name":        options.PVName,
		"description": "Created from Kubernetes FlexVolume Provisioner",
	}

	capacity := options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)]
	requestBytes := capacity.Value()
	params["capacity"] = volUtil.RoundUpSize(requestBytes, 512)

	paramKeys := []string{
		"alloctype",
		"storagepool",
		"qos",
		"authclient",
		"cloneFrom",
		"cloneSpeed",
		"hyperMetro",
		"remoteStoragePool",
	}

	for _, key := range paramKeys {
		if v, exist := options.Parameters[key]; exist && v != "" {
			params[strings.ToLower(key)] = v
		}
	}

	if v, exist := params["clonefrom"].(string); exist {
		_, params["clonefrom"] = utils.GetBackendAndVolume(v)
	}

	if v, exist := params["hypermetro"].(string); exist {
		params["hypermetro"] = utils.StrToBool(v)
	}

	if p.hyperMetroDomain != "" {
		params["metrodomain"] = p.hyperMetroDomain
	}

	return params
}

func (p *Handler) checkRequirements(params map[string]interface{}) error {
	hyperMetro, exist := params["hypermetro"].(bool)
	if exist && hyperMetro {
		if p.remoteCli == nil {
			return fmt.Errorf("No remote backend exists for Hypermetro volume")
		}

		err := p.remoteCli.Login()
		if err != nil {
			return err
		}

		remoteFeatures, err := p.remoteCli.GetLicenseFeature()
		if err != nil {
			return err
		}
		if !utils.IsSupportFeature(remoteFeatures, "HyperMetro") {
			return fmt.Errorf("Remote backend doesn't support Hypermetro feature")
		}

		localFeatures, err := p.cli.GetLicenseFeature()
		if err != nil {
			return err
		}
		if !utils.IsSupportFeature(localFeatures, "HyperMetro") {
			return fmt.Errorf("Local backend doesn't support Hypermetro feature")
		}
	}

	return nil
}

func (p *Handler) createVolume(options controller.VolumeOptions) (*v1.PersistentVolume, error) {
	log.Infof("Provision called for PV %s", options.PVName)

	params := p.getParams(options)

	var err error
	var driverType string

	err = p.checkRequirements(params)
	if err != nil {
		log.Errorf("Cannot satisfy volume creation requirements: %v", err)
		return nil, err
	}

	if options.Parameters["volumetype"] == "fs" {
		nas := osVol.NewNAS(p.cli)
		err = nas.Create(params)
		driverType = "huawei/nas"
	} else {
		san := osVol.NewSAN(p.cli, p.remoteCli)
		err = san.Create(params)
		driverType = "huawei/san"
	}

	if err != nil {
		log.Errorf("Create PV %s error: %v", options.PVName, err)
		return nil, err
	}

	log.Infof("PV %s created", options.PVName)
	lunName := strings.Replace(params["name"].(string), "_", "-", -1)

	annotations := map[string]string{
		"volumetype": options.Parameters["volumetype"],
	}

	hyperMetro, exist := params["hypermetro"].(bool)
	if exist && hyperMetro {
		annotations["hypermetro"] = "true"
	}

	return &v1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:        p.backend + "-" + lunName,
			Annotations: annotations,
		},
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeReclaimPolicy: options.PersistentVolumeReclaimPolicy,
			AccessModes:                   options.PVC.Spec.AccessModes,
			Capacity: v1.ResourceList{
				v1.ResourceName(v1.ResourceStorage): options.PVC.Spec.Resources.Requests[v1.ResourceName(v1.ResourceStorage)],
			},
			PersistentVolumeSource: v1.PersistentVolumeSource{
				FlexVolume: &v1.FlexPersistentVolumeSource{
					Driver: driverType,
					FSType: options.Parameters["fstype"],
				},
			},
		},
	}, nil
}

func (p *Handler) deleteVolume(volume *v1.PersistentVolume, volName string) error {
	log.Infof("Delete called for PV %s", volName)

	if v, exist := volume.Annotations["hypermetro"]; exist && v == "true" {
		if p.remoteCli == nil {
			return fmt.Errorf("No remote backend exists for Hypermetro volume")
		}

		err := p.remoteCli.Login()
		if err != nil {
			return err
		}
	}

	var err error

	if volume.Annotations["volumetype"] == "fs" {
		nas := osVol.NewNAS(p.cli)
		err = nas.Delete(volName)
	} else {
		san := osVol.NewSAN(p.cli, p.remoteCli)
		err = san.Delete(volName)
	}

	if err != nil {
		log.Errorf("Delete PV %s error: %v", volName, err)
		return err
	}

	log.Infof("PV %s deleted", volName)
	return nil
}

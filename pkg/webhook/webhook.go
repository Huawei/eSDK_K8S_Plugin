/*
Copyright (c) Huawei Technologies Co., Ltd. 2020-2025. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
  http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package webhook validate/mutate the request
package webhook

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"reflect"
	"sync"

	admissionV1 "k8s.io/api/admission/v1"
	admissionV1beta1 "k8s.io/api/admission/v1beta1"
	apisErrors "k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"

	xuanwuv1 "github.com/Huawei/eSDK_K8S_Plugin/v4/client/apis/xuanwu/v1"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/backend"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/pkg/utils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/cert"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/iputils"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

// Controller include webhook resources
type Controller struct {
	Recorder record.EventRecorder
	srv      *http.Server
	lock     sync.Mutex
	started  bool
}

// AdmissionWebHookType is the type of the webhook
type AdmissionWebHookType string

const (
	// AdmissionWebHookValidating is for validate webhook
	AdmissionWebHookValidating AdmissionWebHookType = "validating"

	// ClaimBoundFinalizer used when storageBackendClaim bound to a storageBackendContent
	ClaimBoundFinalizer string = "storagebackend.xuanwu.huawei.io/storagebackendclaim-bound-protection"
)

// Config uses to start the webhook server
type Config struct {
	NamespaceEnv     string
	DefaultNamespace string
	ServiceName      string
	SecretName       string

	WebHookPort int
	// address of webhook server
	WebHookAddress string
	WebHookType    AdmissionWebHookType

	PrivateKey     string
	PrivateCert    string
	HandleFuncPair []HandleFuncPair
}

// HandleFuncPair uses for add handle func
type HandleFuncPair struct {
	WebhookPath string
	WebHookFunc func(admissionV1.AdmissionReview) *admissionV1.AdmissionResponse
}

// AdmitFunc handles a v1 admission
var AdmitFunc func(admissionV1.AdmissionReview) *admissionV1.AdmissionResponse

// admitV1Func handles a v1 admission
type admitV1Func func(admissionV1.AdmissionReview) *admissionV1.AdmissionResponse

type admitV1beta1Func func(admissionV1beta1.AdmissionReview) *admissionV1beta1.AdmissionResponse

// admitHandler is a handler, for both validators and mutators, that supports multiple admission review versions
type admitHandler struct {
	admitV1      admitV1Func
	admitV1Beta1 admitV1beta1Func
}

func newDelegateToV1AdmitHandler(f admitV1Func) admitHandler {
	return admitHandler{
		admitV1:      f,
		admitV1Beta1: transformV1beta1AdmitFuncToV1AdmitFunc(f),
	}
}

func (c *Controller) getV1AdmissionReview(ctx context.Context, obj runtime.Object,
	gvk *schema.GroupVersionKind, admit admitHandler) (
	runtime.Object, error) {
	requestedAdmissionReview, ok := obj.(*admissionV1.AdmissionReview)
	if !ok {
		msg := fmt.Sprintf("Expected v1.AdmissionReview but got: %T", obj)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	responseAdmissionReview := &admissionV1.AdmissionReview{}
	responseAdmissionReview.SetGroupVersionKind(*gvk)
	responseAdmissionReview.Response = admit.admitV1(*requestedAdmissionReview)
	responseAdmissionReview.Response.UID = requestedAdmissionReview.Request.UID
	return responseAdmissionReview, nil
}

func (c *Controller) getV1Beta1AdmissionReview(ctx context.Context, obj runtime.Object,
	gvk *schema.GroupVersionKind, admit admitHandler) (
	runtime.Object, error) {
	requestedAdmissionReview, ok := obj.(*admissionV1beta1.AdmissionReview)
	if !ok {
		msg := fmt.Sprintf("Expected v1beta1.AdmissionReview but got: %T", obj)
		log.AddContext(ctx).Errorln(msg)
		return nil, errors.New(msg)
	}

	responseAdmissionReview := &admissionV1beta1.AdmissionReview{}
	responseAdmissionReview.SetGroupVersionKind(*gvk)
	responseAdmissionReview.Response = admit.admitV1Beta1(*requestedAdmissionReview)
	responseAdmissionReview.Response.UID = requestedAdmissionReview.Request.UID
	return responseAdmissionReview, nil
}

func (c *Controller) getRequestBody(ctx context.Context, w http.ResponseWriter, r *http.Request) ([]byte, error) {
	if r.Body == nil {
		msg := "expected request body to be non-empty"
		log.AddContext(ctx).Errorln(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return nil, errors.New(msg)
	}

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		msg := fmt.Sprintf("Request could not be decoded: %v", err)
		log.AddContext(ctx).Errorln(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return nil, errors.New(msg)
	}
	return data, nil
}

// serve handles the http portion of a request prior to handing to an admit function
func (c *Controller) serve(w http.ResponseWriter, r *http.Request, admit admitHandler) {
	var body []byte
	var err error
	ctx := context.Background()
	log.AddContext(ctx).Infof("Start to handle request: %v", r)
	if body, err = c.getRequestBody(ctx, w, r); err != nil {
		return
	}

	// verify the content type is accurate
	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		msg := fmt.Sprintf("contentType=%s, expect application/json", contentType)
		log.AddContext(ctx).Errorf(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	log.AddContext(ctx).Infof("handling request: %s", body)
	deserializer := Codecs.UniversalDeserializer()
	obj, gvk, err := deserializer.Decode(body, nil, nil)
	if err != nil {
		msg := fmt.Sprintf("Request could not be decoded: %v", err)
		log.AddContext(ctx).Errorln(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	var responseObj runtime.Object
	var admissionVersion string
	switch *gvk {
	case admissionV1.SchemeGroupVersion.WithKind("AdmissionReview"):
		responseObj, err = c.getV1AdmissionReview(ctx, obj, gvk, admit)
		admissionVersion = "v1.AdmissionReview"
	case admissionV1beta1.SchemeGroupVersion.WithKind("AdmissionReview"):
		responseObj, err = c.getV1Beta1AdmissionReview(ctx, obj, gvk, admit)
		admissionVersion = "v1beta1.AdmissionReview"
	default:
		err = errors.New("unsupported group version")
		admissionVersion = fmt.Sprintf("%v", gvk)
	}

	if err != nil {
		msg := fmt.Sprintf("Get %s error: %v", admissionVersion, err)
		log.AddContext(ctx).Errorln(msg)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}

	log.AddContext(ctx).Infof("sending response: %v", responseObj)
	respBytes, err := json.Marshal(responseObj)
	if err != nil {
		log.AddContext(ctx).Errorln(err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(respBytes); err != nil {
		log.AddContext(ctx).Errorln(err)
	}
	log.AddContext(ctx).Infoln("return response success")
}

func (c *Controller) getTlsCert(ctx context.Context, webHookCfg Config, ns string) (tls.Certificate, []byte, error) {
	var tlsCert tls.Certificate
	var caBytes []byte
	certSecrets, err := app.GetGlobalConfig().K8sUtils.GetSecret(ctx, webHookCfg.SecretName, ns)
	if err != nil && !apisErrors.IsNotFound(err) {
		log.AddContext(ctx).Errorf("Unable to retrieve %v secret: %v", webHookCfg.SecretName, err)
		return tls.Certificate{}, nil, err
	} else if apisErrors.IsNotFound(err) {
		dnsName := webHookCfg.ServiceName + "." + ns + ".svc"
		cn := fmt.Sprintf("%s CA", webHookCfg.ServiceName)
		caBundle, key, err := cert.GenerateCertificate(ctx, cn, dnsName)
		if err != nil {
			log.AddContext(ctx).Errorf("Unable to generate x509 certificate: %v", err)
			return tls.Certificate{}, nil, err
		}
		caBytes = caBundle

		tlsCert, err = cert.GetTLSCertificate(caBundle, key)
		if err != nil {
			log.AddContext(ctx).Errorf("Unable to create tls certificate: %v", err)
			return tls.Certificate{}, nil, err
		}

		_, err = CreateCertSecrets(ctx, webHookCfg, caBundle, key, ns)
		if err != nil {
			log.AddContext(ctx).Errorf("unable to create secrets for cert details: %v", err)
			return tls.Certificate{}, nil, err
		}
	} else {
		secretData, ok := certSecrets.Data[webHookCfg.PrivateKey]
		if !ok {
			return tls.Certificate{}, nil, fmt.Errorf("invalid secret key data")
		}

		caBundle, ok := certSecrets.Data[webHookCfg.PrivateCert]
		if !ok {
			return tls.Certificate{}, nil, fmt.Errorf("invalid secret certificate")
		}
		caBytes = caBundle

		tlsCert, err = cert.GetTLSCertificate(caBundle, secretData)
		if err != nil {
			log.AddContext(ctx).Errorf("unable to generate tls certs: %v", err)
			return tls.Certificate{}, nil, err
		}
	}
	return tlsCert, caBytes, nil
}

// Start uses to start the webhook server
func (c *Controller) Start(ctx context.Context, webHookCfg Config, admissionWebhooks []AdmissionWebHookCFG) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.started {
		return fmt.Errorf("webhook server has already been started")
	}

	tlsCert, caBundle, err := c.getTlsCert(ctx, webHookCfg, app.GetGlobalConfig().Namespace)
	if err != nil {
		log.AddContext(ctx).Errorf("Get TLS certs failed, error: %v", err)
		return err
	}

	wrapperWebHookAddr := webHookCfg.WebHookAddress
	wrapper := iputils.NewIPDomainWrapper(webHookCfg.WebHookAddress)
	if wrapper != nil {
		wrapperWebHookAddr = wrapper.GetFormatPortalIP()
	}

	c.srv = &http.Server{Addr: fmt.Sprintf("%s:%d", wrapperWebHookAddr, webHookCfg.WebHookPort),
		TLSConfig: &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{tlsCert}}}
	for _, pair := range webHookCfg.HandleFuncPair {
		serverRequest := func(w http.ResponseWriter, r *http.Request) {
			c.serve(w, r, newDelegateToV1AdmitHandler(pair.WebHookFunc))
		}
		http.HandleFunc(pair.WebhookPath, serverRequest)
	}

	go func() {
		err = c.srv.ListenAndServeTLS("", "")
		if err != nil && c.started {
			log.Errorf(" starting webhook server occur error, error is %v", err)
		}
	}()
	c.started = true
	log.AddContext(ctx).Infoln("Webhook server started")
	if webHookCfg.WebHookType == AdmissionWebHookValidating {
		for _, admission := range admissionWebhooks {
			if err := CreateValidateWebhook(ctx, admission, caBundle, app.GetGlobalConfig().Namespace); err != nil {
				return err
			}
		}
		return nil
	}

	log.AddContext(ctx).Errorf("Unsupported webhook type %s", webHookCfg.WebHookType)
	return errors.New("unsupported webhook type")
}

// Stop uses to stop the webhook server
func (c *Controller) Stop(ctx context.Context, webHookCfg Config,
	admissionWebhooks []AdmissionWebHookCFG) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	log.AddContext(ctx).Infof("Got OS shutdown signal, shutting down webhook server gracefully...")
	if !c.started {
		return fmt.Errorf("webhook server has not been started")
	}

	c.started = false
	if err := c.srv.Shutdown(ctx); err != nil {
		return err
	}

	return nil
}

func getNameSpaceFromEnv(webHookCfg Config) string {
	ns := os.Getenv(webHookCfg.NamespaceEnv)
	if ns == "" {
		ns = webHookCfg.DefaultNamespace
	}

	return ns
}

func getStorageBackendClaim(ctx context.Context, operation admissionV1.Operation, oldObjectRaw, newObjectRaw []byte) (
	*xuanwuv1.StorageBackendClaim, *xuanwuv1.StorageBackendClaim, error) {
	deserializer := Codecs.UniversalDeserializer()
	newClaim := &xuanwuv1.StorageBackendClaim{}
	if _, _, err := deserializer.Decode(newObjectRaw, nil, newClaim); err != nil {
		log.AddContext(ctx).Errorf("Decode new object %v failed, error: %v", newObjectRaw, err)
		return newClaim, &xuanwuv1.StorageBackendClaim{}, err
	}

	if operation == admissionV1.Create || operation == admissionV1.Connect {
		return newClaim, &xuanwuv1.StorageBackendClaim{}, nil
	}

	oldClaim := &xuanwuv1.StorageBackendClaim{}
	_, _, err := deserializer.Decode(oldObjectRaw, nil, oldClaim)
	if err != nil {
		log.AddContext(ctx).Errorf("Decode old object %v failed, error: %v", oldObjectRaw, err)
	}
	return newClaim, oldClaim, err
}

func validateCreate(ctx context.Context, claim *xuanwuv1.StorageBackendClaim) error {
	log.AddContext(ctx).Infof("Start to validateCreate %s.", utils.StorageBackendClaimKey(claim))
	defer log.AddContext(ctx).Infof("Finished validateCreate %s.", utils.StorageBackendClaimKey(claim))
	return validateCommon(ctx, claim)
}

func validateUpdate(ctx context.Context, newClaim, oldClaim *xuanwuv1.StorageBackendClaim) error {
	log.AddContext(ctx).Infof("Start to validateUpdate %s.", utils.StorageBackendClaimKey(newClaim))
	defer log.AddContext(ctx).Infof("Finished validateUpdate %s.", utils.StorageBackendClaimKey(newClaim))
	if reflect.DeepEqual(newClaim.Spec, oldClaim.Spec) && reflect.DeepEqual(
		newClaim.Annotations, oldClaim.Annotations) {
		return nil
	}

	if newClaim.Spec.Provider != oldClaim.Spec.Provider {
		msg := fmt.Sprintf("[provider] is forbidden changed with StorageBackendClaim %s",
			utils.StorageBackendClaimKey(newClaim))
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	return validateCommon(ctx, newClaim)
}

func validateDelete(ctx context.Context, claim *xuanwuv1.StorageBackendClaim) error {
	if claim.Finalizers == nil || len(claim.Finalizers) == 0 {
		return nil
	}

	for _, f := range claim.Finalizers {
		if f == ClaimBoundFinalizer {
			continue
		}
		msg := fmt.Sprintf("forbid delete StorageBackendClaim %s, there are some finalizers [%v]",
			utils.StorageBackendClaimKey(claim), claim.Finalizers)
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	return nil
}

func validateCommonClaim(ctx context.Context, claim *xuanwuv1.StorageBackendClaim) error {
	if claim.Spec.ConfigMapMeta == "" {
		msg := fmt.Sprintf("StorageBackendClaim %s's configmap [%s] is empty.",
			claim.Spec.ConfigMapMeta, utils.StorageBackendClaimKey(claim))
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	if claim.Spec.SecretMeta == "" {
		msg := fmt.Sprintf("StorageBackendClaim %s's secret [%s] is empty.",
			claim.Spec.SecretMeta, utils.StorageBackendClaimKey(claim))
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	if claim.Spec.Provider == "" {
		msg := fmt.Sprintf("Provider in StorageBackendClaim [%s] can not be empty",
			utils.StorageBackendClaimKey(claim))
		log.AddContext(ctx).Errorln(msg)
		return errors.New(msg)
	}

	log.AddContext(ctx).Infoln("validateCommonClaim succeeded.")
	return nil
}

func validateCommon(ctx context.Context, claim *xuanwuv1.StorageBackendClaim) error {
	if err := validateCommonClaim(ctx, claim); err != nil {
		return err
	}

	log.AddContext(ctx).Infof("claim name: %s", claim.Name)
	storageInfo, err := backend.GetStorageBackendInfo(ctx,
		utils.MakeMetaWithNamespace(app.GetGlobalConfig().Namespace, claim.Name),
		backend.NewGetBackendInfoArgsFromClaim(claim))
	if err != nil {
		return err
	}

	// make new backend, meanwhile check some common param
	targetBackend, err := backend.NewBackend(claim.Name, storageInfo)
	if err != nil {
		return err
	}

	err = targetBackend.Plugin.Validate(ctx, storageInfo)
	if err != nil {
		return err
	}

	return nil
}

func validateStorageBackendClaim(ctx context.Context, operation admissionV1.Operation,
	newClaim, oldClaim *xuanwuv1.StorageBackendClaim) error {
	switch operation {
	case admissionV1.Create:
		return validateCreate(ctx, newClaim)
	case admissionV1.Update:
		return validateUpdate(ctx, newClaim, oldClaim)
	case admissionV1.Delete:
		return validateDelete(ctx, oldClaim)
	case admissionV1.Connect:
		return nil
	default:
		msg := fmt.Sprintf("the operation [%s] to StorageBackendClaim [%s] is unknown, "+
			"refuse default", operation, newClaim.Name)
		log.AddContext(ctx).Errorln(msg)
	}
	return nil
}

func admitStorageBackendClaim(ar admissionV1.AdmissionReview) *admissionV1.AdmissionResponse {
	log.Infoln("Start admit StorageBackendClaim.")
	ctx := context.Background()
	newClaim, oldClaim, err := getStorageBackendClaim(ctx, ar.Request.Operation, ar.Request.OldObject.Raw,
		ar.Request.Object.Raw)
	if err != nil {
		log.Errorf("Failed to get StorageBackendClaim, error: %v", err)
		return getFalseAdmissionResponse(err)
	}

	err = validateStorageBackendClaim(ctx, ar.Request.Operation, newClaim, oldClaim)
	if err != nil {
		log.Errorf("Failed to validate StorageBackendClaim, error: %v", err)
		return getFalseAdmissionResponse(err)
	}

	log.AddContext(ctx).Infof("Successful admitting StorageBackendClaim %s.",
		utils.StorageBackendClaimKey(newClaim))
	return getTrueAdmissionResponse()
}

func getTrueAdmissionResponse() *admissionV1.AdmissionResponse {
	return &admissionV1.AdmissionResponse{
		Allowed: true,
		Result:  &metaV1.Status{},
	}
}

func getFalseAdmissionResponse(err error) *admissionV1.AdmissionResponse {
	return &admissionV1.AdmissionResponse{
		Allowed: false,
		Result: &metaV1.Status{
			Message: err.Error(),
		},
	}
}

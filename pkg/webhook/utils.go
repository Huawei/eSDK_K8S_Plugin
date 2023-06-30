/*
Copyright (c) Huawei Technologies Co., Ltd. 2022-2023. All rights reserved.

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

// Package webhook validate the request
package webhook

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"time"

	"k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"huawei-csi-driver/csi/app"
	"huawei-csi-driver/utils/log"
)

// GenerateCertificate Self Signed certificate using given CN, returns x509 cert
// and priv key in PEM format
func GenerateCertificate(ctx context.Context, cn string, dnsName string) ([]byte, []byte, error) {
	var err error
	pemCert := &bytes.Buffer{}

	// generate private key
	priv, err := ecdsa.GenerateKey(elliptic.P521(), rand.Reader)
	if err != nil {
		log.AddContext(ctx).Errorf("error generating crypt keys: %v", err)
		return nil, nil, err
	}

	// create certificate
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: cn,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
		DNSNames:              []string{dnsName},
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		log.AddContext(ctx).Errorf("Failed to create x509 certificate: %s", err)
		return nil, nil, err
	}

	err = pem.Encode(pemCert, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	if err != nil {
		log.AddContext(ctx).Errorf("Unable to encode x509 certificate to PEM format: %v", err)
		return nil, nil, err
	}

	b, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		log.AddContext(ctx).Errorf("Unable to marshal ECDSA private key: %v", err)
		return nil, nil, err
	}
	privateBlock := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   b,
	}

	return pemCert.Bytes(), pem.EncodeToMemory(&privateBlock), nil
}

// GetTLSCertificate from pub and priv key
func GetTLSCertificate(cert, priv []byte) (tls.Certificate, error) {
	return tls.X509KeyPair(cert, priv)
}

// CreateCertSecrets creates k8s secret to store signed cert data
func CreateCertSecrets(ctx context.Context, webHookCfg WebHook, cert, key []byte, ns string) (*v1.Secret, error) {
	secretData := make(map[string][]byte)
	secretData[webHookCfg.PrivateKey] = key
	secretData[webHookCfg.PrivateCert] = cert
	secret := &v1.Secret{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      webHookCfg.SecretName,
			Namespace: ns,
		},
		Data: secretData,
	}

	certSecret, err := app.GetGlobalConfig().K8sUtils.CreateSecret(ctx, secret)
	if err != nil {
		return nil, err
	}

	return certSecret, nil
}

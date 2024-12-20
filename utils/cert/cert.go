/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2022-2023. All rights reserved.
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

// Package cert provides methods for TLS certification
package cert

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
	"fmt"
	"math/big"
	"time"

	"google.golang.org/grpc/credentials"
	"k8s.io/api/core/v1"
	apisErrors "k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/Huawei/eSDK_K8S_Plugin/v4/csi/app"
	"github.com/Huawei/eSDK_K8S_Plugin/v4/utils/log"
)

const (
	certUntilYears           = 99
	serviceName              = "huawei-csi-controller"
	grpcSecretName           = "huawei-csi-controller-grpc-secret"
	grpcSecretKey            = "key"
	grpcSecretCert           = "cert"
	getGrpcSecretRetries     = 3
	getGrpcSecretRetryPeriod = time.Second
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
		NotAfter:              time.Now().AddDate(certUntilYears, 0, 0),
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

// GetGrpcCredential gets the gRPC credentials from Secret data in kubernetes cluster
//   - If the secret data that carries the TLS certification information exists in the cluster, just use it.
//   - If no secret is found, create and save it in cluster, and use it for gRPC.
func GetGrpcCredential(ctx context.Context) (credentials.TransportCredentials, error) {
	var pair x509KeyPair
	for {
		// If CSI controller is running on AA mode, create secret may be conflict.
		// When a conflict occurs, try again to get or create the secret.
		var err error
		pair, err = getOrCreateX509PairFromSecret(ctx, grpcSecretName, app.GetGlobalConfig().Namespace)
		if err == nil {
			break
		}

		if !apisErrors.IsAlreadyExists(err) {
			return nil, err
		}

		time.Sleep(getGrpcSecretRetryPeriod)
	}

	tlsCert, err := GetTLSCertificate(pair.certPEMBlock, pair.keyPEMBlock)
	if err != nil {
		return nil, fmt.Errorf("get TLS certificate failed, error: %v", err)
	}

	return credentials.NewTLS(&tls.Config{
		MinVersion:   tls.VersionTLS12,
		Certificates: []tls.Certificate{tlsCert},
	}), nil
}

type x509KeyPair struct {
	certPEMBlock []byte
	keyPEMBlock  []byte
}

func getOrCreateX509PairFromSecret(ctx context.Context, secretName string, namespace string) (x509KeyPair, error) {
	pair := x509KeyPair{}
	secret, err := app.GetGlobalConfig().K8sUtils.GetSecret(ctx, secretName, namespace)
	if err == nil {
		// there is already a secret,
		if secret == nil {
			return pair, fmt.Errorf("get nil secret %s in namespace %s", secretName, namespace)
		}

		if secret.Data == nil {
			return pair, fmt.Errorf("get nil data of secret %s in namespace %s", secretName, namespace)
		}

		var exist bool
		pair.certPEMBlock, exist = secret.Data[grpcSecretCert]
		if !exist {
			return pair, fmt.Errorf("invalid secret cert")
		}

		pair.keyPEMBlock, exist = secret.Data[grpcSecretKey]
		if !exist {
			return pair, fmt.Errorf("invalid secret key")
		}

		return pair, nil
	}

	if !apisErrors.IsNotFound(err) {
		// unexpected error, return it
		return pair, fmt.Errorf("get secret %s in namespace %s failed, error: %v", secretName, namespace, err)
	}

	// not found, create a new secret
	dnsName := serviceName + "." + namespace + ".svc"
	cn := fmt.Sprintf("%s CA", serviceName)
	pair.certPEMBlock, pair.keyPEMBlock, err = GenerateCertificate(ctx, cn, dnsName)
	if err != nil {
		return pair, fmt.Errorf("generate certificate failed when get gRPC credential, error: %v", err)
	}

	if err = createSecretForGrpcServer(ctx, pair, secretName, namespace); err != nil {
		return pair, err
	}

	return pair, nil
}

func createSecretForGrpcServer(ctx context.Context, pair x509KeyPair, name, namespace string) error {
	secretData := make(map[string][]byte)
	secretData[grpcSecretKey] = pair.keyPEMBlock
	secretData[grpcSecretCert] = pair.certPEMBlock
	secret := &v1.Secret{ObjectMeta: metaV1.ObjectMeta{Name: name, Namespace: namespace}, Data: secretData}
	_, err := app.GetGlobalConfig().K8sUtils.CreateSecret(ctx, secret)
	return err
}

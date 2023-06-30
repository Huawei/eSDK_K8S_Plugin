/*
 *  Copyright (c) Huawei Technologies Co., Ltd. 2020-2023. All rights reserved.
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

package pwd

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
)

var (
	commonIV = []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}
)

//加密
func Encrypt(password, keyText string) (string, error) {
	plaintext := []byte(password)
	bytesText := []byte(keyText)
	// 创建加密算法aes
	c, err := aes.NewCipher(bytesText)
	if err != nil {
		return "", err
	}

	ciphertext := make([]byte, len(plaintext))

	//加密字符串
	cfb := cipher.NewCFBEncrypter(c, commonIV)
	cfb.XORKeyStream(ciphertext, plaintext)

	return hex.EncodeToString(ciphertext), nil
}

//解密
func Decrypt(code, keyText string) (string, error) {
	ciphertext, err := hex.DecodeString(code)
	if err != nil {
		return "", err
	}

	bytesText := []byte(keyText)
	// 创建加密算法aes
	c, err := aes.NewCipher(bytesText)
	if err != nil {
		return "", err
	}

	plaintextCopy := make([]byte, len(ciphertext))

	// 解密字符串
	cfbdec := cipher.NewCFBDecrypter(c, commonIV)
	cfbdec.XORKeyStream(plaintextCopy, ciphertext)

	return string(plaintextCopy), nil
}

package client

import "testing"

const csiSecret = "huawei-csi-secret"
const defaultNameSpace = "huawei-csi"

const normalCaseExpected = `
apiVersion: v1
kind: Secret
metadata:
  name: huawei-csi-secret
  namespace: huawei-csi
type: Opaque
stringData:
  secret.json: |
    {
      "secrets": {
        "stringDataKey1": stringDataVal1
      }
    }
`

const emptyStringDataExpected = `
apiVersion: v1
kind: Secret
metadata:
  name: huawei-csi-secret
  namespace: huawei-csi
type: Opaque
stringData:
  secret.json: |
`

func TestGetSecretYAML(t *testing.T) {
	stringData := map[string]string{"stringDataKey1": "stringDataVal1"}

	cases := []struct {
		CaseName              string
		secretName, namespace string
		stringData            map[string]string
		Expected              string
	}{
		{"Normal", csiSecret, defaultNameSpace, stringData, normalCaseExpected},
		{"EmptyStringData", csiSecret, defaultNameSpace, nil, emptyStringDataExpected},
	}

	for _, c := range cases {
		t.Run(c.CaseName, func(t *testing.T) {
			if ans := GetSecretYAML(c.secretName, c.namespace, c.stringData); ans != c.Expected {
				t.Errorf("Test GetSecretYAML failed.\nExpected:%s \nGet:%s", c.Expected, ans)
			}
		})
	}
}

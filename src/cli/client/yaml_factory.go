package client

import (
	"fmt"
	"strings"
)

const secretYAMLTemplate = `
apiVersion: v1
kind: Secret
metadata:
  name: {SECRET_NAME}
  namespace: {NAMESPACE}
type: Opaque
stringData: 
  secret.json: |
`


func GetSecretYAML(secretName, namespace string, data, stringData map[string]string) string {
	secretYAML := strings.ReplaceAll(secretYAMLTemplate, "{SECRET_NAME}", secretName)
	secretYAML = strings.ReplaceAll(secretYAML, "{NAMESPACE}", namespace)


	if data != nil {
		secretYAML += "data:\n"
		for key, value := range data {
			secretYAML += fmt.Sprintf("  %s: %s\n", key, value)
		}
	}

	if stringData != nil {
		secretYAML += "    {\n      \"secrets\": {\n"
		lenData := len(stringData)
		index := 1
		for key, value := range stringData {
			if index < lenData {
				secretYAML += fmt.Sprintf("        \"%s\": %s,\n", key, value)
			} else {
				secretYAML += fmt.Sprintf("        \"%s\": %s\n", key, value)
			}
			index += 1
		}
		secretYAML += "      }\n    }\n"
	}

	return secretYAML
}

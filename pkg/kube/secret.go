package kube

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/IBM/argocd-vault-plugin/pkg/vault"
	corev1 "k8s.io/api/core/v1"
	k8yamldecoder "k8s.io/apimachinery/pkg/util/yaml"
	k8yaml "sigs.k8s.io/yaml"
)

type SecretTemplate struct {
	Resource
}

func NewSecretTemplate(template map[string]interface{}, vault vault.VaultType) (*SecretTemplate, error) {
	path := os.Getenv("VAULT_PATH_PREFIX")
	data, err := vault.GetSecrets(path)
	if err != nil {
		return nil, err
	}

	return &SecretTemplate{
		Resource{
			templateData: template,
			vaultData:    data,
		},
	}, nil
}

func (d *SecretTemplate) Replace() error {

	// Replace metadata normally
	metadata, ok := d.templateData["metadata"].(map[string]interface{})
	if ok {
		replaceInner(&d.Resource, &metadata, genericReplacement)
		if len(d.replacementErrors) != 0 {

			// TODO format multiple errors nicely
			return fmt.Errorf("Replace: could not replace all placeholders in SecretTemplate metadata: %s", d.replacementErrors)
		}
	}

	// Replace the actual secrets with []byte's
	data, ok := d.templateData["data"].(map[string]interface{})
	if ok {
		replaceInner(&d.Resource, &data, secretReplacement)
		if len(d.replacementErrors) != 0 {

			// TODO format multiple errors nicely
			return fmt.Errorf("Replace: could not replace all placeholders in SecretTemplate data: %s", d.replacementErrors)
		}
	}

	return nil
}

func secretReplacement(key, value string, vaultData map[string]interface{}) (_ interface{}, err []error) {
	var byteData []byte
	res, err := genericReplacement(key, value, vaultData)

	// We have to return []byte for k8s secrets,
	// so we convert whatever is in Vault
	switch res.(type) {
	case int:
		{
			byteData = []byte(strconv.Itoa(res.(int)))
		}
	case string:
		{
			byteData = []byte(res.(string))
		}
	}

	return byteData, err
}

func (d *SecretTemplate) ToYAML() (string, error) {
	jsondata, _ := json.Marshal(d.templateData)
	decoder := k8yamldecoder.NewYAMLOrJSONDecoder(bytes.NewReader(jsondata), 1000)
	kubeResource := corev1.Secret{}
	err := decoder.Decode(&kubeResource)
	if err != nil {
		return "", fmt.Errorf("ToYAML: could not convert replaced template into Secret: %s", err)
	}
	res, err := k8yaml.Marshal(&kubeResource)
	if err != nil {
		return "", fmt.Errorf("ToYAML: could not export Secret into YAML: %s", err)
	}
	return string(res), nil
}

package credentials

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"os"

	"github.com/BurntSushi/toml"
	uuid "github.com/satori/go.uuid"
	"k8s.io/klog"
)

const (
	TempAzureCredentialFilePath = "/tmp/azure.json"
	defaultLocation             = "eastus2"
)

// CredentialsConfig is used in Prow to store Azure credentials
type CredentialsConfig struct {
	Creds CredentialsFromProw
}

// CredentialsFromProw is used in Prow to store Azure credentials
type CredentialsFromProw struct {
	ClientID           string
	ClientSecret       string
	TenantID           string
	SubscriptionID     string
	StorageAccountName string
	StorageAccountKey  string
}

// Credentials is used in Azure File CSI Driver to store Azure credentials
type Credentials struct {
	TenantID        string
	SubscriptionID  string
	AADClientID     string
	AADClientSecret string
	ResourceGroup   string
	Location        string
}

// Get returns the Azure credentials needed to perform the test
func Get() (*Credentials, error) {
	// Search credentials through env vars first
	tenantId := os.Getenv("tenantId")
	subscriptionId := os.Getenv("subscriptionId")
	aadClientId := os.Getenv("aadClientId")
	aadClientSecret := os.Getenv("aadClientSecret")

	resourceGroup := os.Getenv("resourceGroup")
	if resourceGroup == "" {
		resourceGroup = "azurefile-csi-driver-test-" + uuid.NewV1().String()
	}

	location := os.Getenv("location")
	if location == "" {
		location = defaultLocation
	}

	if tenantId != "" && subscriptionId != "" && aadClientId != "" && aadClientSecret != "" {
		return parseAndExecuteTemplate(tenantId, subscriptionId, aadClientId, aadClientSecret, resourceGroup, location)
	}

	// If credentials are not supplied through env vars, we need to obtain credentials from env var AZURE_CREDENTIALS
	// and convert it to AZURE_CREDENTIAL_FILE for sanity and integration tests if we are testing in Prow
	// https://github.com/kubernetes/test-infra/blob/master/config/jobs/kubernetes/cloud-provider-azure/cloud-provider-azure-config.yaml#L5
	if azureCredentialsPath, ok := os.LookupEnv("AZURE_CREDENTIALS"); ok {
		klog.V(2).Infof("Running in Prow, converting AZURE_CREDENTIALS to AZURE_CREDENTIAL_FILE")
		c, err := getCredentialsFromAzureCredentials(azureCredentialsPath)
		if err != nil {
			return nil, err
		}
		return parseAndExecuteTemplate(c.TenantID, c.SubscriptionID, c.ClientID, c.ClientSecret, resourceGroup, location)
	}

	return nil, fmt.Errorf("AZURE_CREDENTIALS is not set. You will need to set $tenantId, $subscriptionId, $aadClientId and $aadClientSecret")
}

// getCredentialsFromAzureCredentials parses the azure credentials toml (AZURE_CREDENTIALS)
// in Prow and return the credential information usable to Azure File CSI driver
func getCredentialsFromAzureCredentials(azureCredentialsPath string) (*CredentialsFromProw, error) {
	content, err := ioutil.ReadFile(azureCredentialsPath)
	klog.V(2).Infof("Reading credentials file %v", azureCredentialsPath)
	if err != nil {
		return nil, fmt.Errorf("error reading credentials file %v %v", azureCredentialsPath, err)
	}

	c := CredentialsConfig{}
	if err := toml.Unmarshal(content, &c); err != nil {
		return nil, fmt.Errorf("error parsing credentials file %v %v", azureCredentialsPath, err)
	}

	return &c.Creds, nil
}

// parseAndExecuteTemplate replaces credential placeholders in hack/template/azure.json with actual credentials
func parseAndExecuteTemplate(tenantId, subscriptionId, aadClientId, aadClientSecret, resourceGroup, location string) (*Credentials, error) {
	t, err := template.ParseFiles("../../hack/template/azure.json")
	if err != nil {
		return nil, fmt.Errorf("error parsing hack/template/azure.json %v", err)
	}

	f, err := os.Create(TempAzureCredentialFilePath)
	if err != nil {
		return nil, fmt.Errorf("error creating %s %v", TempAzureCredentialFilePath, err)
	}
	defer f.Close()

	c := Credentials{
		tenantId,
		subscriptionId,
		aadClientId,
		aadClientSecret,
		resourceGroup,
		location,
	}
	err = t.Execute(f, c)
	if err != nil {
		return nil, fmt.Errorf("error executing parsed azure credential file tempalte %v", err)
	}

	return &c, nil
}

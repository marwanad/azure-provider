package e2e

import (
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/compute"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/network"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/resources"
)

// NewAzureServicesClient returns a new instance of the services.AzureClients object.
func NewAzureServicesClient(subscriptionID string) (*services.AzureClients, error) {
	authorizer, err := auth.NewAuthorizerFromEnvironment()
	if err != nil {
		return nil, err
	}

	azureComputeClient := compute.NewService(subscriptionID)
	azureComputeClient.SetAuthorizer(authorizer)
	azureResourceManagementClient := resources.NewService(subscriptionID)
	azureResourceManagementClient.SetAuthorizer(authorizer)
	azureNetworkClient := network.NewService(subscriptionID)
	azureNetworkClient.SetAuthorizer(authorizer)
	return &services.AzureClients{
		Compute:            azureComputeClient,
		Resourcemanagement: azureResourceManagementClient,
		Network:            azureNetworkClient,
	}, nil
}

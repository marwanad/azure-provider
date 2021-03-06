/*
Copyright 2018 The Kubernetes Authors.
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

package cluster

import (
	"fmt"
	"os"

	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	azureconfigv1 "sigs.k8s.io/cluster-api-provider-azure/pkg/apis/azureprovider/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/network"
	"sigs.k8s.io/cluster-api-provider-azure/pkg/cloud/azure/services/resources"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AzureClusterClient holds the Azure SDK Client and Kubernetes Client objects.
type AzureClusterClient struct {
	services *services.AzureClients
	client   client.Client
}

// ClusterActuatorParams holds the Azure SDK Client and Kubernetes Client objects for the cluster actuator.
type ClusterActuatorParams struct {
	Services *services.AzureClients
	Client   client.Client
}

// NewClusterActuator returns a new instance of AzureClusterClient.
func NewClusterActuator(params ClusterActuatorParams) (*AzureClusterClient, error) {
	azureServicesClients, err := azureServicesClientOrDefault(params)
	if err != nil {
		return nil, err
	}

	return &AzureClusterClient{
		services: azureServicesClients,
		client:   params.Client,
	}, nil
}

// Reconcile creates or applies updates to the cluster.
func (azure *AzureClusterClient) Reconcile(cluster *clusterv1.Cluster) error {
	glog.Infof("Reconciling cluster %v.", cluster.Name)

	clusterConfig, err := clusterProviderFromProviderSpec(cluster.Spec.ProviderSpec)
	if err != nil {
		return fmt.Errorf("error loading cluster provider config: %v", err)
	}

	// Reconcile resource group
	_, err = azure.resources().CreateOrUpdateGroup(clusterConfig.ResourceGroup, clusterConfig.Location)
	if err != nil {
		return fmt.Errorf("failed to create or update resource group: %v", err)
	}

	// Reconcile network security group
	networkSGFuture, err := azure.network().CreateOrUpdateNetworkSecurityGroup(clusterConfig.ResourceGroup, "ClusterAPINSG", clusterConfig.Location)
	if err != nil {
		return fmt.Errorf("error creating or updating network security group: %v", err)
	}
	err = azure.network().WaitForNetworkSGsCreateOrUpdateFuture(*networkSGFuture)
	if err != nil {
		return fmt.Errorf("error waiting for network security group creation or update: %v", err)
	}

	// Reconcile virtual network
	vnetFuture, err := azure.network().CreateOrUpdateVnet(clusterConfig.ResourceGroup, "", clusterConfig.Location)
	if err != nil {
		return fmt.Errorf("error creating or updating virtual network: %v", err)
	}
	err = azure.network().WaitForVnetCreateOrUpdateFuture(*vnetFuture)
	if err != nil {
		return fmt.Errorf("error waiting for virtual network creation or update: %v", err)
	}
	return nil
}

// Delete the cluster.
func (azure *AzureClusterClient) Delete(cluster *clusterv1.Cluster) error {
	clusterConfig, err := clusterProviderFromProviderSpec(cluster.Spec.ProviderSpec)
	if err != nil {
		return fmt.Errorf("error loading cluster provider config: %v", err)
	}
	resp, err := azure.resources().CheckGroupExistence(clusterConfig.ResourceGroup)
	if err != nil {
		return fmt.Errorf("error checking for resource group existence: %v", err)
	}
	if resp.StatusCode == 404 {
		return fmt.Errorf("resource group %v does not exist", clusterConfig.ResourceGroup)
	}

	groupsDeleteFuture, err := azure.resources().DeleteGroup(clusterConfig.ResourceGroup)
	if err != nil {
		return fmt.Errorf("error deleting resource group: %v", err)
	}
	err = azure.resources().WaitForGroupsDeleteFuture(groupsDeleteFuture)
	if err != nil {
		return fmt.Errorf("error waiting for resource group deletion: %v", err)
	}
	return nil
}

func azureServicesClientOrDefault(params ClusterActuatorParams) (*services.AzureClients, error) {
	if params.Services != nil {
		return params.Services, nil
	}

	authorizer, err := auth.NewAuthorizerFromEnvironment()
	if err != nil {
		return nil, fmt.Errorf("Failed to get OAuth config: %v", err)
	}
	subscriptionID := os.Getenv("AZURE_SUBSCRIPTION_ID")
	if subscriptionID == "" {
		return nil, fmt.Errorf("error creating azure services. Environment variable AZURE_SUBSCRIPTION_ID is not set")
	}
	azureNetworkClient := network.NewService(subscriptionID)
	azureNetworkClient.SetAuthorizer(authorizer)
	azureResourceManagementClient := resources.NewService(subscriptionID)
	azureResourceManagementClient.SetAuthorizer(authorizer)
	return &services.AzureClients{
		Network:            azureNetworkClient,
		Resourcemanagement: azureResourceManagementClient,
	}, nil
}

func (azure *AzureClusterClient) network() services.AzureNetworkClient {
	return azure.services.Network
}

func (azure *AzureClusterClient) resources() services.AzureResourceManagementClient {
	return azure.services.Resourcemanagement
}

func clusterProviderFromProviderSpec(providerSpec clusterv1.ProviderSpec) (*azureconfigv1.AzureClusterProviderSpec, error) {
	var config azureconfigv1.AzureClusterProviderSpec
	if err := yaml.Unmarshal(providerSpec.Value.Raw, &config); err != nil {
		return nil, err
	}
	return &config, nil
}

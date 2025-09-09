package main

import (
	"context"
	"fmt"
	"strings"

	kclient "github.com/giantswarm/xfnlib/pkg/auth/kubernetes"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/giantswarm/xfnlib/pkg/composite"
)

type AccountInfo struct {
	AccountID         string `json:"accountId"`
	RoleName          string `json:"roleName"`
	ProviderConfigRef string `json:"providerConfigRef"`
}

// DiscoverAccounts discovers AWS accounts by examining ProviderConfig resources and their corresponding AWSCluster resources.
// It extracts account information from roleARNs in ProviderConfig specs and creates a list of unique accounts
// to avoid duplicate OIDC configurations.
//
// Parameters:
//   - patchTo: The target path where the discovered account information should be patched
//   - composed: The composite resource composition to work with
//
// Returns:
//   - error: An error if the discovery process fails, nil otherwise
//
// The function performs the following steps:
// 1. Lists all ProviderConfig resources of type aws.upbound.io/v1beta1
// 2. Lists all AWSCluster resources of type infrastructure.cluster.x-k8s.io/v1beta2
// 3. Filters ProviderConfigs to only include those with matching AWSCluster resources
// 4. Extracts roleARN from each ProviderConfig's webIdentity credentials
// 5. Parses account IDs from roleARNs and deduplicates them
// 6. Creates AccountInfo structs containing account ID, role name, and provider config reference
// 7. Patches the discovered account information to the target path in the composite resource
func (f *Function) DiscoverAccounts(patchTo string, composed *composite.Composition) error {

	client, err := kclient.Client()
	if err != nil {
		return err
	}

	// Get all ProviderConfig resources
	providerConfigs := &unstructured.UnstructuredList{}
	providerConfigs.SetAPIVersion("aws.upbound.io/v1beta1")
	providerConfigs.SetKind("ProviderConfig")

	if err := client.List(context.Background(), providerConfigs); err != nil {
		return fmt.Errorf("cannot list ProviderConfig resources: %w", err)
	}

	// get all awsclusters in all namespaces infrastructure.cluster.x-k8s.io/v1beta2 AWSCluster
	awsClusters := &unstructured.UnstructuredList{}
	awsClusters.SetAPIVersion("infrastructure.cluster.x-k8s.io/v1beta2")
	awsClusters.SetKind("AWSCluster")

	if err := client.List(context.Background(), awsClusters); err != nil {
		return fmt.Errorf("cannot list AWSCluster resources: %w", err)
	}

	// Track unique account IDs to avoid duplicates (avoid creating OIDC multiple times in the same account)
	seenAccountIDs := make(map[string]bool)
	var v []AccountInfo

	for _, item := range providerConfigs.Items {
		// check if there is any AWScluster with the same name, otherwise skip this ProviderConfig
		found := false
		for _, cluster := range awsClusters.Items {
			if cluster.GetName() == item.GetName() {
				found = true
				break
			}
		}
		if !found {
			continue
		}

		// Extract roleARN from the ProviderConfig spec
		roleARN, found, err := unstructured.NestedString(item.Object, "spec", "credentials", "webIdentity", "roleARN")
		if err != nil || !found || roleARN == "" {
			f.log.Debug("cannot get roleARN from ProviderConfig", "error", err)
			continue
		}

		// Extract account ID from roleARN (format: arn:aws:iam::ACCOUNT_ID:role/ROLE_NAME)
		parts := strings.Split(roleARN, ":")
		if len(parts) < 5 {
			f.log.Debug("cannot get account ID from roleARN", "roleARN", roleARN)
			continue
		}
		accountID := parts[4]

		// Skip if we've already seen this account ID
		if seenAccountIDs[accountID] {
			continue
		}
		seenAccountIDs[accountID] = true

		// Extract role name from roleARN
		roleParts := strings.Split(roleARN, "/")
		if len(roleParts) < 2 {
			f.log.Debug("cannot get role name from roleARN", "roleARN", roleARN)
			continue
		}
		roleName := roleParts[len(roleParts)-1]

		v = append(v, AccountInfo{
			AccountID:         accountID,
			RoleName:          roleName,
			ProviderConfigRef: item.GetName(),
		})
	}

	err = f.patchFieldValueToObject(patchTo, v, composed.DesiredComposite.Resource)
	return err
}

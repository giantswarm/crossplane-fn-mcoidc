package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	kclient "github.com/giantswarm/xfnlib/pkg/auth/kubernetes"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/giantswarm/xfnlib/pkg/composite"
)

func (f *Function) DiscoverAccounts(patchTo string, composed *composite.Composition) error {

	client, err := kclient.Client()
	if err != nil {
		return err
	}

	type AccountInfo struct {
		AccountID         string `json:"accountId"`
		RoleName          string `json:"roleName"`
		ProviderConfigRef string `json:"providerConfigRef"`
	}

	// Get all ProviderConfig resources
	providerConfigs := &unstructured.UnstructuredList{}
	providerConfigs.SetAPIVersion("aws.upbound.io/v1beta1")
	providerConfigs.SetKind("ProviderConfig")

	if err := client.List(context.Background(), providerConfigs); err != nil {
		return fmt.Errorf("cannot list ProviderConfig resources: %w", err)
	}

	// Track unique roleARNs to avoid duplicates
	seenRoleARNs := make(map[string]bool)
	var v []AccountInfo

	for _, item := range providerConfigs.Items {
		// Extract roleARN from the ProviderConfig spec
		roleARN, found, err := unstructured.NestedString(item.Object, "spec", "credentials", "webIdentity", "roleARN")
		if err != nil || !found || roleARN == "" {
			f.log.Debug("cannot get roleARN from ProviderConfig", "error", err)
			continue
		}

		// Skip if we've already seen this roleARN
		if seenRoleARNs[roleARN] {
			continue
		}
		seenRoleARNs[roleARN] = true

		// Extract account ID from roleARN (format: arn:aws:iam::ACCOUNT_ID:role/ROLE_NAME)
		parts := strings.Split(roleARN, ":")
		if len(parts) < 5 {
			continue
		}
		accountID := parts[4]

		// Extract role name from roleARN
		roleParts := strings.Split(roleARN, "/")
		if len(roleParts) < 2 {
			continue
		}
		roleName := roleParts[len(roleParts)-1]

		v = append(v, AccountInfo{
			AccountID:         accountID,
			RoleName:          roleName,
			ProviderConfigRef: item.GetName(),
		})
	}

	b := &bytes.Buffer{}

	if err := json.NewEncoder(b).Encode(&v); err != nil {
		return fmt.Errorf("cannot encode to JSON: %w", err)
	}

	err = f.patchFieldValueToObject(patchTo, b.Bytes(), composed.DesiredComposite.Resource)
	return err
}

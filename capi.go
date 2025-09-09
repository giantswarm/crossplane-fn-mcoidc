package main

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/giantswarm/xfnlib/pkg/composite"
)

func (f *Function) DiscoverAccounts(patchTo string, composed *composite.Composition) error {

	type AccountInfo struct {
		AccountID string `json:"accountId"`
		RoleName  string `json:"roleName"`
	}

	v := []AccountInfo{
		// TODO: Populate with actual account discovery logic
	}

	b := &bytes.Buffer{}

	if err := json.NewEncoder(b).Encode(&v); err != nil {
		return fmt.Errorf("cannot encode to JSON: %w", err)
	}

	err := f.patchFieldValueToObject(patchTo, b.Bytes(), composed.DesiredComposite.Resource)
	return err
}

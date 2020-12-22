// Copyright 2020 VMware, Inc.
// SPDX-License-Identifier: Apache-2.0

package lockconfig

import (
	"fmt"
	"io/ioutil"

	regname "github.com/google/go-containerregistry/pkg/name"
	"sigs.k8s.io/yaml"
)

const (
	BundleLockKind       = "BundleLock"
	BundleLockAPIVersion = "imgpkg.carvel.dev/v1alpha1"
)

type BundleLock struct {
	LockVersion
	Bundle BundleRef `yaml:"bundle"`
}

type BundleRef struct {
	Image string `yaml:"image,omitempty"`
	Tag   string `yaml:"tag,omitempty"`
}

func NewBundleLockFromPath(path string) (BundleLock, error) {
	bs, err := ioutil.ReadFile(path)
	if err != nil {
		return BundleLock{}, fmt.Errorf("Reading path %s: %s", path, err)
	}

	return NewBundleLockFromBytes(bs)
}

func NewBundleLockFromBytes(data []byte) (BundleLock, error) {
	var lock BundleLock

	err := yaml.Unmarshal(data, &lock)
	if err != nil {
		return lock, fmt.Errorf("Unmarshaling bundle lock: %s", err)
	}

	err = lock.Validate()
	if err != nil {
		return lock, fmt.Errorf("Validating bundle lock: %s", err)
	}

	return lock, nil
}

func (c BundleLock) Validate() error {
	if c.APIVersion != BundleLockAPIVersion {
		return fmt.Errorf("Validating apiVersion: Unknown version (known: %s)", BundleLockAPIVersion)
	}
	if c.Kind != BundleLockKind {
		return fmt.Errorf("Validating kind: Unknown kind (known: %s)", BundleLockKind)
	}
	if _, err := regname.NewDigest(c.Bundle.Image); err != nil {
		return fmt.Errorf("Expected ref to be in digest form, got '%s'", c.Bundle.Image)
	}
	return nil
}

func (c BundleLock) AsBytes() ([]byte, error) {
	bs, err := yaml.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("Marshaling config: %s", err)
	}

	return bs, nil
}

func (c BundleLock) WriteToPath(path string) error {
	bs, err := c.AsBytes()
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, bs, 600)
}

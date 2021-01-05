// Copyright 2020 VMware, Inc.
// SPDX-License-Identifier: Apache-2.0

package lockconfig

import (
	"fmt"
)

type LockVersion struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
}

func NewLockFromPath(path string) (*BundleLock, *ImagesLock, error) {
	bundleLock, err := NewBundleLockFromPath(path)
	if err == nil {
		return &bundleLock, nil, nil
	}
	imagesLock, err := NewImagesLockFromPath(path)
	if err == nil {
		return nil, &imagesLock, nil
	}
	return nil, nil, fmt.Errorf("Trying to read bundle or images lock file: %s", err)
}

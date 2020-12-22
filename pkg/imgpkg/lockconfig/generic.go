// Copyright 2020 VMware, Inc.
// SPDX-License-Identifier: Apache-2.0

package lockconfig

type LockVersion struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
}

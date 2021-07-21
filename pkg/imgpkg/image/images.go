// Copyright 2020 VMware, Inc.
// SPDX-License-Identifier: Apache-2.0

package image

import (
	regname "github.com/google/go-containerregistry/pkg/name"
	regv1 "github.com/google/go-containerregistry/pkg/v1"
	regremote "github.com/google/go-containerregistry/pkg/v1/remote"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . ImagesMetadata
type ImagesMetadata interface {
	Get(regname.Reference) (*regremote.Descriptor, error)
	Head(regname.Reference) (*regv1.Descriptor, error)
	Digest(regname.Reference) (regv1.Hash, error)
	Index(regname.Reference) (regv1.ImageIndex, error)
	Image(regname.Reference) (regv1.Image, error)
	FirstImageExists(digests []string) (string, error)
}

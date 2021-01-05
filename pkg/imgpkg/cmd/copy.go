// Copyright 2020 VMware, Inc.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/cppforlife/go-cli-ui/ui"
	regname "github.com/google/go-containerregistry/pkg/name"
	ctlimg "github.com/k14s/imgpkg/pkg/imgpkg/image"
	ctlimgset "github.com/k14s/imgpkg/pkg/imgpkg/imageset"
	lf "github.com/k14s/imgpkg/pkg/imgpkg/lockfiles"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

type CopyOptions struct {
	ui ui.UI

	ImageFlags      ImageFlags
	BundleFlags     BundleFlags
	LockInputFlags  LockInputFlags
	LockOutputFlags LockOutputFlags
	TarFlags        TarFlags
	RegistryFlags   RegistryFlags

	RepoDst     string
	Concurrency int
}

func NewCopyOptions(ui ui.UI) *CopyOptions {
	return &CopyOptions{ui: ui}
}

func NewCopyCmd(o *CopyOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "copy",
		Short: "Copy a bundle from one location to another",
		RunE:  func(_ *cobra.Command, _ []string) error { return o.Run() },
		Example: `
    # Copy bundle dkalinin/app1-bundle to local tarball at /Volumes/app1-bundle.tar
    imgpkg copy -b dkalinin/app1-bundle --to-tar /Volumes/app1-bundle.tar

    # Copy bundle dkalinin/app1-bundle to another registry (or repository)
    imgpkg copy -b dkalinin/app1-bundle --to-repo internal-registry/app1-bundle

    # Copy image dkalinin/app1-image to another registry (or repository)
    imgpkg copy -i dkalinin/app1-image --to-repo internal-registry/app1-image`,
	}

	o.ImageFlags.SetCopy(cmd)
	o.BundleFlags.SetCopy(cmd)
	o.LockInputFlags.Set(cmd)
	o.LockOutputFlags.Set(cmd)
	o.TarFlags.Set(cmd)
	o.RegistryFlags.Set(cmd)
	cmd.Flags().StringVar(&o.RepoDst, "to-repo", "", "Location to upload assets")
	cmd.Flags().IntVar(&o.Concurrency, "concurrency", 5, "Concurrency")
	return cmd
}

func (o *CopyOptions) Run() error {
	if !o.hasOneSrc() {
		return fmt.Errorf("Expected either --lock, --bundle (-b), --image (-i), or --from-tar as a source")
	}
	if !o.hasOneDest() {
		return fmt.Errorf("Expected either --to-tar or --to-repo")
	}

	logger := ctlimg.NewLogger(os.Stderr)
	prefixedLogger := logger.NewPrefixedWriter("copy | ")

	registry, err := ctlimg.NewRegistry(o.RegistryFlags.AsRegistryOpts())
	if err != nil {
		return fmt.Errorf("Unable to create a registry with the options %v: %v", o.RegistryFlags.AsRegistryOpts(), err)
	}

	imageSet := ctlimgset.NewImageSet(o.Concurrency, prefixedLogger)

	var bundleURL string
	var processedImages *ctlimgset.ProcessedImages
	switch {
	case o.isTarSrc():
		if o.isTarDst() {
			return fmt.Errorf("Cannot use tar source (--from-tar) with tar destination (--to-tar)")
		}

		importRepo, err := regname.NewRepository(o.RepoDst)
		if err != nil {
			return fmt.Errorf("Building import repository ref: %s", err)
		}
		tarImageSet := ctlimgset.NewTarImageSet(imageSet, o.Concurrency, prefixedLogger)

		processedImages, bundleURL, err = tarImageSet.Import(o.TarFlags.TarSrc, importRepo, registry)
		if err != nil {
			return err
		}

	case o.isRepoSrc():
		copy2 := Copy2{
			ImageFlags:     o.ImageFlags,
			BundleFlags:    o.BundleFlags,
			LockInputFlags: o.LockInputFlags,
		}

		unprocessedImageUrls, bundleURL2, err := copy2.Foo(registry)
		if err != nil {
			return err
		}

		bundleURL = bundleURL2 // TODO

		if o.isTarDst() {
			if o.LockOutputFlags.LockFilePath != "" {
				return fmt.Errorf("cannot output lock file with tar destination")
			}

			tarImageSet := ctlimgset.NewTarImageSet(imageSet, o.Concurrency, prefixedLogger)

			err = tarImageSet.Export(unprocessedImageUrls, o.TarFlags.TarDst, registry) // download to tar
			if err != nil {
				return err
			}
		}
		if o.isRepoDst() {
			importRepo, err := regname.NewRepository(o.RepoDst)
			if err != nil {
				return fmt.Errorf("Building import repository ref: %s", err)
			}

			processedImages, err = imageSet.Relocate(unprocessedImageUrls, importRepo, registry)
			if err != nil {
				return err
			}
		}
	}

	if o.LockOutputFlags.LockFilePath != "" {
		return o.writeLockOutput(processedImages, bundleURL)
	}

	return nil
}

func (o *CopyOptions) isTarSrc() bool {
	return o.TarFlags.TarSrc != ""
}

func (o *CopyOptions) isRepoSrc() bool {
	return o.ImageFlags.Image != "" || o.BundleFlags.Bundle != "" || o.LockInputFlags.LockFilePath != ""
}

func (o *CopyOptions) isTarDst() bool {
	return o.TarFlags.TarDst != ""
}

func (o *CopyOptions) isRepoDst() bool {
	return o.RepoDst != ""
}

func (o *CopyOptions) hasOneDest() bool {
	repoSet := o.isRepoDst()
	tarSet := o.isTarDst()
	return (repoSet || tarSet) && !(repoSet && tarSet)
}

func (o *CopyOptions) hasOneSrc() bool {
	var seen bool
	for _, ref := range []string{o.LockInputFlags.LockFilePath, o.TarFlags.TarSrc,
		o.BundleFlags.Bundle, o.ImageFlags.Image} {
		if ref != "" {
			if seen {
				return false
			}
			seen = true
		}
	}
	return seen
}

func (o *CopyOptions) writeLockOutput(processedImages *ctlimgset.ProcessedImages, bundleURL string) error {
	var outBytes []byte
	var err error

	switch bundleURL {
	case "":
		iLock := lf.ImageLock{ApiVersion: lf.ImagesLockAPIVersion, Kind: lf.ImagesLockKind}
		for _, img := range processedImages.All() {
			iLock.Spec.Images = append(
				iLock.Spec.Images,
				lf.ImageDesc{
					Image: img.Image.URL,
				},
			)
		}

		outBytes, err = yaml.Marshal(iLock)
		if err != nil {
			return err
		}
	default:
		var originalTag, url string
		for _, img := range processedImages.All() {
			if img.UnprocessedImageURL.URL == bundleURL {
				originalTag = img.UnprocessedImageURL.Tag
				url = img.Image.URL
			}
		}

		if url == "" {
			return fmt.Errorf("could not find process item for url '%s'", bundleURL)
		}

		bLock := lf.BundleLock{
			ApiVersion: lf.BundleLockAPIVersion,
			Kind:       lf.BundleLockKind,
			Spec:       lf.BundleSpec{Image: lf.ImageLocation{DigestRef: url, OriginalTag: originalTag}},
		}
		outBytes, err = yaml.Marshal(bLock)
		if err != nil {
			return err
		}

	}

	return ioutil.WriteFile(o.LockOutputFlags.LockFilePath, outBytes, 0700)
}

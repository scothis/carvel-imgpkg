package cmd

import (
	"fmt"
	"os"

	"github.com/cppforlife/go-cli-ui/ui"
	regname "github.com/google/go-containerregistry/pkg/name"
	ctlimg "github.com/k14s/imgpkg/pkg/imgpkg/image"

	"github.com/spf13/cobra"
)

type CopyOptions struct {
	ui ui.UI

	RegistryFlags RegistryFlags
	Concurrency   int

	BundleLockSrc string
	TarSrc        string
	BundleSrc     string
	ImageLockSrc  string
	ImageSrc      string

	RepoDst string
	TarDst  string
}

func NewCopyOptions(ui ui.UI) *CopyOptions {
	return &CopyOptions{ui: ui}
}

func NewCopyCmd(o *CopyOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "copy",
		Short:   "Copy a bundle from one location to another",
		RunE:    func(_ *cobra.Command, _ []string) error { return o.Run() },
		Example: ``,
	}

	// TODO switch to using shared flags and collapse --images-lock into --lock
	cmd.Flags().StringVar(&o.BundleLockSrc, "lock", "", "BundleLock of the bundle to relocate")
	cmd.Flags().StringVarP(&o.BundleSrc, "bundle", "b", "", "BundleLock of the bundle to relocate")
	cmd.Flags().StringVarP(&o.ImageSrc, "image", "i", "", "BundleLock of the bundle to relocate")
	cmd.Flags().StringVar(&o.ImageLockSrc, "images-lock", "", "BundleLock of the bundle to relocate")
	cmd.Flags().StringVar(&o.RepoDst, "to-repo", "", "BundleLock of the bundle to relocate")
	cmd.Flags().StringVar(&o.TarDst, "to-tar", "", "BundleLock of the bundle to relocate")
	cmd.Flags().IntVar(&o.Concurrency, "concurrency", 5, "concurrency")
	return cmd
}

func (o *CopyOptions) Run() error {
	if !o.HasOneSrc() {
		return fmt.Errorf("Expected either --lock, --bundle (-b), --image (-i), or --tar as a source")
	}

	if !o.HasOneDest() {
		return fmt.Errorf("Expected either --tar or --repo destination")
	}

	logger := ctlimg.NewLogger(os.Stderr)
	prefixedLogger := logger.NewPrefixedWriter("copy | ")

	importRepo, err := regname.NewRepository(o.RepoDst)
	if err != nil {
		return fmt.Errorf("Building import repository ref: %s", err)
	}

	registry := ctlimg.NewRegistry(o.RegistryFlags.AsRegistryOpts())
	if err != nil {
		return err
	}

	imageSet := ImageSet{o.Concurrency, prefixedLogger}

	if o.TarSrc != "" {
		imageSet := TarImageSet{imageSet, o.Concurrency, prefixedLogger}
		_, err = imageSet.Import(o.TarSrc, importRepo, registry)
	} else {
		var unprocessedImageUrls *UnprocessedImageURLs
		unprocessedImageUrls, err = o.GetUnprocessedImageURLs()
		if err != nil {
			return err
		}

		if o.TarDst != "" {
			tarImageSet := TarImageSet{imageSet, o.Concurrency, prefixedLogger}
			err = tarImageSet.Export(unprocessedImageUrls, o.TarDst, registry) // download to tar
		} else {
			_, err = imageSet.Relocate(unprocessedImageUrls, importRepo, registry)
		}
	}

	return err
}

func (o *CopyOptions) HasOneDest() bool {
	repoSet := o.RepoDst != ""
	tarSet := o.TarDst != ""
	fmt.Printf("%v, %v", repoSet, tarSet)
	return (repoSet || tarSet) && !(repoSet && tarSet)
}

func (o *CopyOptions) HasOneSrc() bool {
	var seen bool
	for _, ref := range []string{o.BundleLockSrc, o.TarSrc, o.BundleSrc, o.ImageSrc, o.ImageLockSrc} {
		if ref != "" {
			if seen {
				return false
			}
			seen = true
		}
	}
	return seen
}

func (o *CopyOptions) GetUnprocessedImageURLs() (*UnprocessedImageURLs, error) {
	unprocessedImageURLs := NewUnprocessedImageURLs()

	switch {
	case o.ImageSrc != "":
		isBundle, err := isBundle(o.ImageSrc, o.RegistryFlags.AsRegistryOpts())
		if err != nil {
			return nil, err
		}

		if isBundle {
			return nil, fmt.Errorf("Expected image, got bundle")
		}

		unprocessedImageURLs.Add(UnprocessedImageURL{o.ImageSrc})

	case o.ImageLockSrc != "":
		imgLock, err := ReadImageLockFile(o.ImageLockSrc)
		if err != nil {
			return nil, err
		}

		for _, img := range imgLock.Spec.Images {
			unprocessedImageURLs.Add(UnprocessedImageURL{img.DigestRef})
		}

	default:
		bundleRef := o.BundleSrc
		if o.BundleLockSrc != "" {
			lockFile, err := ReadBundleLockFile(o.BundleLockSrc)
			if err != nil {
				return nil, err
			}
			bundleRef = lockFile.Spec.Image.DigestRef
		}

		isBundle, err := isBundle(bundleRef, o.RegistryFlags.AsRegistryOpts())
		if err != nil {
			return nil, err
		}

		if !isBundle {
			return nil, fmt.Errorf("Expected a bundle, got an image")
		}

		imageRefs, err := GetReferencedImages(bundleRef, o.RegistryFlags.AsRegistryOpts())
		if err != nil {
			return nil, err
		}

		for _, imgRef := range imageRefs {
			unprocessedImageURLs.Add(UnprocessedImageURL{imgRef})
		}
		unprocessedImageURLs.Add(UnprocessedImageURL{bundleRef})
	}

	return unprocessedImageURLs, nil
}

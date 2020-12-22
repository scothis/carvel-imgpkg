package plainimage

import (
	"fmt"

	"github.com/cppforlife/go-cli-ui/ui"
	regname "github.com/google/go-containerregistry/pkg/name"
	ctlimg "github.com/k14s/imgpkg/pkg/imgpkg/image"
	lf "github.com/k14s/imgpkg/pkg/imgpkg/lockfiles" // TODO remove
)

type PlainImage struct {
	ref      string
	registry ctlimg.Registry
}

func NewPlainImage(ref string, registry ctlimg.Registry) PlainImage {
	return PlainImage{ref, registry}
}

func (i PlainImage) Pull(outputPath string, ui ui.UI) error {
	ref, err := regname.ParseReference(i.ref, regname.WeakValidation)
	if err != nil {
		return err
	}

	imgs, err := ctlimg.NewImages(ref, i.registry).Images()
	if err != nil {
		return fmt.Errorf("Collecting images: %s", err)
	}

	if len(imgs) == 0 {
		return fmt.Errorf("Expected to find at least one image, but found none")
	}

	if len(imgs) > 1 {
		ui.BeginLinef("Found multiple images, extracting first\n")
	}

	img := imgs[0]
	isBundle, err := lf.IsBundle(img)
	if err != nil {
		return fmt.Errorf("Checking if image is bundle: %v", err)
	}
	if isBundle {
		return fmt.Errorf("Expected bundle flag when pulling a bundle, please use -b instead of --image")
	}

	digest, err := img.Digest()
	if err != nil {
		return fmt.Errorf("Getting image digest: %s", err)
	}

	ui.BeginLinef("Pulling image '%s@%s'\n", ref.Context(), digest)

	err = ctlimg.NewDirImage(outputPath, img, ui).AsDirectory()
	if err != nil {
		return fmt.Errorf("Extracting image into directory: %s", err)
	}

	return nil
}

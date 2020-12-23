package bundle

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cppforlife/go-cli-ui/ui"
	regname "github.com/google/go-containerregistry/pkg/name"
	ctlimg "github.com/k14s/imgpkg/pkg/imgpkg/image"
	"github.com/k14s/imgpkg/pkg/imgpkg/lockconfig"
	lf "github.com/k14s/imgpkg/pkg/imgpkg/lockfiles"
)

const BundleConfigLabel = "dev.carvel.imgpkg.bundle"

type Bundle struct {
	ref      string
	registry ctlimg.Registry
}

func NewBundle(ref string, registry ctlimg.Registry) Bundle {
	return Bundle{ref, registry}
}

func (o Bundle) Pull(outputPath string, ui ui.UI) error {
	ref, err := regname.ParseReference(o.ref, regname.WeakValidation)
	if err != nil {
		return err
	}

	imgs, err := ctlimg.NewImages(ref, o.registry).Images()
	if err != nil {
		return fmt.Errorf("Collecting images: %s", err)
	}

	if len(imgs) != 1 {
		return fmt.Errorf("Expected to find exactly one image, but found zero or more than one")
	}

	img := imgs[0]

	// TODO how to check if something is a bundle?
	isBundle, err := lf.IsBundle(img)
	if err != nil {
		return fmt.Errorf("Checking if image is bundle: %s", err)
	}
	if !isBundle {
		// TODO wrong abstraction level for err msg hint
		return fmt.Errorf("Expected bundle image but found plain image (hint: Did you use -i instead of -b?)")
	}

	digest, err := img.Digest()
	if err != nil {
		return fmt.Errorf("Getting bundle digest: %s", err)
	}

	ui.BeginLinef("Pulling bundle '%s@%s'\n", ref.Context(), digest)

	err = ctlimg.NewDirImage(outputPath, img, ui).AsDirectory()
	if err != nil {
		return fmt.Errorf("Extracting bundle into directory: %s", err)
	}

	err = o.rewriteImagesLock(outputPath, ref, ui)
	if err != nil {
		return fmt.Errorf("Rewriting image lock file: %s", err)
	}

	return nil
}

func (o Bundle) rewriteImagesLock(outputPath string, ref regname.Reference, ui ui.UI) error {
	path := filepath.Join(outputPath, lf.BundleDir, lf.ImageLockFile)

	lockFile, err := lockconfig.NewImagesLockFromPath(path)
	if err != nil {
		return err
	}

	ui.BeginLinef("Locating image lock file images...\n")

	bundleRepo := ref.Context().Name()
	numAlreadyInBundleRepo := 0

	var imageRefs []lockconfig.ImageRef

	for _, imgRef := range lockFile.Images {
		imageInBundleRepo, err := ImageWithRepository(imgRef.Image, bundleRepo)
		if err != nil {
			return err
		}
		// TODO do we really need this feature?
		if imgRef.Image == imageInBundleRepo {
			numAlreadyInBundleRepo += 1
		}

		foundImg, err := o.checkImagesExist([]string{imageInBundleRepo, imgRef.Image})
		if err != nil {
			return err
		}
		if foundImg != imageInBundleRepo {
			ui.BeginLinef("One or more images not found in bundle repo; skipping lock file update\n")
			return nil
		}

		imageRefs = append(imageRefs, lockconfig.ImageRef{
			Image:       foundImg,
			Annotations: imgRef.Annotations,
		})
	}

	if numAlreadyInBundleRepo == len(lockFile.Images) {
		return nil
	}

	lockFile.Images = imageRefs

	ui.BeginLinef("All images found in bundle repo; updating lock file: %s\n", path)

	return lockFile.WriteToPath(path)
}

func (o Bundle) checkImagesExist(urls []string) (string, error) {
	var err error
	for _, img := range urls {
		ref, parseErr := regname.NewDigest(img)
		if parseErr != nil {
			return "", parseErr
		}
		_, err = o.registry.Generic(ref)
		if err == nil {
			return img, nil
		}
	}
	return "", fmt.Errorf("Checking image existance: %s", err)
}

func ImageWithRepository(img string, repo string) (string, error) {
	parts := strings.Split(img, "@")
	if len(parts) != 2 {
		return "", fmt.Errorf("Parsing image URL: %s", img)
	}
	digest := parts[1]

	newURL := repo + "@" + digest
	return newURL, nil
}

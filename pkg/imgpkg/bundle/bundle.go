package bundle

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/cppforlife/go-cli-ui/ui"
	regname "github.com/google/go-containerregistry/pkg/name"
	regv1 "github.com/google/go-containerregistry/pkg/v1"
	ctlimg "github.com/k14s/imgpkg/pkg/imgpkg/image"
	"github.com/k14s/imgpkg/pkg/imgpkg/lockconfig"
	lf "github.com/k14s/imgpkg/pkg/imgpkg/lockfiles"
)

const (
	BundleConfigLabel = "dev.carvel.imgpkg.bundle"
)

type Bundle struct {
	ref      string
	registry ctlimg.Registry

	parsedRef    regname.Reference
	parsedDigest string
}

func NewBundle(ref string, registry ctlimg.Registry) *Bundle {
	return &Bundle{ref: ref, registry: registry}
}

func (o *Bundle) DigestRef() (string, error) {
	if len(o.parsedDigest) == 0 {
		panic("Unexpected usage of DigestRef(); call Pull/ImagesLockLocalized before")
	}
	return ImageWithRepository(o.ref, o.parsedDigest)
}

func (o *Bundle) Tag() (string, error) {
	if o.parsedRef == nil {
		panic("Unexpected usage of Tag(); call Pull/ImagesLockLocalized before")
	}
	if tagRef, ok := o.parsedRef.(regname.Tag); ok {
		return tagRef.TagStr(), nil
	}
	return "", nil
}

func (o *Bundle) Pull(outputPath string, ui ui.UI) error {
	img, err := o.pull()
	if err != nil {
		return err
	}

	digestRef, err := o.DigestRef()
	if err != nil {
		return err
	}

	ui.BeginLinef("Pulling bundle '%s'\n", digestRef)

	err = ctlimg.NewDirImage(outputPath, img, ui).AsDirectory()
	if err != nil {
		return fmt.Errorf("Extracting bundle into directory: %s", err)
	}

	err = o.rewriteImagesLockFile(outputPath, ui)
	if err != nil {
		return fmt.Errorf("Rewriting image lock file: %s", err)
	}

	return nil
}

func (o *Bundle) pull() (regv1.Image, error) {
	var err error

	o.parsedRef, err = regname.ParseReference(o.ref, regname.WeakValidation)
	if err != nil {
		return nil, err
	}

	imgs, err := ctlimg.NewImages(o.parsedRef, o.registry).Images()
	if err != nil {
		return nil, fmt.Errorf("Collecting images: %s", err)
	}

	if len(imgs) != 1 {
		return nil, fmt.Errorf("Expected to find exactly one image, but found zero or more than one")
	}

	img := imgs[0]

	// TODO how to check if something is a bundle?
	isBundle, err := lf.IsBundle(img)
	if err != nil {
		return nil, fmt.Errorf("Checking if image is bundle: %s", err)
	}
	if !isBundle {
		// TODO wrong abstraction level for err msg hint
		return nil, fmt.Errorf("Expected bundle image but found plain image (hint: Did you use -i instead of -b?)")
	}

	digest, err := img.Digest()
	if err != nil {
		return nil, fmt.Errorf("Getting bundle digest: %s", err)
	}

	o.parsedDigest = digest.String()

	return img, nil
}

func (o *Bundle) rewriteImagesLockFile(outputPath string, ui ui.UI) error {
	path := filepath.Join(outputPath, lf.BundleDir, lf.ImageLockFile)

	imagesLock, err := lockconfig.NewImagesLockFromPath(path)
	if err != nil {
		return err
	}

	ui.BeginLinef("Locating image lock file images...\n")

	skipped, err := o.localizeImagesLock(&imagesLock)
	if err != nil {
		return err
	}
	if skipped {
		ui.BeginLinef("One or more images not found in bundle repo; skipping lock file update\n")
		return nil
	}

	ui.BeginLinef("All images found in bundle repo; updating lock file: %s\n", path)

	return imagesLock.WriteToPath(path)
}

func (o *Bundle) localizeImagesLock(imagesLock *lockconfig.ImagesLock) (bool, error) {
	if o.parsedRef == nil {
		panic("Internal inconsistency: parsedRef is not set")
	}

	bundleRepo := o.parsedRef.Context().Name()
	numAlreadyInBundleRepo := 0

	var imageRefs []lockconfig.ImageRef

	for _, imgRef := range imagesLock.Images {
		imageInBundleRepo, err := ImageWithRepository(imgRef.Image, bundleRepo)
		if err != nil {
			return false, err
		}
		// TODO do we really need this feature?
		if imgRef.Image == imageInBundleRepo {
			numAlreadyInBundleRepo += 1
		}

		foundImg, err := o.checkImagesExist([]string{imageInBundleRepo, imgRef.Image})
		if err != nil {
			return false, err
		}
		if foundImg != imageInBundleRepo {
			return true, nil
		}

		imageRefs = append(imageRefs, lockconfig.ImageRef{
			Image:       foundImg,
			Annotations: imgRef.Annotations,
		})
	}

	if numAlreadyInBundleRepo == len(imagesLock.Images) {
		return false, nil
	}

	imagesLock.Images = imageRefs
	return false, nil
}

func (o *Bundle) checkImagesExist(urls []string) (string, error) {
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

// Get image reference with digest
func getRefWithDigest(parsedRef regname.Reference, img regv1.Image) (regname.Reference, error) {
	digest, err := img.Digest()
	if err != nil {
		return nil, err
	}
	refWithDigest, err := regname.NewDigest(fmt.Sprintf("%s@%s", parsedRef.Context().Name(), digest))
	if err != nil {
		return nil, err
	}
	return refWithDigest, err
}

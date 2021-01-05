package cmd

import (
	regname "github.com/google/go-containerregistry/pkg/name"
	regv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/k14s/imgpkg/pkg/imgpkg/bundle"
	"github.com/k14s/imgpkg/pkg/imgpkg/image"
	ctlimg "github.com/k14s/imgpkg/pkg/imgpkg/image"
	ctlimgset "github.com/k14s/imgpkg/pkg/imgpkg/imageset"
	"github.com/k14s/imgpkg/pkg/imgpkg/lockconfig"
	lf "github.com/k14s/imgpkg/pkg/imgpkg/lockfiles"
)

type Copy2 struct {
	ImageFlags     ImageFlags
	BundleFlags    BundleFlags
	LockInputFlags LockInputFlags
}

func (o Copy2) Foo(registry ctlimg.Registry) (*ctlimgset.UnprocessedImageURLs, string, error) {
	unprocessedImageUrls, bundleURL, err := o.getUnprocessedImageURLs(registry)
	if err != nil {
		return nil, "", err
	}

	return unprocessedImageUrls, bundleURL, nil
}

func (o Copy2) getUnprocessedImageURLs(reg ctlimg.Registry) (*ctlimgset.UnprocessedImageURLs, string, error) {
	unprocessedImageURLs := ctlimgset.NewUnprocessedImageURLs()

	switch {
	case o.LockInputFlags.LockFilePath != "":
		bundleLock, imagesLock, err := lockconfig.NewLockFromPath(o.LockInputFlags.LockFilePath)
		if err != nil {
			return nil, "", err
		}

		switch {
		case bundleLock != nil:
			bundle := bundle.NewBundle(bundleLock.Bundle.Image, reg)

			imagesLock, err := bundle.ImagesLockLocalized()
			if err != nil {
				return nil, "", err
			}

			for _, img := range imagesLock.Images {
				unprocessedImageURLs.Add(ctlimgset.UnprocessedImageURL{URL: img.Image})
			}

			unprocessedImageURLs.Add(ctlimgset.UnprocessedImageURL{
				URL: bundleLock.Bundle.Image,
				Tag: bundleLock.Bundle.Tag,
			})

			return unprocessedImageURLs, bundleLock.Bundle.Image, nil

		case imagesLock != nil:
			// TODO imagesLock
			// bundles, err := imgLock.CheckForBundles(reg)
			// if err != nil {
			// 	return nil, "", fmt.Errorf("Checking image lock for bundles: %s", err)
			// }

			// if len(bundles) != 0 {
			// 	return nil, "", fmt.Errorf("Expected image lock to not contain bundle reference: '%v'", strings.Join(bundles, "', '"))
			// }

			// for _, img := range imgLock.Spec.Images {
			// 	unprocessedImageURLs.Add(ctlimgset.UnprocessedImageURL{URL: img.Image})
			// }
			// return unprocessedImageURLs, "", nil

		default:
			panic("Unreachable")
		}

	case o.ImageFlags.Image != "":
		// parsedRef, img, err := getRefAndImage(o.ImageFlags.Image, &reg)
		// if err != nil {
		// 	return nil, "", err
		// }

		// if err := checkIfBundle(img, false, fmt.Errorf("Expected bundle flag when copying a bundle, please use -b instead of -i")); err != nil {
		// 	return nil, "", err
		// }

		// imageTag := getTag(parsedRef)
		// unprocessedImageURLs.Add(ctlimgset.UnprocessedImageURL{o.ImageFlags.Image, imageTag})
		// return unprocessedImageURLs, "", nil

	default:
		bundle := bundle.NewBundle(o.BundleFlags.Bundle, reg)

		// TODO switch to using fallback URLs for each image
		// instead of trying to use localized bundle URLs here
		imagesLock, err := bundle.ImagesLockLocalized()
		if err != nil {
			return nil, "", err
		}

		for _, img := range imagesLock.Images {
			unprocessedImageURLs.Add(ctlimgset.UnprocessedImageURL{URL: img.Image})
		}

		digestURL, err := bundle.DigestRef()
		if err != nil {
			return nil, "", err
		}

		tag, err := bundle.Tag()
		if err != nil {
			return nil, "", err
		}

		unprocessedImageURLs.Add(ctlimgset.UnprocessedImageURL{URL: digestURL, Tag: tag})

		return unprocessedImageURLs, digestURL, nil
	}

	panic("Unreachable")
}

// Get the parsed image reference and associated image struct from a registry
func getRefAndImage(ref string, reg *image.Registry) (regname.Reference, regv1.Image, error) {
	parsedRef, err := regname.ParseReference(ref)
	if err != nil {
		return nil, nil, err
	}

	img, err := reg.Image(parsedRef)
	if err != nil {
		return nil, nil, err
	}

	return parsedRef, img, err
}

// Get the tag from an image reference. Returns empty string
// if no tag found.
func getTag(parsedRef regname.Reference) string {
	var tag string
	if t, ok := parsedRef.(regname.Tag); ok {
		tag = t.TagStr()
	}
	return tag
}

// Determine whether an image is a Bundle or is not a Bundle
func checkIfBundle(img regv1.Image, expectsBundle bool, errMsg error) error {
	isBundle, err := lf.IsBundle(img)
	if err != nil {
		return err
	}
	// bundleCheck lets function caller determine whether to err
	// on if img is a Bundle or is not
	if isBundle != expectsBundle {
		// errMsg is custom err message if isBundle != expectsBundle
		// that caller can specify
		return errMsg
	}

	return nil
}

package cmd

import (
	"fmt"
	"strings"

	regname "github.com/google/go-containerregistry/pkg/name"
	regv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/k14s/imgpkg/pkg/imgpkg/bundle"
	"github.com/k14s/imgpkg/pkg/imgpkg/image"
	ctlimg "github.com/k14s/imgpkg/pkg/imgpkg/image"
	ctlimgset "github.com/k14s/imgpkg/pkg/imgpkg/imageset"
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

	if bundleURL != "" {
		unprocessedImageUrls, err = checkBundleRepoForCollocatedImages(unprocessedImageUrls, bundleURL, registry)
		if err != nil {
			return nil, "", err
		}
	}

	return unprocessedImageUrls, bundleURL, nil
}

func (o Copy2) getUnprocessedImageURLs(reg ctlimg.Registry) (*ctlimgset.UnprocessedImageURLs, string, error) {
	unprocessedImageURLs := ctlimgset.NewUnprocessedImageURLs()
	var bundleRef string

	switch {
	case o.LockInputFlags.LockFilePath != "":
		lock, err := lf.ReadLockFile(o.LockInputFlags.LockFilePath)
		if err != nil {
			return nil, "", err
		}
		switch {
		case lock.Kind == lf.BundleLockKind:
			bundleLock, err := lf.ReadBundleLockFile(o.LockInputFlags.LockFilePath)
			if err != nil {
				return nil, "", err
			}

			bundleRef = bundleLock.Spec.Image.DigestRef
			parsedRef, img, err := getRefAndImage(bundleRef, &reg)
			if err != nil {
				return nil, "", err
			}

			if err := checkIfBundle(img, true, fmt.Errorf("Expected image flag when given an image reference. Please run with -i instead of -b, or use -b with a bundle reference")); err != nil {
				return nil, "", err
			}

			images, err := lf.GetReferencedImages(parsedRef, reg)
			if err != nil {
				return nil, "", err
			}

			bundle := lf.Bundle{bundleRef, bundleLock.Spec.Image.OriginalTag, img}
			collectURLs(images, &bundle, unprocessedImageURLs)

		case lock.Kind == lf.ImagesLockKind:
			imgLock, err := lf.ReadImageLockFile(o.LockInputFlags.LockFilePath)
			if err != nil {
				return nil, "", err
			}

			bundles, err := imgLock.CheckForBundles(reg)
			if err != nil {
				return nil, "", fmt.Errorf("Checking image lock for bundles: %s", err)
			}

			if len(bundles) != 0 {
				return nil, "", fmt.Errorf("Expected image lock to not contain bundle reference: '%v'", strings.Join(bundles, "', '"))
			}

			collectURLs(imgLock.Spec.Images, nil, unprocessedImageURLs)
		default:
			return nil, "", fmt.Errorf("Unexpected lock kind. Expected BundleLock or ImagesLock, got: %v", lock.Kind)
		}

	case o.ImageFlags.Image != "":
		parsedRef, img, err := getRefAndImage(o.ImageFlags.Image, &reg)
		if err != nil {
			return nil, "", err
		}

		if err := checkIfBundle(img, false, fmt.Errorf("Expected bundle flag when copying a bundle, please use -b instead of -i")); err != nil {
			return nil, "", err
		}

		imageTag := getTag(parsedRef)
		unprocessedImageURLs.Add(ctlimgset.UnprocessedImageURL{o.ImageFlags.Image, imageTag})

	default:
		bundleRef = o.BundleFlags.Bundle
		parsedRef, img, err := getRefAndImage(bundleRef, &reg)
		if err != nil {
			return nil, "", err
		}

		bundleTag := getTag(parsedRef)
		refWithDigest, err := getRefWithDigest(parsedRef, img)
		if err != nil {
			return nil, "", err
		}

		if err := checkIfBundle(img, true, fmt.Errorf("Expected image flag when given an image reference. Please run with -i instead of -b, or use -b with a bundle reference")); err != nil {
			return nil, "", err
		}

		images, err := lf.GetReferencedImages(refWithDigest, reg)
		if err != nil {
			return nil, "", err
		}

		bundle := lf.Bundle{bundleRef, bundleTag, img}
		collectURLs(images, &bundle, unprocessedImageURLs)
	}

	return unprocessedImageURLs, bundleRef, nil
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

// And images and bundle reference to unprocessedImageURLs.
// Exclude passing Bundle reference by passing nil.
func collectURLs(images []lf.ImageDesc, bundle *lf.Bundle, unprocessedImageURLs *ctlimgset.UnprocessedImageURLs) {
	for _, img := range images {
		unprocessedImageURLs.Add(ctlimgset.UnprocessedImageURL{URL: img.Image})
	}
	if bundle != nil {
		unprocessedImageURLs.Add(ctlimgset.UnprocessedImageURL{URL: bundle.URL, Tag: bundle.Tag})
	}
}

func checkBundleRepoForCollocatedImages(foundImages *ctlimgset.UnprocessedImageURLs, bundleURL string, registry ctlimg.Registry) (*ctlimgset.UnprocessedImageURLs, error) {
	checkedURLs := ctlimgset.NewUnprocessedImageURLs()
	bundleRef, err := regname.ParseReference(bundleURL)
	if err != nil {
		return nil, err
	}
	bundleRepo := bundleRef.Context().Name()

	for _, img := range foundImages.All() {
		if img.URL == bundleURL {
			checkedURLs.Add(img)
			continue
		}

		newURL, err := bundle.ImageWithRepository(img.URL, bundleRepo)
		if err != nil {
			return nil, err
		}
		ref, err := regname.NewDigest(newURL, regname.StrictValidation)
		if err != nil {
			return nil, err
		}

		_, err = registry.Generic(ref)
		if err == nil {
			checkedURLs.Add(ctlimgset.UnprocessedImageURL{newURL, img.Tag})
		} else {
			checkedURLs.Add(img)
		}
	}

	return checkedURLs, nil
}

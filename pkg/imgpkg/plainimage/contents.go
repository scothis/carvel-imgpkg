package plainimage

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cppforlife/go-cli-ui/ui"
	regname "github.com/google/go-containerregistry/pkg/name"
	ctlimg "github.com/k14s/imgpkg/pkg/imgpkg/image"
	lf "github.com/k14s/imgpkg/pkg/imgpkg/lockfiles"
)

type Contents struct {
	paths         []string
	excludedPaths []string
}

func NewContents(paths []string, excludedPaths []string) Contents {
	return Contents{paths: paths, excludedPaths: excludedPaths}
}

func (b Contents) Push(uploadRef regname.Tag, registry ctlimg.Registry, ui ui.UI) (string, error) {
	err := b.validate()
	if err != nil {
		return "", err
	}

	tarImg := ctlimg.NewTarImage(b.paths, b.excludedPaths, InfoLog{ui})

	img, err := tarImg.AsFileImage()
	if err != nil {
		return "", err
	}

	defer img.Remove()

	err = registry.WriteImage(uploadRef, img)
	if err != nil {
		return "", fmt.Errorf("Writing '%s': %s", uploadRef.Name(), err)
	}

	digest, err := img.Digest()
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s@%s", uploadRef.Context(), digest), nil
}

func (b Contents) validate() error {
	imgpkgDirs, err := b.findImgpkgDirs()
	if err != nil {
		return nil
	}

	if len(imgpkgDirs) > 0 {
		return fmt.Errorf("Images cannot be pushed with '%s' directories (found %d at '%s'), consider using a bundle",
			lf.BundleDir, len(imgpkgDirs), strings.Join(imgpkgDirs, ","))
	}

	return b.checkRepeatedPaths()
}

func (b *Contents) findImgpkgDirs() ([]string, error) {
	var bundlePaths []string
	for _, path := range b.paths {
		err := filepath.Walk(path, func(currPath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if filepath.Base(currPath) != lf.BundleDir {
				return nil
			}

			currPath, err = filepath.Abs(currPath)
			if err != nil {
				return err
			}

			bundlePaths = append(bundlePaths, currPath)

			return nil
		})

		if err != nil {
			return []string{}, err
		}
	}

	return bundlePaths, nil
}

func (b Contents) checkRepeatedPaths() error {
	imageRootPaths := make(map[string][]string)
	for _, flagPath := range b.paths {
		err := filepath.Walk(flagPath, func(currPath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			imageRootPath, err := filepath.Rel(flagPath, currPath)
			if err != nil {
				return err
			}

			if imageRootPath == "." {
				if info.IsDir() {
					return nil
				}
				imageRootPath = filepath.Base(flagPath)
			}
			imageRootPaths[imageRootPath] = append(imageRootPaths[imageRootPath], currPath)
			return nil
		})

		if err != nil {
			return err
		}
	}

	var repeatedPaths []string
	for _, v := range imageRootPaths {
		if len(v) > 1 {
			repeatedPaths = append(repeatedPaths, v...)
		}
	}
	if len(repeatedPaths) > 0 {
		return fmt.Errorf("Found duplicate paths: %s", strings.Join(repeatedPaths, ", "))
	}
	return nil
}

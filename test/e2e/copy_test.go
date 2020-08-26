package e2e

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// Add a test later for non colocated referenced images to test that we
// fall back on original url when not present in bundle repo
func TestCopyBundleToRepoWithColocatedReferencedImages(t *testing.T) {
	env := BuildEnv(t)
	imgpkg := Imgpkg{t, Logger{}, env.ImgpkgPath}

	// create generic image
	assetsPath := filepath.Join("assets", "simple-app")

	// force digest to change so test is meaningful
	randFile := filepath.Join(assetsPath, "rand.yml")
	defer os.Remove(randFile)
	randContents := fmt.Sprintf("%d", time.Now().UnixNano())
	err := ioutil.WriteFile(randFile, []byte(randContents), 0700)
	if err != nil {
		t.Fatalf("failed to write random file: %v", err)
	}

	out := imgpkg.Run([]string{"push", "--tty", "-i", env.Image, "-f", assetsPath})
	genericDigestTag := fmt.Sprintf("@%s", extractDigest(out, t))
	genericDigestRef := env.Image + genericDigestTag

	imgsYml := fmt.Sprintf(`---
apiVersion: pkgx.k14s.io/v1alpha1
kind: ImagesLock
spec:
  images:
  - name: image
    url: %s
`, genericDigestRef)

	// Create a bundle with ref to generic
	imgpkgDir, err := createBundleDir(assetsPath, bundleYAML, imgsYml)
	if err != nil {
		t.Fatalf("failed to create bundle dir: %v", err)
	}
	defer os.RemoveAll(imgpkgDir)

	out = imgpkg.Run([]string{"push", "--tty", "-b", env.Image, "-f", assetsPath})
	bundleDigestTag := fmt.Sprintf("@%s", extractDigest(out, t))
	bundleDigestRef := env.Image + bundleDigestTag

	// copy via created ref
	imgpkg.Run([]string{"copy", "--bundle", bundleDigestRef, "--to-repo", env.RelocationRepo})

	ref, _ := name.ParseReference(env.RelocationRepo + bundleDigestTag)
	if _, err = remote.Image(ref); err != nil {
		t.Fatalf("validating image presence in relocation repo: %v", err)
	}

	ref, _ = name.ParseReference(env.RelocationRepo + genericDigestTag)
	if _, err = remote.Image(ref); err != nil {
		t.Fatalf("validating image presence in relocation repo: %v", err)
	}
}

func TestCopyBundleLockInputToRepo(t *testing.T) {
	t.Skip("skipping until flags are added")
	env := BuildEnv(t)
	imgpkg := Imgpkg{t, Logger{}, env.ImgpkgPath}

	// general setup
	testDir := filepath.Join(os.TempDir(), "imgpkg-test-pull-image-error")
	lockFile := filepath.Join(testDir, "bundle.lock.yml")
	defer os.RemoveAll(testDir)

	// create generic image

	assetsPath := filepath.Join("assets", "simple-app")
	bundleDir, err := createBundleDir(assetsPath, bundleYAML, imagesYAML)
	defer os.RemoveAll(bundleDir)
	if err != nil {
		t.Fatalf("Creating bundle directory: %s", err.Error())
	}

	// Create bundle that refs generic with --lock-ouput

	// copy via output file
	imgpkg.Run([]string{"copy", "--lock", lockFile, "--to-repo", env.RelocationRepo})

	// check if present in dst repo

	// check if referenced images is present
}

func TestCopyImageInputToRepo(t *testing.T) {
	t.Skip("skipping until flags are added")
	env := BuildEnv(t)
	imgpkg := Imgpkg{t, Logger{}, env.ImgpkgPath}
	// create image

	// copy via create ref
	imgpkg.Run([]string{"copy", "--image", env.Image, "--to-repo", env.RelocationRepo})

	// check present in dst
}

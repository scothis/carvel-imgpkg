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

func TestCopyBundleLockInputToRepo(t *testing.T) {
	env := BuildEnv(t)
	imgpkg := Imgpkg{t, Logger{}, env.ImgpkgPath}

	// general setup
	testDir := filepath.Join(os.TempDir(), "imgpkg-test-pull-image-error")
	lockFile := filepath.Join(testDir, "bundle.lock.yml")
	err := os.MkdirAll(testDir, 0700)
	if err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	defer os.RemoveAll(testDir)

	// create generic image

	assetsPath := filepath.Join("assets", "simple-app")

	randFile, err := addRandomFile(assetsPath)
	if err != nil {
		t.Fatalf("failed to create unuique file: %v", err)
	}
	defer os.Remove(randFile)

	out := imgpkg.Run([]string{"push", "--tty", "-i", env.Image, "-f", assetsPath})
	genericDigestTag := fmt.Sprintf("@%s", extractDigest(out, t))
	genericDigestRef := env.Image + genericDigestTag

	imgsYml := fmt.Sprintf(`---
apiVersion: imgpkg.k14s.io/v1alpha1
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

	// Create bundle that refs generic with --lock-ouput
	imgpkg.Run([]string{"push", "-b", env.Image, "-f", assetsPath, "--lock-output", lockFile})

	// copy via output file
	imgpkg.Run([]string{"copy", "--lock", lockFile, "--to-repo", env.RelocationRepo})

	// check if present in dst repo
	// check if referenced images is present
	refs := []string{env.RelocationRepo + genericDigestTag}
	if err := validateImagePresence(refs); err != nil {
		t.Fatalf("could not validate image presence: %v", err)
	}

}

func TestCopyImageLockInputToRepo(t *testing.T) {
	env := BuildEnv(t)
	imgpkg := Imgpkg{t, Logger{}, env.ImgpkgPath}

	// general setup
	testDir := filepath.Join(os.TempDir(), "imgpkg-test-pull-image-error")
	lockFile := filepath.Join(testDir, "images.lock.yml")
	err := os.MkdirAll(testDir, 0700)
	if err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	defer os.RemoveAll(testDir)

	// create generic image
	assetsPath := filepath.Join("assets", "simple-app")

	// force digest to change so test is meaningful
	randFile, err := addRandomFile(assetsPath)
	if err != nil {
		t.Fatalf("failed to create unuique file: %v", err)
	}
	defer os.Remove(randFile)

	out := imgpkg.Run([]string{"push", "--tty", "-i", env.Image, "-f", assetsPath})
	genericDigestTag := fmt.Sprintf("@%s", extractDigest(out, t))
	genericDigestRef := env.Image + genericDigestTag

	imgsYml := fmt.Sprintf(`---
apiVersion: imgpkg.k14s.io/v1alpha1
kind: ImagesLock
spec:
  images:
  - name: image
    url: %s
`, genericDigestRef)

	err = ioutil.WriteFile(lockFile, []byte(imgsYml), 0700)
	if err != nil {
		t.Fatalf("failed to create images.lock file: %v", err)
	}

	// Create bundle that refs generic with --lock-ouput

	// copy via output file
	imgpkg.Run([]string{"copy", "--lock", lockFile, "--to-repo", env.RelocationRepo})

	// check if present in dst repo
	// check if referenced images is present
	refs := []string{env.RelocationRepo + genericDigestTag}
	if err := validateImagePresence(refs); err != nil {
		t.Fatalf("could not validate image presence: %v", err)
	}
}

func TestCopyBundleWithCollocatedReferencedImagesToRepo(t *testing.T) {
	env := BuildEnv(t)
	imgpkg := Imgpkg{t, Logger{}, env.ImgpkgPath}

	// create generic image
	assetsPath := filepath.Join("assets", "simple-app")

	// force digest to change so test is meaningful
	randFile, err := addRandomFile(assetsPath)
	if err != nil {
		t.Fatalf("failed to create unuique file: %v", err)
	}
	defer os.Remove(randFile)

	out := imgpkg.Run([]string{"push", "--tty", "-i", env.Image, "-f", assetsPath})
	genericDigestTag := fmt.Sprintf("@%s", extractDigest(out, t))

	imgsYml := fmt.Sprintf(`---
apiVersion: imgpkg.k14s.io/v1alpha1
kind: ImagesLock
spec:
  images:
  - name: image
    url: index.docker.io/k8slt/does-not-exist%s
`, genericDigestTag)

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

	refs := []string{env.RelocationRepo + genericDigestTag, env.RelocationRepo + bundleDigestTag}
	if err := validateImagePresence(refs); err != nil {
		t.Fatalf("could not validate image presence: %v", err)
	}
}

func TestCopyBundleWithNonCollocatedReferencedImagesToRepo(t *testing.T) {
	t.Skip("skipping")
	env := BuildEnv(t)
	imgpkg := Imgpkg{t, Logger{}, env.ImgpkgPath}

	// create generic image
	assetsPath := filepath.Join("assets", "simple-app")

	// force digest to change so test is meaningful
	randFile, err := addRandomFile(assetsPath)
	if err != nil {
		t.Fatalf("failed to create unuique file: %v", err)
	}
	defer os.Remove(randFile)

	out := imgpkg.Run([]string{"push", "--tty", "-i", env.Image, "-f", assetsPath})
	genericDigestTag := fmt.Sprintf("@%s", extractDigest(out, t))
	//fallback url for image, image does not exist in this repo, but we will use that our advantage
	genericDigestRef := "index.docker.io/k8slt/test" + genericDigestTag

	imgsYml := fmt.Sprintf(`---
apiVersion: imgpkg.k14s.io/v1alpha1
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

	refs := []string{env.RelocationRepo + genericDigestTag, env.RelocationRepo + bundleDigestTag}
	if err := validateImagePresence(refs); err != nil {
		t.Fatalf("could not validate image presence: %v", err)
	}
}

func TestCopyImageInputToRepo(t *testing.T) {
	env := BuildEnv(t)
	imgpkg := Imgpkg{t, Logger{}, env.ImgpkgPath}

	// create generic image
	assetsPath := filepath.Join("assets", "simple-app")

	// force digest to change so test is meaningful
	randFile, err := addRandomFile(assetsPath)
	if err != nil {
		t.Fatalf("failed to create unuique file: %v", err)
	}
	defer os.Remove(randFile)

	out := imgpkg.Run([]string{"push", "--tty", "-i", env.Image, "-f", assetsPath})
	imageDigestTag := fmt.Sprintf("@%s", extractDigest(out, t))
	imageDigestRef := env.Image + imageDigestTag

	// copy via create ref
	imgpkg.Run([]string{"copy", "--image", imageDigestRef, "--to-repo", env.RelocationRepo})

	// check present in dst
	refs := []string{env.RelocationRepo + imageDigestTag}
	if err := validateImagePresence(refs); err != nil {
		t.Fatalf("could not validate image presence: %v", err)
	}
}

func addRandomFile(dir string) (string, error) {
	randFile := filepath.Join(dir, "rand.yml")
	randContents := fmt.Sprintf("%d", time.Now().UnixNano())
	err := ioutil.WriteFile(randFile, []byte(randContents), 0700)
	if err != nil {
		return "", err
	}

	return randFile, nil
}

func validateImagePresence(refs []string) error {
	for _, refString := range refs {
		ref, _ := name.ParseReference(refString)
		if _, err := remote.Image(ref); err != nil {
			return fmt.Errorf("validating image %s: %v", refString, err)
		}
	}
	return nil
}

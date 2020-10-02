package cmd

import (
	"fmt"
	"strings"

	regname "github.com/google/go-containerregistry/pkg/name"
	regv1 "github.com/google/go-containerregistry/pkg/v1"
	regtypes "github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/k14s/imgpkg/pkg/imgpkg/image"
	ctlimg "github.com/k14s/imgpkg/pkg/imgpkg/image"
)

// overall idea:
//	Copy
//	L two interfaces -- source & dest one returns list of Reference structs, one takes list of Reference structs
//  Sources
//		L Bundle struct will implement source
//		L LockSource will implement source
//	  L ImageSource will implement source
//		L Tar implements source
//  Dest
//		L Tar implements dst
//		L Repo/Registry implements Dest

// RegistryArtifact abstracts away the differences between an image and an index
// In most of the code, we don't care about the difference, so we can
// instead leave it generic and then in the sources and sinks cast
// to image or index or any other artifact we support (e.g. Bundles)
type RegistryArtifact interface {
	// MediaType of this artifact.
	MediaType() (regtypes.MediaType, error)

	// Digest returns the digest of the artifact.
	Digest() (regv1.Hash, error)
}

// References acts as an abstratcion of a registry
// artifact. It stores the constituents of a reference
// as separate fields to allow easy alterations.
type Reference struct {
	repo     string
	digest   string
	tag      string
	name     string
	registry ctlimg.Registry
	opts     []regname.Option
	artifact RegistryArtifact
}

const defaultTag string = "latest"

// NewRNewReference will create a new reference given a name, ref, tag and opts
// See tag cases for break down of tag logic
// must resolve reference on creation to avoid WithTag().Artifact() not being a thing
func NewReference(ref, tag, name string, registry ctlimg.Registry, opts ...regname.Option) (Reference, error) {
	parsedRef, err := regname.ParseReference(ref)
	if err != nil {
		return Reference{}, err
	}

	// Cases: tag != "" + tag ref => error or choose tag
	//        tag != "" + digest ref => use tag arg as tag
	//        tag == "" + tag ref => use tag from tag ref + resolve the digest
	//                    L with explicit :x tag (x can be "latest") => use that tag
	//                    L without explicit tag => leave empty
	//        tag == "" + digest ref => tag is "" + set digest
	if tagRef, ok := parsedRef.(regname.Tag); ok {
		if tag != "" {
			return Reference{}, fmt.Errorf("cannot create reference with tag ref and tag")
		}

		if tagStr := tagRef.TagStr(); tagStr != defaultTag || strings.Contains(ref, defaultTag) {
			tag = tagStr
		}
	}

	// should be generic registry call instead of typed
	artifact, err := registry.Image(parsedRef)
	if err != nil {
		return Reference{}, fmt.Errorf("getting artifact: %v", err)
	}

	digest, err := artifact.Digest()
	if err != nil {
		return Reference{}, fmt.Errorf("getting digest: %v", err)
	}

	return Reference{
		repo:     parsedRef.Context().Name(),
		digest:   digest.String(),
		tag:      tag,
		name:     name,
		registry: registry,
		opts:     opts,
		artifact: artifact,
	}, nil
}

// Artifact returns the registry artifact associated with this ref at creation
func (ref *Reference) Artifact() RegistryArtifact {
	return ref.artifact
}

// WithRepo returns a new reference with newRepo as its repository string
func (ref *Reference) WithRepo(newRepo string) Reference {
	// we may want to do a deep copy?
	return Reference{
		repo:     newRepo,
		digest:   ref.digest,
		tag:      ref.tag,
		name:     ref.name,
		registry: ref.registry,
		opts:     ref.opts,
		artifact: ref.artifact,
	}
}

// WithRepo returns a new reference with tag as its tag
func (ref *Reference) WithTag(tag string) Reference {
	// we may want to do a deep copy?
	return Reference{
		repo:     ref.repo,
		digest:   ref.digest,
		tag:      tag,
		name:     ref.name,
		registry: ref.registry,
		opts:     ref.opts,
		artifact: ref.artifact,
	}
}

// AsTag returned this reference as a parsed tag reference
func (ref *Reference) AsTag(opts ...regname.Option) (regname.Tag, error) {
	finalOpts := append(ref.opts, opts...)
	return regname.NewTag(fmt.Sprintf("%s:%s", ref.repo, ref.tag), finalOpts...)
}

// AsDigest returns this reference as a parsed digest reference
func (ref *Reference) AsDigest(opts ...regname.Option) (regname.Digest, error) {
	finalOpts := append(ref.opts, opts...)
	return regname.NewDigest(fmt.Sprintf("%s@%s", ref.repo, ref.digest), finalOpts...)
}

// IsBundle checks to see if the current artifact pointed to by this reference
// is a bundle
func (ref *Reference) IsBundle() (bool, error) {
	if img, ok := ref.artifact.(regv1.Image); ok {
		manifest, err := img.Manifest()
		if err != nil {
			return false, fmt.Errorf("getting manifest: %v", err)
		}

		if _, present := manifest.Annotations[image.BundleAnnotation]; present {
			return true, nil
		}
	}

	return false, nil
}

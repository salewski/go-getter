package getter

import (
	"fmt"

	"github.com/hashicorp/go-getter/helper/url"
)

// CtxDetector (read: "Contextual Detector"), like its Detector predecessor,
// defines an interface that an invalid URL or a URL with a blank scheme can
// be passed through in order to determine if its shorthand for something else
// well-known.
//
// In addition to the capabilities provided by Detector, a CtxDetector allows
// the caller to provide more information about the context in which it is
// being invoked, which allows for some types of useful detections and
// transformations that were not previously possible.
//
type CtxDetector interface {
	// CtxDetect will detect whether the string matches a known pattern to
	// turn it into a proper URL
	//
	// FIXME: Providing 'ctxSubDir' in order make all context data available
	//        to CtxDetector implementation(s). Does this help avoid blind
	//        spots in the CtxDetector's? Or does it just add unnecessary
	//        complexity?
	//
	// The 'ctxSubDir' value, if non-empty, will be the '//some/subdir' value
	// already parsed out of 'src' (as by SourceDirSubdir(...)). It is
	// provided to the CtxDetector impl. only for contextual awareness, which
	// conceivably could inform its decision-making process. It should not be
	// incorporated into the result returned by the CtxDetector impl.
	//
	// Protocol: Where they need to be resolved, relative filepath values in
	//           'src' will be resolved relative to 'pwd', unless
	//           'srcResolveFrom' is non-empty; then they will be resolved
	//           relative to 'srcResolveFrom'.
	//
	//           Note that some CtxDetector impls. (FileCtxDetector,
	//           GitCtxDetector) can only produce meaningful results in some
	//           circumstances if they have an absolute directory to resolve
	//           to. For best results, when 'srcResolveFrom' is non-empty,
	//           provide an absolute filepath.
	//
	//           The CtxDetect interface itself does not require that either
	//           'pwd' or 'srcResolveFrom' be absolute filepaths, but that
	//           might be required by a particular CtxDetector implementation.
	//           Know that RFC-compliant use of 'file://' URIs (which some
	//           CtxDetector impls. emit) permit only absolute filepaths, and
	//           tools (such as Git) expect this. Providing relative filepaths
	//           for 'pwd' and/or 'srcResolveFrom' may result in the
	//           generation of non-legit 'file://' URIs with relative paths in
	//           them, and CtxDetector implementations are permitted to reject
	//           them with an error if it requires an absolute path.
	//
	CtxDetect(src, pwd, forceToken, ctxSubDir, srcResolveFrom string) (string, bool, error)
}

// ContextualDetectors is the list of detectors that are tried on an invalid URL.
// This is also the order they're tried (index 0 is first).
var ContextualDetectors []CtxDetector

func init() {
	ContextualDetectors = []CtxDetector{
		// new(GitHubCtxDetector),
		new(GitCtxDetector),
		// new(BitBucketCtxDetector),
		// new(S3CtxDetector),
		// new(GCSCtxDetector),
		// new(FileCtxDetector),
	}
}

// CtxDetect turns a source string into another source string if it is
// detected to be of a known pattern.
//
// An empty-string value provided for 'pwd' is interpretted as "not
// provided". Likewise for 'srcResolveFrom'.
//
// The (optional) 'srcResolveFrom' parameter allows the caller to provide a
// directory from which any reletive filepath in 'src' should be resolved,
// instead of relative to 'pwd'. This supports those use cases (e.g.,
// Terraform modules with relative 'source' filepaths) where the caller
// context for path resolution may be different than the pwd. For best result,
// the provided value should be an absolute filepath. If unneeded, use specify
// the empty string.
//
// The 'cds' []CtxDetector parameter should be the list of detectors to use in
// the order to try them. If you don't want to configure this, just use the
// global ContextualDetectors variable.
//
// This is safe to be called with an already valid source string: Detect
// will just return it.
//
func CtxDetect(src, pwd, srcResolveFrom string, cds []CtxDetector) (string, error) {
	//
	// Design note: We considered accepting *string rather than string for
	//              'pwd' and 'srcResolveFrom' params, as that would give us a
	//              better way to distinguish between "not provided" and
	//              "provided, but empty". We avoided doing so, however, for
	//              two reasons:
	//
	//              1. Because we are providing an evolutionary step away from
	//                 Detect(...), we want to make it as easy as possible to
	//                 migrate existing code. It is presumably easier for
	//                 current users of the Detect API to migrate to this one
	//                 by adding an extra string param (for 'srcResolveFrom')
	//                 than to change the types of the param they are passing
	//                 for 'pwd' at all call sites.
	//
	//              2. In real-world use cases of this lib, having an empty
	//                 string as the value against which to resolve a filepath
	//                 would (probably) be non-sensible on all OS's. That is,
	//                 we are safe in our context interpretting the empty
	//                 string as meaning "not provided".

	getForce, getSrc := getForcedGetter(src)

	// Separate out the subdir if there is one, we don't pass that to detect
	getSrc, subDir := SourceDirSubdir(getSrc)

	u, err := url.Parse(getSrc)
	if err == nil && u.Scheme != "" {
		// Valid URL
		return src, nil
	}

	for _, d := range cds {
		result, ok, err := d.CtxDetect(getSrc, pwd, getForce, subDir, srcResolveFrom)
		if err != nil {
			return "", err
		}
		if !ok {
			continue
		}

		result, err = handleDetected(result, getForce, subDir)
		if err != nil {
			return "", err
		}

		return result, nil
	}

	return "", fmt.Errorf("invalid source string: %s", src)
}

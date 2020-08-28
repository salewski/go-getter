package getter

import (
	"fmt"
	"path/filepath"

	"github.com/hashicorp/go-getter/helper/url"
)

// Detector defines the interface that an invalid URL or a URL with a blank
// scheme is passed through in order to determine if its shorthand for
// something else well-known.
type Detector interface {
	// Detect will detect whether the string matches a known pattern to
	// turn it into a proper URL.
	Detect(string, string) (string, bool, error)
}

// Detectors is the list of detectors that are tried on an invalid URL.
// This is also the order they're tried (index 0 is first).
var Detectors []Detector

func init() {
	Detectors = []Detector{
		new(GitHubDetector),
		new(GitDetector),
		new(BitBucketDetector),
		new(S3Detector),
		new(GCSDetector),
		new(FileDetector),
	}
}

// Detect turns a source string into another source string if it is
// detected to be of a known pattern.
//
// The third parameter should be the list of detectors to use in the
// order to try them. If you don't want to configure this, just use
// the global Detectors variable.
//
// This is safe to be called with an already valid source string: Detect
// will just return it.
func Detect(src string, pwd string, ds []Detector) (string, error) {
	getForce, getSrc := getForcedGetter(src)

	// Separate out the subdir if there is one, we don't pass that to detect
	getSrc, subDir := SourceDirSubdir(getSrc)

	u, err := url.Parse(getSrc)
	if err == nil && u.Scheme != "" {
		// Valid URL
		return src, nil
	}

	// Special case for when the 'git::' forcing token is used with a filepath
	// (which may be either absolute or relative). If relative, it MUST begin
	// with './' or '../', or the Windows equivalent).
	//
	// This needs to be done here because our Detector interface does not
	// provide a way for us to communicate the forcing token value to the
	// implementation; GitDetector.Detect(...) cannot assume any filepath it
	// sees (in its 'src' param) as intended for Git without also knowing that
	// the forcing token was specified. The forcing token explicitly tells us
	// that the filepath is intended to be interpreted as the file system path
	// to a Git repository.
	//
	if getForce == "git" {
		rslt, ok, err := detectGitForceFilepath(getSrc, pwd, getForce)
		if err != nil {
			return "", err
		}
		if ok {
			// 'git::' forced on a filepath was detected
			rslt, err = handleDetected(rslt, getForce, subDir)
			if err != nil {
				return "", err
			}

			return rslt, nil
		}
	}

	for _, d := range ds {
		result, ok, err := d.Detect(getSrc, pwd)
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

func handleDetected(detectedResult, srcGetForce, subDir string) (string, error) {
	var detectForce string
	detectForce, result := getForcedGetter(detectedResult)
	result, detectSubdir := SourceDirSubdir(result)

	// If we have a subdir from the detection, then prepend it to our
	// requested subdir.
	if detectSubdir != "" {
		if subDir != "" {
			subDir = filepath.Join(detectSubdir, subDir)
		} else {
			subDir = detectSubdir
		}
	}

	if subDir != "" {
		u, err := url.Parse(result)
		if err != nil {
			return "", fmt.Errorf("Error parsing URL: %s", err)
		}
		u.Path += "//" + subDir

		// a subdir may contain wildcards, but in order to support them we
		// have to ensure the path isn't escaped.
		u.RawPath = u.Path

		result = u.String()
	}

	// Preserve the forced getter if it exists. We try to use the
	// original set force first, followed by any force set by the
	// detector.
	if srcGetForce != "" {
		result = fmt.Sprintf("%s::%s", srcGetForce, result)
	} else if detectForce != "" {
		result = fmt.Sprintf("%s::%s", detectForce, result)
	}

	return result, nil
}

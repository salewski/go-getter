package getter

import (
	"fmt"
	// "log"
	"net/url"
	// "os"
	"path/filepath"
	"strings"
)

// GitCtxDetector implements CtxDetector to detect Git SSH URLs such as
// git@host.com:dir1/dir2 and converts them to proper URLs.
type GitCtxDetector struct{}

func (d *GitCtxDetector) CtxDetect(src, pwd, forceToken, _, srcResolveFrom string) (string, bool, error) {

	// If the 'git::' force token was specified, then our work here is more
	// than just "best effort"; we must complain if we were not able to detect
	// how to parse a src string that was explicitly flagged for us to handle.
	//
	mustField := "git" == forceToken

	if len(src) == 0 {
		if mustField {
			return "", false, fmt.Errorf("forced 'git::' handling: src string must be non-empty")
		}
		return "", false, nil
	}

	if len(forceToken) > 0 {
		rslt, ok, err := detectGitForceFilepath(src, pwd, forceToken, srcResolveFrom)
		if err != nil {
			return "", false, err
		}
		if ok {
			return rslt, true, nil
		}
	}

	u, err := detectSSH(src)
	if err != nil {
		return "", false, err
	}
	if u == nil {
		if mustField {
			return "", false, fmt.Errorf("forced 'git::' handling: ssh detection yielded nil URL for src: %s", src)
		}
		return "", false, nil
	}

	// We require the username to be "git" to assume that this is a Git URL
	if u.User.Username() != "git" {
		if mustField {
			return "", false, fmt.Errorf("forced 'git::' handling: ssh username is not 'git'; got: %s; src is: %s", u.User.Username(), src)
		}
		return "", false, nil
	}

	return "git::" + u.String(), true, nil
}

// detectGitForceFilepath() allows the 'git::' force token to be used on a
// filepath to a Git repository. Both absolute and relative filepaths are
// supported.
//
// When in-effect, the returned string will contain:
//
//     "git::file://<abspath>"
// OR:
//     "git::file://<abspath>?<query_params>"
//
// where <abspath> is the provided src param expanded to an absolute filepath
// (if it wasn't already). If the provided src param is a relative filepath,
// then the expanded absolute file path is based on the current working
// directory of the process.
//
// Note that detectGitForceFilepath() only handles filepaths that have been
// explicitly forced (via the 'git::' force token) for Git processing. The
// caller must still utilize the Detect() interface for other bits of
// Git-specific detection.
//
//
// Q: Why isn't this funcationality in our Detect() implementation?
//
// A: It is only safe to do when the 'git::' force token is provided, and the
//    Detector interface does not currently support a mechanism for the caller
//    to supply that context. Consequently, the GitDetector implementation
//    cannot assume that a filepath is intended for Git processing (that is,
//    as representing a Git repository). Enter this function.
//
//    As a special case for when the 'git::' forcing key was provided, the
//    caller can communicate that info to this function in a manner similar to
//    the Detect() method, but with the additional context of the force
//    key. That makes it safe for us to treat a filepath as knowlingly
//    intended for Git.
//
//
// Q: Why is the returned value embedded in a 'file://' URI?
//
// A. When specifying the 'git::' force token on a filepath, the intention is
//    to indicate that it is a path to a Git repository that can be cloned,
//    etc. Though Git will accept both relative and absolute file paths for
//    this purpose, we unlock more functionality by using a 'file://' URI. In
//    particular, that allows our GitGetter to use any provided query params
//    to specify a specific tag or commit hash, for example.
//
//
// Q: Why is a relative path expanded to an absolute path in the returned
//    'file://' URI?
//
// A: Relative paths are technically not "legal" in RFC 1738- and RFC 8089-
//    compliant 'file://' URIs, and they are not accepted by Git. When
//    generating a 'file://' URI as we're doing here, using the absolute path
//    is the only useful thing to do.
//
//
// Q: Why support this functionality at all? Why not just require that a
//    source location use an absolute path in a 'file://' URI explicitly if
//    that's what is needed?
//
// A: The primary reason is to allow support for relative filepaths to Git
//    repos. There are use cases in which the absolute path cannot be known in
//    advance, but a relative path to a Git repo is known. For example, when a
//    Terraform project (or any Git-based project) uses Git submodules, it
//    will know the relative location of the Git submodule repos, but cannot
//    know the absolute path in advance because it will vary based on where
//    the "superproject" repo is cloned. Nevertheless, those relative paths
//    should be usable as clonable Git repos, and this mechanism allows for
//    that. Support for filepaths that are already absolute is provided mainly
//    for symmetry.
//
func detectGitForceFilepath(src, pwd, force, srcResolveFrom string) (string, bool, error) {

	// FIXME: Tweaked interface requires that in order for this function to
	//        have an effect, our 'src' param start with './', '../', '/', or
	//        their Windows equivalents. That is, the value must be clearly
	//        identifiable as a filepath reference without requiring that we
	//        actually touch the filesystem itself.
	//
	//        Values such as 'foo', or 'foo/bar', which may or may not be
	//        filepaths, are ignored (this function will not have an effect on
	//        them).
	//
	//        Also, neither 'pwd' nor 'srcResolveFrom' is /required/ to be an
	//        absolute file path. As long as 'src' is determined to be a
	//        filepath, this function will resolve it relative to 'pwd' by
	//        default, or 'srcResolveFrom' if it is non-empty.
	//
	//        Yes, this means that our returned string value may produce
	//        invalid 'file://' URIs (that is, we DO NOT guarantee that the
	//        pathname in the 'file://' URI produced here is an absolute
	//        pathname. This is The Right Thing To Do, though, if the original
	//        source string looke something like:
	//
	//            "git::../../some/value"
	//        or:
	//            "git::/some/value"
	//
	//        This is doing what the caller explicitly requested, even if it
	//        yield results that seem non-sensical here at the time of
	//        writing. As a library, our emphasis is on providing "mechanism"
	//        over "policy".
	//
	//        However, when producing a value that we strongly suspect may be
	//        other than what is truly wanted, we /can/ emit a
	//        warning. However, our library does not currently have a
	//        well-defined means to support that in a controlled way. We take
	//        a "better than nothing" approach by emitting a warning via
	//        golang's "standard logger" (which, by default, writes to stderr)
	//        if the user has set a 'GO_GETTER_DEBUG' environment variable to
	//        any truthy-looking value ("1", "true", "yes", etc., but not "",
	//        "0", "false", or "no").
	//
	//        Document that the filepath in our return string is (mostly)
	//        Clean, in the filepath.Clean() sense. That means:
	//
	//            1. Even when 'src' was provided with an absolute filepath
	//               value, the emitted cleaned value may be different.
	//
	//            2. Anything that looks like a go-getter "subdir" value
	//               ('//some/subdir') in 'src' will not be distinguishable as
	//               such in our result string (because '//' would get cleaned
	//               to just '/'). This should not be a problem in practice,
	//               though as the CtxDetect() dispatch function removes such
	//               values from 'src' prior to invoking the 'CtxDetect()'
	//               method of the CtxDetector(s); this function should only
	//               see such values in 'src' when running under our unit
	//               tests.

	// debugging := isTruthy(os.Getenv("GO_GETTER_DEBUG"))

	// The full force key token is 'git::', but the Detect() dispatcher
	// function provides our CtxDetect() method with the parsed value (without
	// the trailing colons).
	//
	if force != "git" {
		return "", false, nil
	}

	if len(src) == 0 {
		return "", false, nil
	}

	var srcResolved string

	if filepath.IsAbs(src) {
		srcResolved = src
	} else {

		// A relative filepath MUST begin with './' or '../', or the Windows
		// equivalent.
		//
		if !isLocalSourceAddr(src) {
			// src is neither an absolute nor relative filepath (at least not
			// obviously so), so we'll treat it as "not for us".
			return "", false, nil
		}

		// Recall that the result of filepath.Join() is Cleaned
		if len(srcResolveFrom) > 0 {

			if !filepath.IsAbs(srcResolveFrom) {
				return "", false, fmt.Errorf("unable to resolve 'git::' forced filepath (%s)" +
					"; provided srcResolveFrom filepath (%s) is not rooted", src, srcResolveFrom)
			}

			srcResolved = filepath.Join(srcResolveFrom, src)

		} else if len(pwd) > 0 {

			if !filepath.IsAbs(pwd) {
				return "", false, fmt.Errorf("unable to resolve 'git::' forced filepath (%s)" +
					"; provided pwd filepath (%s) is not rooted", pwd, srcResolveFrom)
			}

			srcResolved = filepath.Join(pwd, src)
		} else {
			// // There's no way to resolve a more complete filepath for 'src'
			// // other than to do so relative to the current working directory
			// // of the process (which go-getter won't do, by design). So we're
			// // going to just use the relative filepath "as is".
			// //
			// // A different approach would be to avoid processing the value at
			// // all, and just return !ok here. The rationale for processing the
			// // provided value anyway is that we are only in here because 'src'
			// // had the form:
			// //
			// //     git::./some/relative/path
			// //
			// // It would be even more broken for the Git filepath detector
			// // (that's us) to /not/ process such a value.
			// if debugging {
			// 	// The best we can do is maybe complain a bit...
			// 	//
			// 	log.Printf("[WARN] go-getter GitCtxDetector neither pwd nor srcResolveFrom provided; using relative filepath \"as is\": %s", src);
			// }
			// srcResolved = src

			return "", false, fmt.Errorf("unable to resolve 'git::' force filepath (%s)" +
				"; neither pwd nor srcResolveFrom param was provided", src)
		}
	}

	// // For consistent (and maximally sane) output, we run the resolved
	// // filepath through filepath.Clean(). But we need a little bit of our own
	// // mojo sprinkled on top to avoid accidentally presenting a relative
	// // filepaths as an absolute one. Consequently, the cleaned value we obtain
	// // here might be ever-so-slightly "less clean" than what filepath.Clean()
	// // would produce on its own.
	// //
	// // CAREFUL: We must clean the path prior to ensuring it starts with a '/'
	// //          character (see below). Otherwise the clean operation would
	// //          change a string that starts with '/./foo' to just '/foo'. That
	// //          would change the semantic meaning in an unintended way.
	// //
	// srcResolvedClean := cleanPreservingRelativePrefix(srcResolved)

	srcResolvedClean := filepath.Clean(srcResolved)

	// // Since 'file://' URIs with non-abosolute filepath values are a) not
	// // RFC-compliant and b) unlikely to work in most scenarios, we'll warn
	// // about the situation (if we were asked to).
	// //
	// if debugging && !filepath.IsAbs(srcResolvedClean) {
	// 	log.Printf("[WARN] go-getter GitCtxDetector resolved non-absolute filepath: %s", srcResolvedClean);
	// }

	// To make the filepath usable (hopefully) in a 'file://' URI, we may need
	// to flip Windows-style '\' to URI-style '/'.
	//
	// Note that filepath.ToSlash is effectively a NOOP on platforms where '/'
	// is the os.Separator; when running on Unix-like platforms, this WILL NOT
	// hose your Unix filepaths that just happen to have backslash characters
	// in them.
	//
	srcResolvedClean = filepath.ToSlash(srcResolvedClean)

	if !strings.HasPrefix(srcResolvedClean, "/") {

		// An absolute file path on Unix will start with a '/', but that is
		// not true for all OS's. RFC 8089 makes the authority component
		// (including the '//') optional in a 'file:' URL, but git (at least
		// as of version 2.28.0) only recognizes the 'file://' form. In fact,
		// the git-clone(1) manpage is explicit that it wants the syntax to
		// be:
		//
		//     file:///path/to/repo.git/
		//
		// Some notes on the relevant RFCs:
		//
		// RFC 1738 (section 3.10, "FILES") documents a <host> and <path>
		// portion being separated by a '/' character:
		//
		//     file://<host>/<path>
		//
		// RFC 2396 (Appendix G.2, "Modifications from both RFC 1738 and RFC
		// 1808") refines the above by declaring that the '/' is actually part
		// of the path. It is still required to separate the "authority
		// portion" of the URI from the path portion, but is not a separate
		// component of the URI syntax.
		//
		// RFC 3986 (Section 3.2, "Authority") states that the authority
		// component of a URI "is terminated by the next slash ("/"), question
		// mark ("?"), or number sign ("#") character, or by the end of the
		// URI." However, for 'file:' URIs, only those terminated by a '/'
		// characters are supported by Git (as noted above).
		//
		// RFC 8089 (Appendix A, "Differences from Previous Specifications")
		// references the RFC 1738 form including the required '/' after the
		// <host>/authority component.
		//
		// Because it is the most compatible approach across the only
		// partially compatible RFC recommendations, and (more importantly)
		// because it is what Git requires for 'file:' URIs, we require that
		// our absolute path value start with a '/' character.
		//
		// Note that even on Unix-like systems we need to do this if the
		// parameters provided did not resolve to an absolute path. In that
		// case, the filepath we provide in the URI will not be absolute (so
		// in that respect will be bogus), but the filepath will be correctly
		// deliminated in the URI from the <host>/authority component.
		//
		srcResolvedClean = "/" + srcResolvedClean
	}

	// We know our 'srcResolvedClean' value starts with '/', and may start
	// with '/.' for some types of relative paths. It may also have URI query
	// parameters (e.g., "ref=v1.2.3") and the path elements may have
	// characters that would need to be escaped in a proper URI. We'll
	// leverage url.Parse() to deal with all of that, and then down below
	// the stringified version of it will be properly encoded.
	//
	// u, err := url.Parse(srcResolvedClean)  // note: no URI "scheme" (okay)
	u, err := url.Parse("file://" + srcResolvedClean)
	if err != nil {
		return "", false, fmt.Errorf("error parsing 'git::' force filepath (%s) to URL: %s", srcResolvedClean, err)
	}

// FIXME: cleanup (tighten-up)
	uSerialized := u.String();

	// rtn := fmt.Sprintf("%s::file://%s", force, srcResolvedClean)
	// rtn := fmt.Sprintf("%s::file://%s", force, uSerialized)
	rtn := fmt.Sprintf("%s::%s", force, uSerialized)

	return rtn, true, nil
}

// Borrowed from terraform/internal/initwd/getter.go
// (modified here to accept "." and "..", too, if exact, full matches)
var localSourcePrefixes = []string{
	"./",
	"../",
	".\\",
	"..\\",
}
var localExactMatches = []string{
	`.`,
	`..`,
}

func isLocalSourceAddr(addr string) bool {
	for _, value := range localExactMatches {
		if value == addr {
			return true
		}
	}
	for _, prefix := range localSourcePrefixes {
		if strings.HasPrefix(addr, prefix) {
			return true
		}
	}
	return false
}

// // isTruthy is a predicate function that returns true if its string parameter
// // represents a value that can be interpretted as a boolean true value.
// //
// // Any value that is not considered falsey by isFalsey is considered
// // truthy. See isFalsey for details.
// //
// func isTruthy(str string) bool {
// 	return !isFalsey(str)
// }

// // isFalsey is a predicate function that returns true if its string parameter
// // represents a value that we would want to interpret as a boolean false
// // value. Such falsey values include:
// //
// //   * The empty string ("")
// //   * Stringified integer zero ("0")
// //   * Case-insensitive variations of "false" ("FALSE", "False", ...)
// //   * Case-insensitive variations of "F"     ("F", "f")
// //   * Case-insensitive variations of "no"    ("NO", "No", ...)
// //
// // All other values are interpretted as truthy. For example, the values "+0",
// // "-0", "O.O", "+0.0", "-0.0", "zero", "nope", "nay", and "whatev" are all
// // considered truthy for our purposes here.
// //
// func isFalsey(str string) bool {
// 	if str == "" {
// 		return true
// 	}

// 	// "false" is our longest legit falsey string value
// 	if len(str) > len("false") {
// 		return false
// 	}

// 	switch strings.ToLower(str) {
// 	case "false":
// 		return true
// 	case "f":
// 		return true
// 	case "0":
// 		return true
// 	case "no":
// 		return true

// 	default:
// 		return false
// 	}
// }

// // cleanPreservingRelativePrefix augments filepath.Clean() to produce a
// // cleaned filepath, while ensuring that a relative filepath starts with a
// // './' or '../' prefix (or the Windows equivalent, if needed).
// //
// // If you pass a relative filepth value such as './foo' through
// // filepath.Clean(), you'll get back 'foo'. That's usually what you want, but
// // not what we need here.
// //
// // For the purpose of constructing a 'file://' URI -- especially a
// // non-RFC-compliant one that contains a relative filepath value in it -- we
// // need the cleaned version of our relative filepath to preserve some obvious
// // indicator that it is relative. This is because the filepath portion of the
// // URI must be separated from the <host>/authorization part of the URI by a
// // '/' character.
// //
// // For absolute paths there's no problem (well, at least not on Unix-like
// // systems):
// //
// //     orig:    /some/random/./file/path
// //     clean:   /some/random/file/path
// //     in URI:  file:///some/random/file/path
// //
// // For relative paths we need to be careful to not accidentally turn the
// // relative value into an abosolute value. A naive application of
// // filepath.Clean could lead to:
// //
// //     orig:    ./some/random/./file/path
// //     clean:   some/random/file/path
// //     in URI:  file:///some/random/file/path   <== broken! has become absolute
// //
// // For our use case here, we need the previous example to work like this:
// //
// //     orig:    ./some/random/./file/path
// //     clean:   some/random/file/path
// //         <the magic of this function applied>
// //     in URI:  file:///./some/random/file/path   <== only half broken!  ;-)
// //
// func cleanPreservingRelativePrefix(maybeRelativeFilepath string) (string) {

// 	abs := filepath.IsAbs(maybeRelativeFilepath)

// 	maybeRelativeFilepath = filepath.Clean(maybeRelativeFilepath)

// 	if abs {
// 		return maybeRelativeFilepath
// 	}

// 	if len(filepath.VolumeName(maybeRelativeFilepath)) > 0 {
// 		//
// 		// Writing in 2020, Windows is the only golang-supported platform on
// 		// which filepaths can have a "volume name" of non-zero length. We're
// 		// generally talking about one of three things: a "drive letter"
// 		// (e.g., "c:"), a "reserved name" (e.g., "COM1", "LPT1"), or a UNC
// 		// path ("\\server\\share", "//server//share"). In all cases, that
// 		// non-empty volume token "shields" the path from being inadvertently
// 		// changed from a relative path to something that looks like an
// 		// abosolute path when dropped into a 'file://' URI. So there's
// 		// nothing more for us to do to protect the value here.
// 		//
// 		return maybeRelativeFilepath
// 	}

// 	if strings.HasPrefix(maybeRelativeFilepath, string(os.PathSeparator)) {
// 		return maybeRelativeFilepath
// 	}

// 	// un-clean it, just a little
// 	//
// 	// ex:  "some/file/path"  => "./some/file/path"
// 	//
// 	rtn := fmt.Sprintf(".%s%s", string(os.PathSeparator), maybeRelativeFilepath)

// 	return rtn
// }

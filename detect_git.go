package getter

// import (
// 	"fmt"
// 	"path/filepath"
// 	"strings"
// )

// GitDetector implements Detector to detect Git SSH URLs such as
// git@host.com:dir1/dir2 and converts them to proper URLs.
type GitDetector struct{}

func (d *GitDetector) Detect(src, _ string) (string, bool, error) {
	if len(src) == 0 {
		return "", false, nil
	}

	u, err := detectSSH(src)
	if err != nil {
		return "", true, err
	}
	if u == nil {
		return "", false, nil
	}

	// We require the username to be "git" to assume that this is a Git URL
	if u.User.Username() != "git" {
		return "", false, nil
	}

	return "git::" + u.String(), true, nil
}

// // detectGitForceFilepath(...) allows the 'git::' force token to be used on a
// // filepath to a Git repository. Both absolute and relative filepaths are
// // supported.
// //
// // When in-effect, the returned string will contain:
// //
// //     "git::file://<abspath>"
// // OR:
// //     "git::file://<abspath>?<query_params>"
// //
// // where <abspath> is the provided src param expanded to an absolute filepath
// // (if it wasn't already). If the provided src param is a relative filepath,
// // then the expanded absolute file path is based on the current working
// // directory of the process.
// //
// // Note that detectGitForceFilepath(...) only handles filepaths that have been
// // explicitly forced (via the 'git::' force token) for Git processing. The
// // caller must still utilize the Detect(...) interface for other bits of
// // Git-specific detection.
// //
// //
// // Q: Why isn't this funcationality in our Detect(...) implementation?
// //
// // A: It is only safe to do when the 'git::' force token is provided, and the
// //    Detector interface does not currently support a mechanism for the caller
// //    to supply that context. Consequently, the GitDetector implementation
// //    cannot assume that a filepath is intended for Git processing (that is,
// //    as representing a Git repository). Enter this function.
// //
// //    As a special case for when the 'git::' forcing key was provided, the
// //    caller can communicate that info to this function in a manner similar to
// //    the Detect(...) method, but with the additional context of the force
// //    key. That makes it safe for us to treat a filepath as knowlingly
// //    intended for Git.
// //
// //
// // Q: Why is the returned value embedded in a 'file://' URI?
// //
// // A. When specifying the 'git::' force token on a filepath, the intention is
// //    to indicate that it is a path to a Git repository that can be cloned,
// //    etc. Though Git will accept both relative and absolute file paths for
// //    this purpose, we unlock more functionality by using a 'file://' URI. In
// //    particular, that allows our GitGetter to use any provided query params
// //    to specify a specific tag or commit hash, for example.
// //
// //
// // Q: Why is a relative path expanded to an absolute path in the returned
// //    'file://' URI?
// //
// // A: Relative paths are technically not "legal" in RFC 1738- and RFC 8089-
// //    compliant 'file://' URIs, and they are not accepted by Git. When
// //    generating a 'file://' URI as we're doing here, using the absolute path
// //    is the only useful thing to do.
// //
// //
// // Q: Why support this functionality at all? Why not just require that a
// //    source location use an absolute path in a 'file://' URI explicitly if
// //    that's what is needed?
// //
// // A: The primary reason is to allow support for relative filepaths to Git
// //    repos. There are use cases in which the absolute path cannot be known in
// //    advance, but a relative path to a Git repo is known. For example, when a
// //    Terraform project (or any Git-based project) uses Git submodules, it
// //    will know the relative location of the Git submodule repos, but cannot
// //    know the absolute path in advance because it will vary based on where
// //    the "superproject" repo is cloned. Nevertheless, those relative paths
// //    should be usable as clonable Git repos, and this mechanism allows for
// //    that. Support for filepaths that are already absolute is provided mainly
// //    for symmetry.
// //
// func detectGitForceFilepath(src, _, force string) (string, bool, error) {

// 	// The full force key token is 'git::', but the Detect(...) dispatcher
// 	// function provides us with the parsed value (without the trailing
// 	// colons).
// 	//
// 	if force != "git" {
// 		return "", false, nil
// 	}

// 	if len(src) == 0 {
// 		return "", false, nil
// 	}

// 	var srcAbs string

// 	if filepath.IsAbs(src) {
// 		srcAbs = src
// 	} else {

// 		// A relative filepath MUST begin with './' or '../', or the Windows
// 		// equivalent.
// 		//
// 		if !isLocalSourceAddr(src) {
// 			return "", false, nil
// 		}

// 		var err error
// 		srcAbs, err = filepath.Abs(src)
// 		if err != nil {
// 			return "", false, err
// 		}
// 	}

// 	if !strings.HasPrefix(srcAbs, "/") {
// 		// An absolute file path on Unix will start with a '/', but that is
// 		// not true for all OS's. RFC 8089 makes the authority component
// 		// (including the '//') optional in a 'file:' URL, but git (at least
// 		// as of version 2.28.0) only recognizes the 'file://' form. In fact,
// 		// the git-clone(1) manpage is explicit that it wants the syntax to
// 		// be:
// 		//
// 		//     file:///path/to/repo.git/
// 		//
// 		// Some notes on the relevant RFCs:
// 		//
// 		// RFC 1738 (section 3.10, "FILES") documents a <host> and <path>
// 		// portion being separated by a '/' character:
// 		//
// 		//     file://<host>/<path>
// 		//
// 		// RFC 2396 (Appendix G.2, "Modifications from both RFC 1738 and RFC
// 		// 1808") refines the above by declaring that the '/' is actually part
// 		// of the path. It is still required to separate the "authority
// 		// portion" of the URI from the path portion, but is not a separate
// 		// component of the URI syntax.
// 		//
// 		// RFC 3986 (Section 3.2, "Authority") states that the authority
// 		// component of a URI "is terminated by the next slash ("/"), question
// 		// mark ("?"), or number sign ("#") character, or by the end of the
// 		// URI." However, for 'file:' URIs, only those terminated by a '/'
// 		// characters are supported by Git (as noted above).
// 		//
// 		// RFC 8089 (Appendix A, "Differences from Previous Specifications")
// 		// references the RFC 1738 form including the required '/' after the
// 		// <host>/authority component.
// 		//
// 		// Because it is the most compatible approach across the only
// 		// partially compatible RFC recommendations, and (more importantly)
// 		// because it is what Git requires for 'file:' URIs, we require that
// 		// our absolute path value start with a '/' character.
// 		//
// 		srcAbs = "/" + srcAbs
// 	}

// 	rtn := fmt.Sprintf("%s::file://%s", force, srcAbs)
// 	return rtn, true, nil
// }

// // Borrowed from terraform/internal/initwd/getter.go
// var localSourcePrefixes = []string{
// 	"./",
// 	"../",
// 	".\\",
// 	"..\\",
// }

// func isLocalSourceAddr(addr string) bool {
// 	for _, prefix := range localSourcePrefixes {
// 		if strings.HasPrefix(addr, prefix) {
// 			return true
// 		}
// 	}
// 	return false
// }

package getter

import (
	"testing"
	"regexp"
)

func TestGitDetector(t *testing.T) {
	cases := []struct {
		Input  string
		Output string
	}{
		{
			"git@github.com:hashicorp/foo.git",
			"git::ssh://git@github.com/hashicorp/foo.git",
		},
		{
			"git@github.com:org/project.git?ref=test-branch",
			"git::ssh://git@github.com/org/project.git?ref=test-branch",
		},
		{
			"git@github.com:hashicorp/foo.git//bar",
			"git::ssh://git@github.com/hashicorp/foo.git//bar",
		},
		{
			"git@github.com:hashicorp/foo.git?foo=bar",
			"git::ssh://git@github.com/hashicorp/foo.git?foo=bar",
		},
		{
			"git@github.xyz.com:org/project.git",
			"git::ssh://git@github.xyz.com/org/project.git",
		},
		{
			"git@github.xyz.com:org/project.git?ref=test-branch",
			"git::ssh://git@github.xyz.com/org/project.git?ref=test-branch",
		},
		{
			"git@github.xyz.com:org/project.git//module/a",
			"git::ssh://git@github.xyz.com/org/project.git//module/a",
		},
		{
			"git@github.xyz.com:org/project.git//module/a?ref=test-branch",
			"git::ssh://git@github.xyz.com/org/project.git//module/a?ref=test-branch",
		},
		{
			// Already in the canonical form, so no rewriting required
			// When the ssh: protocol is used explicitly, we recognize it as
			// URL form rather than SCP-like form, so the part after the colon
			// is a port number, not part of the path.
			"git::ssh://git@git.example.com:2222/hashicorp/foo.git",
			"git::ssh://git@git.example.com:2222/hashicorp/foo.git",
		},
	}

	pwd := "/pwd"
	f := new(GitDetector)
	ds := []Detector{f}
	for _, tc := range cases {
		t.Run(tc.Input, func(t *testing.T) {
			output, err := Detect(tc.Input, pwd, ds)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			if output != tc.Output {
				t.Errorf("wrong result\ninput: %s\ngot:   %s\nwant:  %s", tc.Input, output, tc.Output)
			}
		})
	}
}

func Test_detectGitForceFilepath_directly_pos(t *testing.T) {

	// Test case input values represent the parsed inputs, similar to what
	// would be presented by the Detect(...) dispatch function.
	//
	// Expected outputs are regexen because the exact values (which contain
	// absolute filepaths) are dependent upon where the tests are run. We test
	// the bits we're certain of, and allow for environment-specific
	// differences for the rest.
	//
	// CAREFUL: Recall that Detect(...) does subdir parsing before attempting
	//          any Detect-related handling, so in practice
	//          detectGitForceFilepath(...) should never see a ('//') subdir in
	//          its 'src' param. Consequently, our positive tests below that
	//          fake one up, anyway, result in "normalized" paths as a result
	//          of expanding the relative filepaths to absolute filepaths; the
	//          function is behaving correctly by /not/ doing addtional subdir
	//          handling. Compare with:
	//          Test_detectGitForceFilepath_indirectly(...), which leverages
	//          the higher-level processing of Detect(...), as well.

	posCases := []struct {
		Input  string
		Output string
	}{
		{
			"/somedir",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*somedir$",
		},
		{
			"./somedir",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*somedir$",
		},
		{
			"/somedir/two",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*somedir/two$",
		},
		{
			"./somedir/two",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*somedir/two$",
		},
		{
			"/somedir/two/three",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*somedir/two/three$",
		},
		{
			"./somedir/two/three",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*somedir/two/three$",
		},
		{   // subdir is retained here: abs path is not expanded (good)
			"/somedir/two//three",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*somedir/two//three$",
		},
		{
			"./somedir/two//three",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*somedir/two/three$",  // no subdir here (okay)
		},
		{
			"/somedir/two/three?ref=v4.5.6",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*somedir/two/three[?]ref=v4[.]5[.]6$",
		},
		{
			"./somedir/two/three?ref=v4.5.6",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*somedir/two/three[?]ref=v4[.]5[.]6$",
		},
		{
			"../some-parent-dir",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*some-parent-dir$",
		},
		{
			"../some-parent-dir/two",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*some-parent-dir/two$",
		},
		{
			"../some-parent-dir/two/three",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*some-parent-dir/two/three$",
		},
		{
			"../some-parent-dir/two//three",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*some-parent-dir/two/three$",  // no subdir here (okay)
		},
		{
			"../../some-grandparent-dir",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*some-grandparent-dir$",
		},
		{
			"../../some-grandparent-dir?ref=v1.2.3",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*some-grandparent-dir[?]ref=v1[.]2[.]3$",
		},
	}

	pwd := "/pwd-ignored"
	force := "git"  // parsed form of magic 'git::' force token

	for _, tc := range posCases {
		t.Run(tc.Input, func(t *testing.T) {
			output, ok, err := detectGitForceFilepath(tc.Input, pwd, force)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if !ok {
				t.Fatalf("unexpected !ok")
			}

			matched, err := regexp.MatchString(tc.Output, output)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if !matched {
				t.Errorf("wrong result\ninput: %s\ngot:   %s\nwant (regex match):  %s", tc.Input, output, tc.Output)
			}
		})
	}
}

func Test_detectGitForceFilepath_directly_neg(t *testing.T) {

	// Test case input values represent the parsed inputs, similar to what
	// would be presented by the Detect(...) dispatch function.
	//
	// These are negative tests; the input represent values that
	// detectGitForceFilepath(...) should effectively ignore. So most outputs
	// are expected to be a !ok flag coupled with an empty result string.
	//
	// Recall that detectGitForceFilepath(...) considers as relative only those
	// paths that begin with './' or '../', or their Windows
	// equivalents. Paths in the form 'first/second' are not considered
	// relative.

	negCases := []struct {
		Input  string
		Output string
	}{
		{
			"",
			"",
		},
		{
			"somedir",
			"",
		},
		{
			"somedir/two",
			"",
		},
		{
			"somedir/two//three",
			"",
		},
		{
			"somedir/two/three?ref=v4.5.6",
			"",
		},
	}

	pwd := "/pwd-ignored"

	// We'll loop over our tests multiple times, with 'force' set to different
	// values. The tests in which the value is not 'git' should short circuit
	// quickly because the function should only have an effect when the force
	// token is in effect. For these iterations, the input string values
	// should be irrelevant.
	//
	// Those tests in which 'force is set to "git" should also result in
	// negative results here, but exercise a different part of the
	// logic. These simulate the force token being in effect, and the function
	// interpretting the string input values to decide whether to
	// respond. Check the coverage report.
	//
	// The parsed form of the magic 'git::' force token is "git". All of the
	// other values (including "git::") should be ignored by
	// detectGitForceFilepath(...).
	//
	forceVals := []string{"", "blah:", "blah::", "git:", "git::", "git"}

	for _, force := range forceVals {

		for _, tc := range negCases {
			t.Run(tc.Input, func(t *testing.T) {
				output, ok, err := detectGitForceFilepath(tc.Input, pwd, force)
				if err != nil {
					t.Fatalf("unexpected error: %s", err)
				}
				if ok {
					t.Errorf("unexpected ok on input: %s", tc.Input)
				}
				if output != "" {
					t.Errorf("unexpected non-empty output string; input: %s; output: %s", tc.Input, output)
				}
			})
		}
	}

}

func Test_detectGitForceFilepath_indirectly_pos(t *testing.T) {

	// Test case input values represent the raw input provided to the
	// Detect(...) dispatch function, which will parse them prior to invoking
	// detectGitForceFilepath(...).
	//
	// Expected outputs are regexen because the exact values (which contain
	// absolute filepaths) are dependent upon where the tests are run. We test
	// the bits we're certain of, and allow for environment-specific
	// differences for the rest.

	posCases := []struct {
		Input  string
		Output string
	}{
		{
			"git::/somedir",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*somedir$",
		},
		{
			"git::./somedir",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*somedir$",
		},

		{
			"git::/somedir/two",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*somedir/two$",
		},
		{
			"git::./somedir/two",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*somedir/two$",
		},

		{
			"git::/somedir/two/three",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*somedir/two/three$",
		},
		{
			"git::./somedir/two/three",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*somedir/two/three$",
		},

		{
			"git::/somedir/two//three",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*somedir/two//three$",  // subdir is preserved
		},
		{
			"git::./somedir/two//three",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*somedir/two//three$",  // subdir is preserved
		},

		{
			"git::/somedir/two/three?ref=v4.5.6",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*somedir/two/three[?]ref=v4[.]5[.]6$",
		},
		{
			"git::./somedir/two/three?ref=v4.5.6",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*somedir/two/three[?]ref=v4[.]5[.]6$",
		},

		{
			"git::/somedir/two//three?ref=v4.5.6",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*somedir/two//three[?]ref=v4[.]5[.]6$",  // subdir is preserved
		},
		{
			"git::./somedir/two//three?ref=v4.5.6",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*somedir/two//three[?]ref=v4[.]5[.]6$",  // subdir is preserved
		},

		{
			"git::../some-parent-dir",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*some-parent-dir$",
		},
		{
			"git::../some-parent-dir/two",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*some-parent-dir/two$",
		},
		{
			"git::../some-parent-dir/two/three",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*some-parent-dir/two/three$",
		},
		{
			"git::../some-parent-dir/two//three",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*some-parent-dir/two//three$",  // subdir is preserved
		},
		{
			"git::../../some-grandparent-dir",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*some-grandparent-dir$",
		},
		{
			"git::../../some-grandparent-dir?ref=v1.2.3",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*some-grandparent-dir[?]ref=v1[.]2[.]3$",
		},
		{   // subdir is preserved on output
			"git::../../some-grandparent-dir/childdir//moduledir?ref=v1.2.3",
			"^git::file:///(?:[.]{1,2}/){0}(?:.*/+)*some-grandparent-dir/childdir//moduledir[?]ref=v1[.]2[.]3$",
		},
	}

	pwd := "/pwd-ignored"

	f := new(GitDetector)
	ds := []Detector{f}

	for _, tc := range posCases {
		t.Run(tc.Input, func(t *testing.T) {

			output, err := Detect(tc.Input, pwd, ds)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}

			matched, err := regexp.MatchString(tc.Output, output)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if !matched {
				t.Errorf("wrong result\ninput: %s\ngot:   %s\nwant (regex match):  %s", tc.Input, output, tc.Output)
			}
		})
	}
}

func Test_detectGitForceFilepath_indirectly_neg(t *testing.T) {

	// Test case input values represent the path string input provided to the
	// Detect(...) dispatch function, which will parse them prior to invoking
	// detectGitForceFilepath(...). We loop over the list several times with
	// different force key values to trigger different paths through the
	// code. All of these tests should produce negative results either due to
	// the absence of a legit 'git::' force key, or because the path value
	// does not represent a relative in the form required.

	negCases := []struct {
		Input  string
		Output string
	}{
		// FIXME: This first case (which is pathological, to be sure)
		//        accidentally succeeds, or so it seems, with joined with the
		//        'git::' prefix.
		// {
		// 	"",
		// 	"",
		// },
		{
			"somedir",
			"",
		},
		{
			"somedir/two",
			"",
		},
		{
			"somedir/two/three",
			"",
		},
		{
			"somedir/two//three",
			"",
		},
		{
			"somedir/two/three?ref=v4.5.6",
			"",
		},
		{
			"somedir/two//three?ref=v4.5.6",
			"",
		},
	}

	pwd := "/pwd-ignored"

	f := new(GitDetector)
	ds := []Detector{f}

	// Empty-string force will fail because no Detector handles these
	// quasi-relative filepaths.
	//
	// The 'git::' force will fail because neither GitDetector.Detect(...) nor
	// detectGitForceFilepath(...) recognize the quasi-relative filepaths (and
	// neither do any other Detector implemenations).
	//
	forceVals := []string{"", "git::"}

	for _, force := range forceVals {
		for _, tc := range negCases {
			t.Run(tc.Input, func(t *testing.T) {

				output, err := Detect(force + tc.Input, pwd, ds)
				if err == nil {
					t.Fatalf("was expecting invalid source string error, but call succeeded: output: %s (force is: %s)", output, force)
				}

				if output != "" {
					t.Errorf("unexpected non-empty output string; input: %s; output: %s (force is: %s)", tc.Input, output, force)
				}
			})
		}
	}

	// // These look vaguely like a URI. They won't error out, but the output
	// // should just be identical to the input. It should just get "passed
	// // through"
	// // forceVals = []string{"blah:", "blah::", "git:"}
	// forceVals = []string{"git::"}

	// for _, force := range forceVals {
	// 	for _, tc := range negCases {
	// 		t.Run(tc.Input, func(t *testing.T) {

	// 			catenatedInput := force + tc.Input
	// 			output, err := Detect(catenatedInput, pwd, ds)
	// 			if err != nil {
	// 				t.Fatalf("unexpected error: %s (force is: %s)", err, force)
	// 			}

	// 			if output != catenatedInput {
	// 				// t.Errorf("unexpected non-empty output string; input: %s; output: %s (force is: %s)", tc.Input, output, force)
	// 				t.Errorf("expected input to be passed through; input: %s; output: %s (force is: %s)", catenatedInput, output, force)
	// 			}
	// 		})
	// 	}
	// }

}

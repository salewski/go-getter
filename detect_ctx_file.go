package getter

// "Contextual Detector" implementation for producing 'file://' URIs from
// generic file system paths.
//
type FileCtxDetector struct{
	fileDetector FileDetector
}

func (d *FileCtxDetector) CtxDetect(src, pwd, _, _, _ string) (string, bool, error) {

	// Currently not taking advantage of the extra contextual data available
	// to us. For now, we just delegate to our wrapped FileDetector.
	//
	return d.fileDetector.Detect(src, pwd)
}

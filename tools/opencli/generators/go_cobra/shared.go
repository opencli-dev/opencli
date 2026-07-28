package go_cobra

// renderShared returns the body (package clause and imports are added by
// assembleFile) of opencli.gen.go: the small set of helpers every other
// generated file in the package relies on.
func renderShared() (string, error) {
	return executeTemplate("shared", nil)
}

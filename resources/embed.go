package resources

import _ "embed"

// Icon is the application icon shared by packaging and the login screen.
//
//go:embed icon.png
var Icon []byte

// Instances is a directory of suggested domains, not a login allowlist.
//
//go:embed instances.json
var Instances []byte

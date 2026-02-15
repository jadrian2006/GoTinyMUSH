package server

// Version is the GoTinyMUSH version string.
// Override at build time with: go build -ldflags "-X github.com/crystal-mush/gotinymush/pkg/server.Version=0.2.0"
var Version = "0.2.0"

// VersionString returns the full version display string.
func VersionString() string {
	return "GoTinyMUSH " + Version
}

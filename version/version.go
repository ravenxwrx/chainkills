package version

import (
	"fmt"
	"runtime"
)

var (
	hash      string
	tag       string = "v0.1.0"
	buildTime string
)

func Version() string {
	return tag
}

func Print() {
	fmt.Printf("Version:     %s\n", tag)
	fmt.Printf("Build date:  %s\n", buildTime)
	fmt.Printf("Git hash:    %s\n", hash)
	fmt.Printf("Go version:  %s\n", runtime.Version())
	fmt.Printf("OS/Arch:     %s / %s\n", runtime.GOOS, runtime.GOARCH)
}

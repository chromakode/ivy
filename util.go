package ivy

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var timeStringFormat = "@%010d.%09d"

func timeString(timestamp time.Time) string {
	buf := new(bytes.Buffer)
	fmt.Fprintf(buf, timeStringFormat, timestamp.Unix(), timestamp.Nanosecond())
	return buf.String()
}

var validPath = regexp.MustCompile(`^/[0-9a-z-./]+$`)

func normalizePath(path string) string {
	path = strings.TrimSpace(path)

	// ensure starting /
	if path[0] != '/' {
		path = "/" + path
	}

	// trim trailing /
	if path[len(path)-1] == '/' {
		path = path[:len(path)-2]
	}

	// check for invalid characters
	if !validPath.MatchString(path) {
		return ""
	}

	return path
}

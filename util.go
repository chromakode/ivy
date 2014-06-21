package ivy

import (
	"bytes"
	"fmt"
	"time"
)

var timeStringFormat = "@%010d.%09d"

func timeString(timestamp time.Time) string {
	buf := new(bytes.Buffer)
	fmt.Fprintf(buf, timeStringFormat, timestamp.Unix(), timestamp.Nanosecond())
	return buf.String()
}

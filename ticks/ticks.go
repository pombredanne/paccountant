// Package for converting ticks to duration
package ticks

/*
#include <unistd.h>
*/
import "C"

import (
	"time"
)

func getTickHz() int64 {
	return int64(C.sysconf(C._SC_CLK_TCK))
}

func TicksToDuration(ticks int64) time.Duration {
	const ONE_OVER_NANO = 100000000
	return time.Duration((ticks * ONE_OVER_NANO) / getTickHz())
}

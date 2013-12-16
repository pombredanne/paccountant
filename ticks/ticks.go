// Package for converting ticks to duration
package ticks

/*
#include <unistd.h>
*/
import "C"

import (
	"fmt"
	"io/ioutil"
	"time"
)

// 1 / 1e-9
const NS_PER_S = 1000000000

func GetTickHz() int64 {
	return int64(C.sysconf(C._SC_CLK_TCK))
}

func TicksToDuration(ticks int64) time.Duration {
	return time.Duration((ticks * NS_PER_S) / GetTickHz())
}

func DurationToTicks(duration time.Duration) int64 {
	ns := int64(duration)
	return (ns * GetTickHz()) / NS_PER_S
}

func TicksSinceBootAsDuration(ticks int64) time.Duration {
	return TicksToDuration(DurationToTicks(Uptime()) - ticks)
	// return time.Now().Add(-Uptime()).Add(TicksToDuration(ticks))
	// uptime := Uptime()

	// panic("todo")
	// return time.Now()
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func Uptime() time.Duration {

	// Measured in ticks since boot
	uptimeString, err := ioutil.ReadFile("/proc/uptime")
	check(err)

	var uptime float64
	fmt.Sscan(string(uptimeString), &uptime)

	return time.Duration(uptime * NS_PER_S)
}

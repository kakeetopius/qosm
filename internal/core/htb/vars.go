package htb

import (
	"log"
	"math"

	"github.com/florianl/go-tc/core"
	"github.com/kakeetopius/qosm/internal/priority"
)

//1: Root (The HTB classfull qdisc to be attached at egress)
//└── 1:1 ParentClass (parent of the all classes.)
//    ├── 1:10 HighClass (high priority class. Only Packets matched by HighPrioClassFilter reach here.)
//    ├── 1:19 LowClass  (low priority class. Only Packets matched by LowPrioClassFilter reach here.)
//    └── 1:15 DefaultClass (class where packets go if no filter matches on them)

func Root() *HTBQdisc {
	return &HTBQdisc{
		Handle:       HTBQDISCHANDLE,
		Parent:       ROOTHANDLE,
		DefaultClass: uint32(HTBDEFAULTCLASS),
	}
}

func ParentClass(rateMBs uint32) *HTBClass {
	return &HTBClass{
		Handle:       HTBPARENTCLASSHANDLE,
		ParentHandle: HTBQDISCHANDLE,
		Rate:         bytesPerSecFromMBsPerSec(rateMBs),
		Burst:        calcBurst(rateMBs),
		Cburst:       calcBurst(rateMBs),
	}
}

func HighClass(rate uint32) *HTBClass {
	return &HTBClass{
		Handle:        HTBHIGHPRIOCLASSHANDLE,
		ParentHandle:  HTBPARENTCLASSHANDLE,
		ClassPriority: uint32(HTBHIGHCLASSPRIO),
		Rate:          bytesPerSecFromMBsPerSec(rate),
		Burst:         calcBurst(rate),
		Cburst:        calcBurst(rate),
	}
}

func LowClass(rate uint32) *HTBClass {
	return &HTBClass{
		Handle:        HTBLOWPRIOCLASSHANDLE,
		ParentHandle:  HTBPARENTCLASSHANDLE,
		ClassPriority: uint32(HTBLOWCLASSPRIO),
		Rate:          bytesPerSecFromMBsPerSec(rate),
		Burst:         calcBurst(rate),
		Cburst:        calcBurst(rate),
	}
}

func DefaultClass(rate uint32) *HTBClass {
	return &HTBClass{
		Handle:        HTBDEFAULTCLASSHANDLE,
		ParentHandle:  HTBPARENTCLASSHANDLE,
		ClassPriority: uint32(HTBDEFAULTCLASSPRIO),
		Rate:          bytesPerSecFromMBsPerSec(rate),
		Burst:         calcBurst(rate),
		Cburst:        calcBurst(rate),
	}
}

func HighPrioClassFilter() *FWFilter {
	return &FWFilter{
		Handle:       uint32(priority.PRIORITYHIGH),
		ParentHandle: HTBQDISCHANDLE,
		ClassID:      HTBHIGHPRIOCLASSHANDLE,
	}
}

func LowPrioClassFilter() *FWFilter {
	return &FWFilter{
		Handle:       uint32(priority.PRIORITYLOW),
		ParentHandle: HTBQDISCHANDLE,
		ClassID:      HTBLOWPRIOCLASSHANDLE,
	}
}

func bytesPerSecFromMBsPerSec(megaBitsPerSecond uint32) uint32 {
	rate := uint64(megaBitsPerSecond) * 1_000_000 / 8

	rate = min(rate, math.MaxUint32)

	return uint32(rate)
}

func calcBurst(megabitsPerSecond uint32) uint32 {
	if !core.IsClockInitialized() {
		err := core.InitializeClock()
		if err != nil {
			log.Println(err)
		}
	}

	// convert Mb/s to bytes/s
	rateBytesPerSec := bytesPerSecFromMBsPerSec(megabitsPerSecond)

	// allow a burst worth of 5ms at the given rate
	burstBytes := rateBytesPerSec * 5 / 1000

	// how much time in ticks does it take to transmit the burstBytes at the given rate
	xmitTime := core.XmitTime(uint64(rateBytesPerSec), burstBytes)

	// we return ticks burst as duration not size. That is what tc wants.
	return xmitTime
}

func getClassRates(totalRate uint32) (highClassRate uint32, defaultClassRate uint32, lowClassRate uint32) {
	highClassRate = uint32(float64(totalRate) * 50 / 100)
	defaultClassRate = uint32(float64(totalRate) * 40 / 100)
	lowClassRate = uint32(float64(totalRate) * 10 / 100)

	return
}

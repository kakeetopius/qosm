package htb

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/florianl/go-tc"
	"github.com/florianl/go-tc/core"
)

const (
	ROOTHANDLE = tc.HandleRoot
)

var (
	HTBQDISCHANDLE         = core.BuildHandle(0x1, 0x0)  // 1:0 handle of the qdisc itself to be attached at root
	HTBPARENTCLASSHANDLE   = core.BuildHandle(0x1, 0x1)  // 1:1 handle of parent class to be attached right under the root qdisc
	HTBHIGHPRIOCLASSHANDLE = core.BuildHandle(0x1, 0x10) //1:10 handle of high priority class
	HTBDEFAULTCLASSHANDLE  = core.BuildHandle(0x1, 0x15) //1:15 handle of the default class (packets that don't match any rule are sent here.)
	HTBLOWPRIOCLASSHANDLE  = core.BuildHandle(0x1, 0x19) // 1:19 handle of the low priority class

	HTBDEFAULTCLASS = 0x15 // default class minor (ie minor of 1:15 which is 15)

	HTBHIGHCLASSPRIO    = 0
	HTBDEFAULTCLASSPRIO = 2
	HTBLOWCLASSPRIO     = 4
)

type HTBObjects struct {
	Root         *tc.Object
	ParentClass  *tc.Object
	HighClass    *tc.Object
	LowClass     *tc.Object
	DefaultClass *tc.Object

	HighClassFilter *tc.Object
	LowClassFilter  *tc.Object
}

type HTBQdisc struct {
	Handle       uint32
	Parent       uint32
	DefaultClass uint32
}

type HTBClass struct {
	Handle        uint32
	ParentHandle  uint32
	Rate          uint32 // in bytes per second
	Burst         uint32
	Cburst        uint32
	ClassPriority uint32
}

type FWFilter struct {
	Handle       uint32
	ParentHandle uint32
	ClassID      uint32
}

type HTBClassStats struct {
	Bytes      uint64 /* Number of enqueued bytes */
	Packets    uint32 /* Number of enqueued packets	*/
	Drops      uint32 /* Packets dropped because of lack of resources */
	Overlimits uint32 /* Number of throttle events when this
	 * flow goes out of allocated bandwidth */
	Bps     uint32 /* Current flow byte rate */
	Pps     uint32 /* Current flow packet rate */
	Qlen    uint32
	Backlog uint32
	Lends   uint32
	Borrows uint32
	Giants  uint32
	Tokens  uint32
	CTokens uint32
}

var ErrQdiscNotFound = errors.New("qdisc not found")

var ErrQdiscExists = errors.New("qdisc already exists on the interface")

type ErrClassNotFound struct {
	ClassName   string
	ClassHandle uint32
}

type ErrFilterNotFound struct {
	FilterName   string
	FilterHandle uint32
}

func (f ErrClassNotFound) Error() string {
	return "class " + f.ClassName + " not found"
}

func (f ErrFilterNotFound) Error() string {
	return "filter " + f.FilterName + " not found"
}

type ClassPercentages struct {
	HighPrioClass float32
	LowPrioClass  float32
	DefaultClass  float32
}

func (p ClassPercentages) String() string {
	return fmt.Sprintf(
		"High: %.1f%%, Default: %.1f%%, Low: %.1f%%",
		p.HighPrioClass,
		p.DefaultClass,
		p.LowPrioClass,
	)
}

func (p ClassPercentages) Verify() error {
	err := isValidPercentage(p.HighPrioClass)
	if err != nil {
		return err
	}
	err = isValidPercentage(p.LowPrioClass)
	if err != nil {
		return err
	}
	err = isValidPercentage(p.DefaultClass)
	if err != nil {
		return err
	}

	if p.DefaultClass+p.HighPrioClass+p.LowPrioClass != 100 {
		return fmt.Errorf("invalid percentages: %v %v %v. Percentages do not add up to 100", p.HighPrioClass, p.DefaultClass, p.LowPrioClass)
	}

	return nil
}

func (p ClassPercentages) Equal(q ClassPercentages) bool {
	if p.HighPrioClass != q.HighPrioClass || p.DefaultClass != q.DefaultClass || p.LowPrioClass != q.LowPrioClass {
		return false
	}

	return true
}

func ClassPercentagesFromStrings(highStr, defaultPercentageStr, lowStr string) (ClassPercentages, error) {
	high, err := strconv.ParseFloat(highStr, 32)
	if err != nil {
		return ClassPercentages{}, err
	}

	low, err := strconv.ParseFloat(lowStr, 32)
	if err != nil {
		return ClassPercentages{}, err
	}

	defaultPercentage, err := strconv.ParseFloat(defaultPercentageStr, 32)
	if err != nil {
		return ClassPercentages{}, err
	}

	return ClassPercentages{
		HighPrioClass: float32(high),
		LowPrioClass:  float32(low),
		DefaultClass:  float32(defaultPercentage),
	}, nil
}

func DefaultClassPercentages() ClassPercentages {
	return ClassPercentages{
		HighPrioClass: 50,
		LowPrioClass:  10,
		DefaultClass:  40,
	}
}

func isValidPercentage(n float32) error {
	if n > 100 || n < 0 {
		return fmt.Errorf("invalid percentage: %v", n)
	}

	return nil
}

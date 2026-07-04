// Package priority contains constants and functions to work with the different priority classes.
package priority

import "fmt"

type Priority int

const (
	PRIORITYHIGH Priority = 10
	PRIORITYLOW  Priority = 20
)

func PriorityFromString(prioString string) (Priority, error) {
	switch prioString {
	case "high":
		return PRIORITYHIGH, nil
	case "low":
		return PRIORITYLOW, nil
	}

	return 0, fmt.Errorf("unknown priority: %v", prioString)
}

func (p Priority) String() string {
	switch p {
	case PRIORITYHIGH:
		return "high"
	case PRIORITYLOW:
		return "low"
	default:
		return ""
	}
}

package govim

import (
	"fmt"
	"strings"
)

// SwitchBufValue typed constants define the set of values that the Vim setting
// switchbuf can take. See :help switchbuf for more details and definitions of
// each value.
type SwitchBufMode string

const (
	SwitchBufUseOpen SwitchBufMode = "useopen"
	SwitchBufUseTag  SwitchBufMode = "usetab"
	SwitchBufSplit   SwitchBufMode = "split"
	SwitchBufVsplit  SwitchBufMode = "vsplit"
	SwitchBufNewTab  SwitchBufMode = "newtab"
)

// ParseSwitchBufModes assumes vs is a valid value for &switchbuf
func ParseSwitchBufModes(vs string) ([]SwitchBufMode, error) {
	var modes []SwitchBufMode
	for _, v := range strings.Split(vs, ",") {
		sm := SwitchBufMode(v)
		switch sm {
		case SwitchBufUseOpen, SwitchBufUseTag, SwitchBufSplit, SwitchBufVsplit, SwitchBufNewTab:
		default:
			return nil, fmt.Errorf("invalid SwitchBufMode %q", sm)
		}
		modes = append(modes, sm)
	}
	return modes, nil
}

// Code generated by "stringer -type=GenAttr -linecomment -output gen_genattr_stringer.go"; DO NOT EDIT.

package govim

import "strconv"

func _() {
	// An "invalid array index" compiler error signifies that the constant values have changed.
	// Re-run the stringer command to generate them again.
	var x [1]struct{}
	_ = x[AttrBang-0]
	_ = x[AttrBar-1]
	_ = x[AttrRegister-2]
	_ = x[AttrBuffer-3]
}

const _GenAttr_name = "-bang-bar-register-buffer"

var _GenAttr_index = [...]uint8{0, 5, 9, 18, 25}

func (i GenAttr) String() string {
	if i >= GenAttr(len(_GenAttr_index)-1) {
		return "GenAttr(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _GenAttr_name[_GenAttr_index[i]:_GenAttr_index[i+1]]
}

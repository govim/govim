package stringfns

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strconv"
)

type Function func(string) (string, error)

var Functions = map[string]Function{
	"strconv.Quote":               strconv_Quote,
	"strconv.Unquote":             strconv.Unquote,
	"regexp.QuoteMeta":            regexp_QuoteMeta,
	"crypto/sha256.Sum256":        crypto__sha256_Sum256,
	"encoding/hex.EncodeToString": encoding__hex_EncodeToString,
}

func strconv_Quote(v string) (string, error) {
	return strconv.Quote(v), nil
}

func regexp_QuoteMeta(v string) (string, error) {
	return regexp.QuoteMeta(v), nil
}

func crypto__sha256_Sum256(s string) (string, error) {
	v := sha256.Sum256([]byte(s))
	return string(v[:]), nil
}

func encoding__hex_EncodeToString(s string) (string, error) {
	return hex.EncodeToString([]byte(s)), nil
}

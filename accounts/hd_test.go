package accounts

import (
	"reflect"
	"testing"
)

// Tests that HD derivation paths can be correctly parsed into our internal binary
// representation.
func TestHDPathParsing(t *testing.T) {
	tests := []struct {
		input  string
		output DerivationPath
	}{
		// Plain absolute derivation paths
		{"m/44'/6060'/0'/0", DerivationPath{0x80000000 + 44, 0x80000000 + 6060, 0x80000000 + 0, 0}},
		{"m/44'/6060'/0'/128", DerivationPath{0x80000000 + 44, 0x80000000 + 6060, 0x80000000 + 0, 128}},
		{"m/44'/6060'/0'/0'", DerivationPath{0x80000000 + 44, 0x80000000 + 6060, 0x80000000 + 0, 0x80000000 + 0}},
		{"m/44'/6060'/0'/128'", DerivationPath{0x80000000 + 44, 0x80000000 + 6060, 0x80000000 + 0, 0x80000000 + 128}},
		{"m/2147483692/2147489708/2147483648/0", DerivationPath{0x80000000 + 44, 0x80000000 + 6060, 0x80000000 + 0, 0}},
		{"m/2147483692/2147489708/2147483648/2147483648", DerivationPath{0x80000000 + 44, 0x80000000 + 6060, 0x80000000 + 0, 0x80000000 + 0}},

		// Plain relative derivation paths
		{"0", DerivationPath{0x80000000 + 44, 0x80000000 + 6060, 0x80000000 + 0, 0, 0}},
		{"128", DerivationPath{0x80000000 + 44, 0x80000000 + 6060, 0x80000000 + 0, 0, 128}},
		{"0'", DerivationPath{0x80000000 + 44, 0x80000000 + 6060, 0x80000000 + 0, 0, 0x80000000 + 0}},
		{"128'", DerivationPath{0x80000000 + 44, 0x80000000 + 6060, 0x80000000 + 0, 0, 0x80000000 + 128}},
		{"2147483648", DerivationPath{0x80000000 + 44, 0x80000000 + 6060, 0x80000000 + 0, 0, 0x80000000 + 0}},

		// Hexadecimal absolute derivation paths
		{"m/0x2C'/0x17AC'/0x00'/0x00", DerivationPath{0x80000000 + 44, 0x80000000 + 6060, 0x80000000 + 0, 0}},
		{"m/0x2C'/0x17AC'/0x00'/0x80", DerivationPath{0x80000000 + 44, 0x80000000 + 6060, 0x80000000 + 0, 128}},
		{"m/0x2C'/0x17AC'/0x00'/0x00'", DerivationPath{0x80000000 + 44, 0x80000000 + 6060, 0x80000000 + 0, 0x80000000 + 0}},
		{"m/0x2C'/0x17AC'/0x00'/0x80'", DerivationPath{0x80000000 + 44, 0x80000000 + 6060, 0x80000000 + 0, 0x80000000 + 128}},
		{"m/0x8000002C/0x800017AC/0x80000000/0x00", DerivationPath{0x80000000 + 44, 0x80000000 + 6060, 0x80000000 + 0, 0}},
		{"m/0x8000002C/0x800017AC/0x80000000/0x80000000", DerivationPath{0x80000000 + 44, 0x80000000 + 6060, 0x80000000 + 0, 0x80000000 + 0}},

		// Hexadecimal relative derivation paths
		{"0x00", DerivationPath{0x80000000 + 44, 0x80000000 + 6060, 0x80000000 + 0, 0, 0}},
		{"0x80", DerivationPath{0x80000000 + 44, 0x80000000 + 6060, 0x80000000 + 0, 0, 128}},
		{"0x00'", DerivationPath{0x80000000 + 44, 0x80000000 + 6060, 0x80000000 + 0, 0, 0x80000000 + 0}},
		{"0x80'", DerivationPath{0x80000000 + 44, 0x80000000 + 6060, 0x80000000 + 0, 0, 0x80000000 + 128}},
		{"0x80000000", DerivationPath{0x80000000 + 44, 0x80000000 + 6060, 0x80000000 + 0, 0, 0x80000000 + 0}},

		// Weird inputs just to ensure they work
		{"	m  /   44			'\n/\n   6060	\n\n\t'   /\n0 ' /\t\t	0", DerivationPath{0x80000000 + 44, 0x80000000 + 6060, 0x80000000 + 0, 0}},

		// Invalid derivation paths
		{"", nil},                // Empty relative derivation path
		{"m", nil},               // Empty absolute derivation path
		{"m/", nil},              // Missing last derivation component
		{"/44'/6060'/0'/0", nil}, // Absolute path without m prefix, might be user error
		{"m/2147483648'", nil},   // Overflows 32 bit integer
		{"m/-1'", nil},           // Cannot contain negative number
	}
	for i, tt := range tests {
		if path, err := ParseDerivationPath(tt.input); !reflect.DeepEqual(path, tt.output) {
			t.Errorf("test %d: parse mismatch: have %v (%v), want %v", i, path, err, tt.output)
		} else if path == nil && err == nil {
			t.Errorf("test %d: nil path and error: %v", i, err)
		}
	}
}

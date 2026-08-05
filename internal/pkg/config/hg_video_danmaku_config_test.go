package ConfigPackage

import "testing"

func TestPowerOfTwoResourceBoundary(t *testing.T) {
	for _, testCase := range []struct {
		value, minValue, maxValue int
		want                      bool
	}{
		{value: 16, minValue: 16, maxValue: 4096, want: true},
		{value: 256, minValue: 16, maxValue: 4096, want: true},
		{value: 15, minValue: 16, maxValue: 4096, want: false},
		{value: 48, minValue: 16, maxValue: 4096, want: false},
		{value: 8192, minValue: 16, maxValue: 4096, want: false},
	} {
		if got := hgPowerOfTwo(testCase.value, testCase.minValue, testCase.maxValue); got != testCase.want {
			t.Fatalf("hgPowerOfTwo(%d, %d, %d) = %t, want %t", testCase.value, testCase.minValue, testCase.maxValue, got, testCase.want)
		}
	}
}

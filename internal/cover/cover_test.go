package cover

import "testing"

func TestParseTotal(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want float64
	}{
		{
			name: "typical output",
			in: "github.com/x/index.go:8:\tGreet\t100.0%\n" +
				"github.com/x/index.go:12:\tToLabel\t66.7%\n" +
				"total:\t\t(statements)\t80.0%\n",
			want: 80,
		},
		{
			name: "single file",
			in:   "total:\t\t(statements)\t100.0%\n",
			want: 100,
		},
		{
			name: "fractional",
			in:   "total:\t\t(statements)\t61.5%\n",
			want: 61.5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseTotal(tt.in)
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("ParseTotal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseTotalErrors(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{name: "empty", in: ""},
		{name: "no total line", in: "file.go:1:\tF\t50.0%\n"},
		{name: "malformed percent", in: "total:\t(statements)\txx%\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ParseTotal(tt.in); err == nil {
				t.Error("ParseTotal() succeeded, want error")
			}
		})
	}
}

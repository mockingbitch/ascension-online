package character

import "testing"

func TestNormalizeName(t *testing.T) {
	ok := map[string]string{
		"Hàn Lập":        "Hàn Lập",
		"  Tiêu   Dao  ": "Tiêu Dao",
		"Nguyễn Thị Ánh": "Nguyễn Thị Ánh",
		"Hàn":           "Hàn", // decomposed → NFC
	}
	for in, want := range ok {
		got, err := NormalizeName(in)
		if err != nil || got != want {
			t.Errorf("NormalizeName(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "A", "admin123", "<script>", "Tên_Có_Gạch", "Một cái tên rất là dài quá mức cho phép",
		"\u3164\u3164", "\u0300\u0301", "\u0410dmin", "Tên\u200bẨn"} {
		if _, err := NormalizeName(bad); err == nil {
			t.Errorf("NormalizeName(%q) accepted", bad)
		}
	}
}

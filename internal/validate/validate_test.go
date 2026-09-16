package validate

import "testing"

func TestPhone(t *testing.T) {
	valid := []struct {
		in   string
		note string
	}{
		{"+2341234567890", "Nigeria, correct international form"},
		{"2341234567890", "same number without the + prefix"},
		{"+5491123456789", "Argentina"},
		{"+14155552671", "US"},
		{"+447911123456", "UK"},
		{"+861380013800", "China"},
		{"+1234567", "shortest accepted (7 digits)"},
		{"+123456789012345", "longest accepted (15 digits)"},
	}
	for _, c := range valid {
		if !Phone(c.in) {
			t.Errorf("Phone(%q) = false, want true (%s)", c.in, c.note)
		}
	}

	invalid := []struct {
		in   string
		note string
	}{
		// The case that motivated this: a national number given a + prefix.
		// No country code starts with 0, so this can never route.
		{"+01234567890", "national trunk prefix after +"},
		{"01234567890", "national form, no country code"},
		{"+0000000", "all zeros"},
		{"+123456", "too short (6 digits)"},
		{"+1234567890123456", "too long (16 digits)"},
		{"", "empty"},
		{"+", "prefix only"},
		{"+55 11 91234 5678", "spaces"},
		{"+55-11-912345678", "dashes"},
		{"not-a-number", "letters"},
	}
	for _, c := range invalid {
		if Phone(c.in) {
			t.Errorf("Phone(%q) = true, want false (%s)", c.in, c.note)
		}
	}
}

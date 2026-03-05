package toolkit

import "testing"

func TestTools_RandomString(t *testing.T) {
	var testTools Tools

	s := testTools.RandomString(10)
	if len(s) != 10 {
		t.Error("Wrong length random string generated")
	}

	ns := testTools.RandomString(10)
	if s == ns {
		t.Error("Same string are getting generated. Algo not random enough")
	}
}

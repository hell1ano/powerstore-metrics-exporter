package utils

import "testing"

func TestRequestLimit(t *testing.T) {
	for _, limit := range []int{0, 1, 200} {
		if err := InitReqCounter(limit); err != nil {
			t.Fatal(err)
		}
		select {
		case ReqCounter <- 1:
			<-ReqCounter
		default:
			t.Fatalf("first request would block with reqLimit=%d", limit)
		}
	}
	if err := InitReqCounter(-1); err == nil {
		t.Fatal("negative request limit accepted")
	}
}

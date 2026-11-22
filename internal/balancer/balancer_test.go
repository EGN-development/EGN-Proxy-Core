package balancer

import (
	"net/url"
	"testing"
)

func TestWeightedSelectionAndHealth(t *testing.T) {
	a, _ := url.Parse("http://a")
	b, _ := url.Parse("http://b")
	beA := &Backend{Name: "a", Target: a, Weight: 3, Hostnames: map[string]struct{}{"*": {}}}
	beB := &Backend{Name: "b", Target: b, Weight: 1, Hostnames: map[string]struct{}{"*": {}}}
	lb, err := New([]*Backend{beA, beB})
	if err != nil {
		t.Fatal(err)
	}
	countA := 0
	countB := 0
	for i := 0; i < 1000; i++ {
		be := lb.Next("test")
		if be == beA {
			countA++
		}
		if be == beB {
			countB++
		}
	}
	if countA <= countB {
		t.Fatalf("weighted selection broken: A=%d B=%d", countA, countB)
	}

	beA.Healthy.Store(false)
	for i := 0; i < 20; i++ {
		be := lb.Next("test")
		if be != beB {
			t.Fatalf("expected only healthy backend, got %v", be.Name)
		}
	}
}

func TestHostMatchWithoutPortAndCaseInsensitive(t *testing.T) {
	u, _ := url.Parse("http://a")
	be := &Backend{
		Name:      "site",
		Target:    u,
		Weight:    1,
		Hostnames: map[string]struct{}{"example.com": {}},
	}
	lb, err := New([]*Backend{be})
	if err != nil {
		t.Fatal(err)
	}

	got := lb.Next("EXAMPLE.com:443")
	if got == nil || got.Name != "site" {
		t.Fatalf("expected host-based match, got %#v", got)
	}
}

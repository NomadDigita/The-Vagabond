package main

import "testing"

func TestCommentBaseline(t *testing.T) {
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	current, err := collect(root)
	if err != nil {
		t.Fatal(err)
	}
	base, err := readBaseline(baselinePath(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(current) != len(base) || !isSubset(current, base) || !isSubset(base, current) {
		t.Fatal(reportMismatch(current, base))
	}
}

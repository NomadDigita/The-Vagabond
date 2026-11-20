package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func main() {
	root, err := repoRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	write := false
	initial := false
	ratchet := ""
	for _, arg := range os.Args[1:] {
		switch {
		case arg == "--write":
			write = true
		case arg == "--write-initial":
			write = true
			initial = true
		case strings.HasPrefix(arg, "--ratchet="):
			ratchet = strings.TrimPrefix(arg, "--ratchet=")
		default:
			fmt.Fprintf(os.Stderr, "unknown argument %s\n", arg)
			os.Exit(2)
		}
	}
	current, err := collect(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	path := baselinePath(root)
	if write {
		if old, err := readBaseline(path); err == nil && !initial && !isSubset(current, old) {
			fmt.Fprintln(os.Stderr, "refusing to add comments to the baseline")
			fmt.Fprint(os.Stderr, reportMismatch(current, old))
			os.Exit(1)
		}
		if err := writeBaseline(path, current); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Printf("wrote %d comments to %s\n", len(current), path)
		return
	}
	base, err := readBaseline(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "reading baseline: %v\n", err)
		os.Exit(1)
	}
	if len(current) != len(base) || !isSubset(current, base) || !isSubset(base, current) {
		fmt.Fprint(os.Stderr, reportMismatch(current, base))
		os.Exit(1)
	}
	if ratchet != "" {
		if err := checkRatchet(ratchet, len(base)); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

func checkRatchet(ref string, current int) error {
	spec := ref + ":tools/commentlint/baseline.txt"
	cmd := exec.Command("git", "show", spec)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := string(out)
		if strings.Contains(msg, "does not exist") || strings.Contains(msg, "exists on disk") || strings.Contains(msg, "Not a valid object") {
			return nil
		}
		return fmt.Errorf("ratchet %s: %v\n%s", spec, err, msg)
	}
	previous := len(parseBaseline(out))
	if current > previous {
		return fmt.Errorf("comment baseline grew from %d to %d against %s", previous, current, ref)
	}
	return nil
}

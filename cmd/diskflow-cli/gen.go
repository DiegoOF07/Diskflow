package main

import (
	"fmt"
	"io"
	"time"

	"github.com/DiegoOF07/Diskflow/internal/engine/fixture"
)

func runGen(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("gen", stderr)
	out := fs.String("out", "", "output directory (must not exist or be empty)")
	entries := fs.Int("entries", 10_000, "files + dirs to create")
	seed := fs.Uint64("seed", 1, "random seed; the same seed gives the same tree")
	size := fs.Int64("size", 4096, "mean file size in bytes")
	if ok, err := parse(fs, args, 0); !ok {
		return err
	}
	if *out == "" {
		fs.Usage()
		return fmt.Errorf("%w: gen: --out is required", errUsage)
	}

	start := time.Now()
	st, err := fixture.Generate(*out, fixture.Spec{Entries: *entries, Seed: *seed, FileSize: *size})
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "files=%d dirs=%d bytes=%d elapsed_ms=%d\n",
		st.Files, st.Dirs, st.Bytes, time.Since(start).Milliseconds())
	return nil
}

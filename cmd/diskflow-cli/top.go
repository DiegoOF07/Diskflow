package main

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/DiegoOF07/Diskflow/internal/engine"
)

func runTop(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("top", stderr)
	db := fs.String("db", "diskflow.db", "database file")
	limit := fs.Int("limit", 20, "directories to list")
	if ok, err := parse(fs, args, 1); !ok {
		return err
	}
	root := fs.Arg(0)

	eng, err := engine.New(*db, engine.Options{})
	if err != nil {
		return err
	}
	defer eng.Close()

	r, err := eng.Root(root)
	if err != nil {
		return err
	}
	dirs, err := eng.TopDirs(root, *limit)
	if err != nil {
		return err
	}

	fmt.Fprintf(stdout, "%s  %s in %d files\n\n", r.Name, humanBytes(r.Size), r.Files)
	w := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(w, "SIZE\tFILES\t%\t\tPATH")
	for _, d := range dirs {
		path, err := eng.Path(root, d.ID)
		if err != nil {
			return err
		}
		pct := 0.0
		if r.Size > 0 {
			pct = 100 * float64(d.Size) / float64(r.Size)
		}
		fmt.Fprintf(w, "%s\t%d\t%.1f\t\t%s\n", humanBytes(d.Size), d.Files, pct, path)
	}
	return w.Flush()
}

// humanBytes formats n with binary units: 1536 -> "1.5 KiB".
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit && exp < 3; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGT"[exp])
}

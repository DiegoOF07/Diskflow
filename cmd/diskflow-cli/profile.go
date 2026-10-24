package main

import (
	"errors"
	"flag"
	"os"
	"runtime"
	"runtime/pprof"
)

// profiles holds the output paths of the optional pprof profiles. Empty paths
// are skipped. Read them with `go tool pprof -http=:8080 FILE`.
type profiles struct {
	cpu, mem, block string
}

func (p *profiles) register(fs *flag.FlagSet) {
	fs.StringVar(&p.cpu, "cpuprofile", "", "write a CPU profile to `file`")
	fs.StringVar(&p.mem, "memprofile", "", "write a heap profile to `file` when the scan ends")
	fs.StringVar(&p.block, "blockprofile", "", "write a blocking profile (channels, mutexes) to `file`")
}

// start creates every file up front, so a bad path fails before the scan,
// and begins recording. stop ends the recording and writes the profiles.
func (p profiles) start() (stop func() error, err error) {
	files := map[string]*os.File{}
	closeAll := func() {
		for _, f := range files {
			f.Close()
		}
	}
	for name, path := range map[string]string{"cpu": p.cpu, "mem": p.mem, "block": p.block} {
		if path == "" {
			continue
		}
		f, err := os.Create(path)
		if err != nil {
			closeAll()
			return nil, err
		}
		files[name] = f
	}

	if f := files["cpu"]; f != nil {
		if err := pprof.StartCPUProfile(f); err != nil {
			closeAll()
			return nil, err
		}
	}
	if files["block"] != nil {
		// 1 records every blocking event: precise, at some cost to throughput.
		runtime.SetBlockProfileRate(1)
	}

	return func() error {
		var errs []error
		if files["cpu"] != nil {
			pprof.StopCPUProfile()
		}
		if f := files["mem"]; f != nil {
			runtime.GC() // up-to-date in-use figures; alloc_space covers the whole scan
			errs = append(errs, pprof.Lookup("heap").WriteTo(f, 0))
		}
		if f := files["block"]; f != nil {
			errs = append(errs, pprof.Lookup("block").WriteTo(f, 0))
			runtime.SetBlockProfileRate(0)
		}
		for _, f := range files {
			errs = append(errs, f.Close())
		}
		return errors.Join(errs...)
	}, nil
}

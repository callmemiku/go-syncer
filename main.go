package main

import (
	"context"
	"flag"
	"sync"
	"time"
)

var wgMain sync.WaitGroup

func main() {

	source := flag.String("s", "", "Set a sync source directory absolute path.")
	target := flag.String("t", "", "Set a sync target directory absolute path.")
	interval := flag.Int64("i", 60, "Set a sync interval in seconds.")
	debug := flag.Bool("d", false, "Set debug output enabled.")

	flag.Parse()

	wgMain.Add(1)
	go daemon(
		Args{
			Source:   *source,
			Target:   *target,
			Interval: *interval,
			Debug:    *debug,
		},
	)
	wgMain.Wait()
}

func daemon(args Args) {
	defer wgMain.Done()
	wait := args.Interval
	for true {
		handleSync(args)
		time.Sleep(time.Duration(wait) * time.Second)
	}
}

func handleSync(args Args) {

	var wg sync.WaitGroup
	var sourceList []string
	var targetList []string

	wg.Add(2)

	go func() {
		defer wg.Done()
		listFiles(
			context.WithValue(
				context.Background(),
				"vars",
				vars{
					File: args.Source,
					List: &sourceList,
				},
			),
		)
	}()

	go func() {
		defer wg.Done()
		listFiles(
			context.WithValue(
				context.Background(),
				"vars",
				vars{
					File: args.Target,
					List: &targetList,
				},
			),
		)
	}()

	wg.Wait()

	process(
		context.WithValue(
			context.Background(),
			"arrays", processContext{
				SourceArray: &sourceList,
				TargetArray: &targetList,
				Source:      args.Source,
				Target:      args.Target,
				Debug:       args.Debug,
			},
		),
	)
}

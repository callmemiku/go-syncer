package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type vars struct {
	File string
	List *[]string
}

type processContext struct {
	SourceArray *[]string
	TargetArray *[]string
	Source      string
	Target      string
	Debug       bool
}

type fileVars struct {
	Action       Action
	Source       string
	Target       string
	ListToCopy   []string
	ListToDelete []string
	Debug        bool
}

type Args struct {
	Source   string
	Target   string
	Interval int64
	Debug    bool
}

var wgMain sync.WaitGroup

func main() {

	source := flag.String("s", "", "Set a sync source directory absolute path.")
	target := flag.String("t", "", "Set a sync target directory absolute path.")
	interval := flag.Int64("i", 60, "Set a sync interval in seconds.")
	debug := flag.Bool("d", false, "Set debug output enabled.")

	flag.Parse()

	wgMain.Add(1)
	go daemon(Args{
		Source:   *source,
		Target:   *target,
		Interval: *interval,
		Debug:    *debug,
	})
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

func process(ctx context.Context) {
	var wg sync.WaitGroup
	arrays := ctx.Value("arrays").(processContext)
	if arrays.Debug {
		fmt.Println("Current context: ")
		fmt.Println("\tSource files:")
		for _, file := range *arrays.SourceArray {
			fmt.Println("\t\t" + file)
		}
		fmt.Println("\tTarget files:")
		for _, file := range *arrays.TargetArray {
			fmt.Println("\t\t" + file)
		}
	}
	formattedTime := time.Now().Format("2006-01-02 15:04:05")
	fmt.Println(formattedTime + ": Looking for deletion list...")
	wg.Add(3)
	deleteList := difference(*arrays.TargetArray, *arrays.SourceArray, arrays.Target, arrays.Source)
	if len(deleteList) > 0 {
		fmt.Printf("Found %d files to delete...\n", len(deleteList))
		go func() {
			defer wg.Done()
			action(
				context.WithValue(
					context.Background(),
					"fileVars",
					fileVars{
						Action:       DELETE,
						Source:       arrays.Source,
						Target:       arrays.Target,
						ListToCopy:   nil,
						ListToDelete: deleteList,
						Debug:        arrays.Debug,
					},
				),
			)
		}()
	} else {
		wg.Done()
	}
	fmt.Println(formattedTime + ": Looking for copying list...")
	copyList := difference(*arrays.SourceArray, *arrays.TargetArray, arrays.Source, arrays.Target)
	if len(copyList) > 0 {
		fmt.Printf("Found %d files to copy...\n", len(copyList))
		go func() {
			defer wg.Done()
			action(
				context.WithValue(
					context.Background(),
					"fileVars",
					fileVars{
						Action:       COPY,
						Source:       arrays.Source,
						Target:       arrays.Target,
						ListToCopy:   copyList,
						ListToDelete: nil,
						Debug:        arrays.Debug,
					},
				),
			)
		}()
	} else {
		wg.Done()
	}
	add, del := resolvableDifferences(*arrays.SourceArray, *arrays.TargetArray, arrays.Source, arrays.Target)
	if len(add) > 0 {
		go func() {
			defer wg.Done()
			action(
				context.WithValue(
					context.Background(),
					"fileVars",
					fileVars{
						Action:       RESOLVE,
						Source:       arrays.Source,
						Target:       arrays.Target,
						ListToCopy:   add,
						ListToDelete: del,
						Debug:        arrays.Debug,
					},
				),
			)
		}()
	} else {
		wg.Done()
	}
	wg.Wait()
}

func listFiles(ctx context.Context) {
	variables := ctx.Value("vars").(vars)
	err := filepath.Walk(variables.File, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		*variables.List = append(*variables.List, path)
		return nil
	})

	if err != nil {
		log.Fatalf("error walking the path %v: %v\n", variables.List, err)
	}
}

func resolvableDifferences(original, copies []string, source, target string) ([]string, []string) {
	add := make([]string, 0)
	del := make([]string, 0)
	var diff string
	for _, file := range original {

		fi, err := os.Stat(file)
		if err != nil {
			return nil, nil
		}

		switch mode := fi.Mode(); {
		case mode.IsDir():
			continue
		}

		name := strings.TrimPrefix(file, source)
		found := false
		for _, cfile := range copies {
			cname := strings.TrimPrefix(cfile, target)
			if name == cname {
				diff = cfile
				found = true
				break
			}
		}

		if found {
			cfi, er := os.Stat(diff)
			if er != nil {
				return nil, nil
			}
			if fi.ModTime().After(cfi.ModTime()) {
				add = append(add, file)
				del = append(del, diff)
			}
		}
	}

	return add, del
}

func difference(original, copies []string, source, target string) []string {

	differences := make([]string, 0)

	for _, file := range original {

		fi, err := os.Stat(file)
		if err != nil {
			return nil
		}

		switch mode := fi.Mode(); {
		case mode.IsDir():
			continue
		}

		name := strings.TrimPrefix(file, source)
		found := false
		for _, cfile := range copies {
			cname := strings.TrimPrefix(cfile, target)
			if name == cname {
				found = true
				break
			}
		}

		if !found {
			differences = append(differences, file)
		}
	}

	sort.Slice(differences, func(i, j int) bool {
		return len(differences[i]) > len(differences[j])
	})

	return differences
}

func action(ctx context.Context) {
	variables := ctx.Value("fileVars").(fileVars)
	switch variables.Action {
	case DELETE:
		log.Printf("Deleting %d files...\n", len(variables.ListToDelete))
		err := deleteFiles(variables.ListToDelete, variables.Target, variables.Debug)
		if err != nil {
			log.Println(err)
		}
	case COPY:
		log.Printf("Copying %d files...\n", len(variables.ListToCopy))
		err := copyFiles(variables.ListToCopy, variables.Target, variables.Source, variables.Debug)
		if err != nil {
			log.Println(err)
		}
	case RESOLVE:
		log.Printf("Updating %d files...\n", len(variables.ListToCopy))
		err := deleteFiles(variables.ListToDelete, variables.Target, false)
		if err != nil {
			log.Println(err)
		}
		err = copyFiles(variables.ListToCopy, variables.Target, variables.Source, false)
		if err != nil {
			log.Println(err)
		}
		if variables.Debug {
			for _, s := range variables.ListToCopy {
				fmt.Println("Updated file: " + s)
			}
		}
	}
}

func deleteFiles(files []string, target string, debug bool) error {
	for _, file := range files {
		if debug {
			fmt.Println("Removing now: " + file)
		}
		err := os.Remove(file)
		if err != nil {
			return err
		}
		dir := strings.TrimSuffix(file, filepath.Base(file))
		if dir != target {
			empty, err := IsEmpty(dir)
			if err != nil {
				return err
			}
			if empty {
				err = os.Remove(dir)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func copyFiles(files []string, target, source string, debug bool) error {
	for _, file := range files {
		if debug {
			fmt.Println("Copying now: " + file)
		}
		_, err := copyFile(file, target, source)
		if err != nil {
			return err
		}
	}
	return nil
}

func IsEmpty(dir string) (bool, error) {
	f, err := os.Open(dir)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1) // Or f.Readdir(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err // Either not empty or error, suits both cases
}

func copyFile(src string, target, corePath string) (int64, error) {

	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file\n", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer func(source *os.File) {
		err := source.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(source)

	cleanName := strings.TrimPrefix(src, corePath)
	fullPath := target + cleanName
	pathNoName := strings.TrimSuffix(fullPath, filepath.Base(fullPath))
	err = os.MkdirAll(pathNoName, os.ModePerm)
	if err != nil {
		return 0, err
	}
	destination, err := os.Create(fullPath)
	if err != nil {
		return 0, err
	}
	defer func(destination *os.File) {
		err := destination.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(destination)
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}

type Action int

const (
	DELETE Action = iota
	COPY
	RESOLVE
)

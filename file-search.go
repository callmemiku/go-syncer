package main

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

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

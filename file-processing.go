package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

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

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
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

package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"hash/crc32"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

const (
	Version     = "1.0.0"
	ZlibVersion = "1.2.11"
)

type FileEntry struct {
	Filename string `json:"filename"`
	CRC      uint32 `json:"crc"`
}

type Data struct {
	Files       []FileEntry `json:"files"`
	Version     string      `json:"version"`
	ZlibVersion string      `json:"zlib_version"`
}

// Result struct to pass data back from workers
type jobResult struct {
	entry FileEntry
	err   error
}

var (
	outputFlag    string
	readFlag      bool
	recursiveFlag bool
	prettyFlag    bool
)

func main() {
	flag.StringVar(&outputFlag, "o", "", "save output json")
	flag.StringVar(&outputFlag, "output", "", "save output json")
	flag.BoolVar(&readFlag, "r", false, "reads a crc json file")
	flag.BoolVar(&readFlag, "read", false, "reads a crc json file")
	flag.BoolVar(&recursiveFlag, "R", false, "recursively scan folders")
	flag.BoolVar(&recursiveFlag, "recursive", false, "recursively scan folders")
	flag.BoolVar(&prettyFlag, "p", false, "pretty print output")
	flag.BoolVar(&prettyFlag, "pretty-output", false, "pretty print output")

	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Println("Usage: crcsum <path> [options]")
		flag.PrintDefaults()
		return
	}
	targetPath := flag.Arg(0)

	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		fmt.Printf("Error resolving path: %v\n", err)
		return
	}

	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		fmt.Printf("ERROR: %s not found.\n", absPath)
		return
	}

	realDirname := absPath
	if !info.IsDir() {
		realDirname = filepath.Dir(absPath)
	}

	if err := os.Chdir(realDirname); err != nil {
		fmt.Printf("Error changing directory: %v\n", err)
		return
	}

	// Read Mode remains serial as it just verifies a list
	if readFlag {
		fmt.Printf("reading crc file %s\n", targetPath)
		verifyCRC(absPath)
		return
	}

	// --- Scan/Create Mode ---

	// 1. Gather file paths
	var filesToProcess []string

	if recursiveFlag {
		err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				filesToProcess = append(filesToProcess, path)
			}
			return nil
		})
		if err != nil {
			fmt.Printf("Error walking directory: %v\n", err)
		}
	} else if !info.IsDir() {
		filesToProcess = []string{filepath.Base(absPath)}
	} else if info.IsDir() {
		entries, err := os.ReadDir(".")
		if err != nil {
			fmt.Printf("Error reading directory: %v\n", err)
			return
		}
		for _, e := range entries {
			if !e.IsDir() {
				filesToProcess = append(filesToProcess, e.Name())
			}
		}
	} else {
		fmt.Printf("unknown file type %s\n", targetPath)
		return
	}

	filesTotal := len(filesToProcess)
	if filesTotal == 0 {
		fmt.Println("No files found to process.")
		return
	}

	// 2. Setup Concurrency
	// Limit concurrency to GOMAXPROCS (usually num CPU cores) to avoid OS limits on open files
	maxWorkers := runtime.NumCPU()
	semaphore := make(chan struct{}, maxWorkers) // Buffered channel as a semaphore
	results := make(chan jobResult, filesTotal)
	var wg sync.WaitGroup

	// Atomic counter for progress reporting
	var processedCount int64

	fmt.Printf("Starting concurrent processing with %d workers...\n", maxWorkers)

	for _, path := range filesToProcess {
		wg.Add(1)
		
		// Acquire token
		semaphore <- struct{}{}

		go func(fPath string) {
			defer wg.Done()
			defer func() { <-semaphore }() // Release token

			// Double check it's a valid file
			fInfo, err := os.Stat(fPath)
			if err != nil || fInfo.IsDir() {
				results <- jobResult{err: fmt.Errorf("invalid file")}
				return
			}

			crc := calculateCRC32(fPath)
			
			// Increment progress safely
			newCount := atomic.AddInt64(&processedCount, 1)
			
			// Print progress (every 10 files or if < 100 total) to reduce console spam
			if filesTotal < 100 || newCount%10 == 0 || newCount == int64(filesTotal) {
				percent := (float64(newCount) / float64(filesTotal)) * 100
				fmt.Printf("Processed [%d / %d] (%.2f%%)\n", newCount, filesTotal, percent)
			}

			results <- jobResult{
				entry: FileEntry{Filename: fPath, CRC: crc},
			}
		}(path)
	}

	// Closer routine: wait for all workers, then close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// 3. Collect Results
	var processedFiles []FileEntry
	for res := range results {
		if res.err == nil {
			processedFiles = append(processedFiles, res.entry)
		}
	}

	// Sort results to ensure deterministic output (concurrency randomizes order)
	sort.Slice(processedFiles, func(i, j int) bool {
		return processedFiles[i].Filename < processedFiles[j].Filename
	})

	// 4. Output Logic (same as before)
	outputData := Data{
		Files:       processedFiles,
		Version:     Version,
		ZlibVersion: ZlibVersion,
	}

	var jsonData []byte
	if prettyFlag {
		jsonData, _ = json.MarshalIndent(outputData, "", "    ")
	} else {
		jsonData, _ = json.Marshal(outputData)
	}

	fmt.Println(string(jsonData))

	if outputFlag != "" {
		if _, err := os.Stat(outputFlag); err == nil {
			fmt.Printf("The file path %s already exists. Do you want to overwrite? (y/N) : ", outputFlag)
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))
			if response != "y" {
				fmt.Println("Cancelled.")
				return
			}
		}

		err := os.WriteFile(outputFlag, jsonData, 0644)
		if err != nil {
			fmt.Printf("Error writing output file: %v\n", err)
		}
	}
}

func calculateCRC32(filename string) uint32 {
	file, err := os.Open(filename)
	if err != nil {
		// Suppress error printing here to avoid race conditions on stdout
		return 0
	}
	defer file.Close()

	table := crc32.MakeTable(crc32.IEEE)
	hash := crc32.New(table)

	if _, err := io.Copy(hash, file); err != nil {
		return 0
	}

	return hash.Sum32()
}

func verifyCRC(jsonPath string) {
	file, err := os.Open(jsonPath)
	if err != nil {
		fmt.Printf("Error opening CRC file: %v\n", err)
		return
	}
	defer file.Close()

	byteValue, _ := io.ReadAll(file)

	var data Data
	if err := json.Unmarshal(byteValue, &data); err != nil {
		fmt.Printf("Error parsing JSON: %v\n", err)
		return
	}

	// Verify could also be concurrent, but verification is usually fast enough
	// unless checking massive files. Keeping it simple here.
	for _, fileEntry := range data.Files {
		currentCRC := calculateCRC32(fileEntry.Filename)
		status := "XX"
		if currentCRC == fileEntry.CRC {
			status = "OK"
		}
		fmt.Printf("%s : %d == %d : %s\n", status, fileEntry.CRC, currentCRC, fileEntry.Filename)
	}
}

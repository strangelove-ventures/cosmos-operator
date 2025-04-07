package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// CustomResourceDefinition represents a Kubernetes CRD structure.
type CustomResourceDefinition struct {
	APIVersion string      `yaml:"apiVersion"`
	Kind       string      `yaml:"kind"`
	Metadata   interface{} `yaml:"metadata"`
	Spec       struct {
		Group string `yaml:"group"`
		Names struct {
			Kind     string `yaml:"kind"`
			ListKind string `yaml:"listKind"`
			Plural   string `yaml:"plural"`
			Singular string `yaml:"singular"`
		} `yaml:"names"`
		Scope    string `yaml:"scope"`
		Versions []struct {
			AdditionalPrinterColumns []struct {
				JSONPath string `yaml:"jsonPath"`
				Name     string `yaml:"name"`
				Type     string `yaml:"type"`
			} `yaml:"additionalPrinterColumns,omitempty"`
			Name   string `yaml:"name"`
			Served bool   `yaml:"served"`
			Schema struct {
				OpenAPIV3Schema map[string]interface{} `yaml:"openAPIV3Schema"`
			} `yaml:"schema,omitempty"`
			Storage      bool `yaml:"storage"`
			Subresources struct {
				Status struct {
				} `yaml:"status"`
			} `yaml:"subresources"`
		} `yaml:"versions"`
	} `yaml:"spec"`
}

// FileInfo tracks information about processed files
type FileInfo struct {
	Path             string
	OriginalSize     int
	MinifiedSize     int
	DescriptionCount int
}

func main() {
	// Parse command-line arguments
	verbose := flag.Bool("v", false, "Enable verbose output for debugging")
	overwrite := flag.Bool("o", false, "Overwrite the original files with minified versions")
	flag.Parse()

	// Get the remaining arguments after flags
	args := flag.Args()

	if len(args) == 0 {
		fmt.Println("Please provide at least one file or directory path")
		os.Exit(1)
	}

	// Process all provided paths
	processedFiles := []FileInfo{}
	for _, path := range args {
		fileInfo, err := os.Stat(path)
		if err != nil {
			fmt.Printf("Error accessing path %s: %v\n", path, err)
			continue
		}

		if fileInfo.IsDir() {
			// Process directory
			dirFiles, err := processCRDsInDirectory(path, *verbose, *overwrite)
			if err != nil {
				fmt.Printf("Error processing directory %s: %v\n", path, err)
				continue
			}
			processedFiles = append(processedFiles, dirFiles...)
		} else {
			// Process single file
			fileResult, err := processCRDFile(path, *verbose, *overwrite)
			if err != nil {
				fmt.Printf("Error processing file %s: %v\n", path, err)
				continue
			}
			processedFiles = append(processedFiles, fileResult)
		}
	}

	// Check if any CRDs were processed
	if len(processedFiles) == 0 {
		fmt.Println("Error: No valid CRD files were found or processed")
		os.Exit(1)
	}

	// Print summary
	totalOriginal := 0
	totalMinified := 0
	totalDescriptions := 0

	fmt.Println("\nSummary of processed CRD files:")
	fmt.Println("-------------------------------")
	for _, file := range processedFiles {
		reductionPct := float64(file.OriginalSize-file.MinifiedSize) / float64(file.OriginalSize) * 100
		fmt.Printf("- %s: Removed %d descriptions, Size: %d → %d bytes (%.2f%% reduction)\n",
			file.Path, file.DescriptionCount, file.OriginalSize, file.MinifiedSize, reductionPct)

		totalOriginal += file.OriginalSize
		totalMinified += file.MinifiedSize
		totalDescriptions += file.DescriptionCount
	}

	totalReduction := float64(totalOriginal-totalMinified) / float64(totalOriginal) * 100
	fmt.Printf("\nTotal: Processed %d files, removed %d descriptions\n", len(processedFiles), totalDescriptions)
	fmt.Printf("Total size reduced from %d to %d bytes (%.2f%% reduction)\n",
		totalOriginal, totalMinified, totalReduction)
}

// processCRDsInDirectory finds and processes all CRD files in a directory
func processCRDsInDirectory(dirPath string, verbose, overwrite bool) ([]FileInfo, error) {
	var results []FileInfo

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only process YAML or YML files
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		// Try to process the file
		fileResult, err := processCRDFile(path, verbose, overwrite)
		if err == nil {
			results = append(results, fileResult)
		} else if verbose {
			fmt.Printf("Skipping non-CRD file: %s\n", path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no CRD files found in directory %s", dirPath)
	}

	return results, nil
}

// processCRDFile processes a single CRD file and returns file information
func processCRDFile(filePath string, verbose, overwrite bool) (FileInfo, error) {
	result := FileInfo{
		Path: filePath,
	}

	// Read the input file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return result, fmt.Errorf("error reading file: %v", err)
	}

	result.OriginalSize = len(data)

	// Parse the YAML
	var crd CustomResourceDefinition
	err = yaml.Unmarshal(data, &crd)
	if err != nil {
		return result, fmt.Errorf("error parsing YAML: %v", err)
	}

	// Only process if it's a CustomResourceDefinition
	if crd.Kind != "CustomResourceDefinition" {
		return result, fmt.Errorf("file is not a CustomResourceDefinition")
	}

	outputFile := filePath

	if !overwrite {
		// Generate the output file name
		ext := filepath.Ext(filePath)
		baseName := strings.TrimSuffix(filePath, ext)
		outputFile = baseName + ".min" + ext
	}

	// Process the schema to minify it by removing descriptions
	descriptionCount := 0
	if len(crd.Spec.Versions) > 0 && crd.Spec.Versions[0].Schema.OpenAPIV3Schema != nil {
		// Remove descriptions recursively
		removeDescriptions(crd.Spec.Versions[0].Schema.OpenAPIV3Schema, &descriptionCount, false)
	}

	result.DescriptionCount = descriptionCount

	// Marshal the modified CRD back to YAML
	modifiedData, err := yaml.Marshal(&crd)
	if err != nil {
		return result, fmt.Errorf("error generating YAML: %v", err)
	}

	result.MinifiedSize = len(modifiedData)

	// Write to the output file
	err = os.WriteFile(outputFile, modifiedData, 0644)
	if err != nil {
		return result, fmt.Errorf("error writing to file: %v", err)
	}

	if verbose {
		fmt.Printf("Successfully minified CRD %s and saved to %s\n", filePath, outputFile)
		fmt.Printf("Removed %d description fields\n", descriptionCount)
	}

	return result, nil
}

// removeDescriptions recursively removes all description fields from the schema
func removeDescriptions(node interface{}, count *int, shouldDelete bool) {
	// Handle map types (objects)
	if obj, ok := node.(map[string]interface{}); ok {
		// Remove description if it exists
		if shouldDelete {
			if _, exists := obj["description"]; exists {
				delete(obj, "description")
				*count++
			}
		}

		// Process all child elements
		for key, value := range obj {
			removeDescriptions(value, count, shouldDelete || shouldDeleteAllRecursiveDescriptions(key))
		}
	}

	// Handle slice types (arrays)
	if arr, ok := node.([]interface{}); ok {
		for _, item := range arr {
			removeDescriptions(item, count, shouldDelete)
		}
	}
}

func shouldDeleteAllRecursiveDescriptions(key string) bool {
	return slices.Contains([]string{"initContainers", "containers", "volumes"}, key)
}

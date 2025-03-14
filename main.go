package main

import (
    "archive/zip"
    "fmt"
    "io"
    "log"
    "os"
    "path/filepath"
    "strings"
)

func fixJarFile(inputPath, outputPath string) error {
    reader, err := zip.OpenReader(inputPath)
    if err != nil {
        return fmt.Errorf("failed to open input JAR file: %w", err)
    }
    defer reader.Close()

    outputFile, err := os.Create(outputPath)
    if err != nil {
        return fmt.Errorf("failed to create output JAR file: %w", err)
    }
    defer outputFile.Close()

    writer := zip.NewWriter(outputFile)
    defer writer.Close()

    seenDirs := make(map[string]bool)

    for _, f := range reader.File {
        name := strings.TrimSuffix(f.Name, "/") // Remove trailing slash if present (Python zipfile quirk)

        isClassFile := strings.HasSuffix(name, ".class")
        isDirectoryEntry := f.Mode().IsDir()
        isZeroSizeDir := isDirectoryEntry && f.UncompressedSize64 == 0

        // Force .class entries to be files
        if isClassFile {
            isDirectoryEntry = false
        }

        // Force non-zero size directory entries to be files
        if isDirectoryEntry && !isZeroSizeDir {
            isDirectoryEntry = false
        }

        // Handle directory entries
        if isDirectoryEntry {
            if !seenDirs[name] {
                header := &zip.FileHeader{
                    Name: name + "/", // Ensure directory entry has a trailing slash
                    Method: zip.Deflate, // Or zip.Store if you want no compression for dirs (usually not needed)
                }
                header.SetMode(os.ModeDir | 0755) // Explicitly set directory mode
                _, err := writer.CreateHeader(header)
                if err != nil {
                    return fmt.Errorf("failed to create directory entry '%s': %w", name, err)
                }
                seenDirs[name] = true
            }
            continue // Skip to the next file, directory entry is created
        }


        // Ensure parent directories exist for file entries
        dirPath := filepath.Dir(name)
        if dirPath != "." && dirPath != "" {
            dirsToAdd := make([]string, 0)
            currentDir := dirPath
            for currentDir != "." && currentDir != "" {
                if !seenDirs[currentDir] {
                    dirsToAdd = append([]string{currentDir}, dirsToAdd...) // Prepend to maintain order
                    seenDirs[currentDir] = true
                }
                currentDir = filepath.Dir(currentDir)
            }

            for _, dirToAdd := range dirsToAdd {
                header := &zip.FileHeader{
                    Name: dirToAdd + "/", // Ensure directory entry has a trailing slash
                    Method: zip.Deflate, // Or zip.Store
                }
                header.SetMode(os.ModeDir | 0755) // Explicitly set directory mode
                _, err := writer.CreateHeader(header)
                if err != nil {
                    return fmt.Errorf("failed to create directory entry '%s': %w", dirToAdd, err)
                }
            }
        }


        // Create file entry
        header := f.FileHeader
        header.Name = name // Use stripped name
        header.Flags &= ^uint16(0x0008) // Clear the directory flag (Bit 3) to force file
        header.Method = zip.Deflate // Or keep original if needed, but Deflate is common for JARs

        fileWriter, err := writer.CreateHeader(&header)
        if err != nil {
            return fmt.Errorf("failed to create file header for '%s': %w", name, err)
        }

        fileReader, err := f.Open()
        if err != nil {
            return fmt.Errorf("failed to open input file '%s': %w", name, err)
        }
        defer fileReader.Close()

        _, err = io.Copy(fileWriter, fileReader)
        if err != nil {
            return fmt.Errorf("failed to copy content for '%s': %w", name, err)
        }
    }

    return nil
}



func main() {
    if len(os.Args) != 3 {
        fmt.Println("Usage: go run fixjar.go <input.jar> <output.jar>")
        return
    }

    inputPath := os.Args[1]
    outputPath := os.Args[2]

    err := fixJarFile(inputPath, outputPath)
    if err != nil {
        log.Fatalf("Error fixing JAR file: %v", err)
    }

    fmt.Printf("Successfully fixed JAR file '%s' and saved as '%s'\n", inputPath, outputPath)
}
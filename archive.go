package main

import (
	"archive/zip"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

func zipDirectory(dir, destination string) error {
	file, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := zip.NewWriter(file)
	err = filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() {
			return walkErr
		}
		name, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		out, err := writer.Create(filepath.ToSlash(name))
		if err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		_, err = io.Copy(out, in)
		return err
	})

	closeErr := writer.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return file.Close()
}

func extractZip(source, destination string) error {
	reader, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer reader.Close()
	if err := os.MkdirAll(destination, 0700); err != nil {
		return err
	}
	root, err := os.OpenRoot(destination)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, entry := range reader.File {
		if entry.Mode()&os.ModeSymlink != 0 || !filepath.IsLocal(entry.Name) {
			return errors.New("ZIP contains an unsafe path")
		}
		if entry.FileInfo().IsDir() {
			if err := root.MkdirAll(entry.Name, 0700); err != nil {
				return err
			}
			continue
		}
		if err := root.MkdirAll(filepath.Dir(entry.Name), 0700); err != nil {
			return err
		}
		if err := extractEntry(root, entry); err != nil {
			return err
		}
	}
	return nil
}

func extractEntry(root *os.Root, entry *zip.File) error {
	in, err := entry.Open()
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := root.OpenFile(entry.Name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

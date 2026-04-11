package cli

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gote/src/data"
)

func ImportCommand(rawArgs []string) {
	args := ParseArgs(rawArgs)
	srcPath := args.First()
	if srcPath == "" {
		fmt.Println("Usage: gote import <file.tar.gz>")
		return
	}

	cfg, ui, ok := LoadConfigAndUI()
	if !ok {
		return
	}

	// Confirm if destination already has notes
	index, _ := data.LoadIndex()
	if len(index) > 0 {
		fmt.Printf("Destination already has %d notes. Overwrite? [y/n]: ", len(index))
		input := ui.ReadMenuInput()
		if input != "y" {
			ui.Info("Import cancelled.")
			return
		}
	}

	goteDir := data.GoteDir()
	noteDir := cfg.NoteDir // capture before extraction overwrites config

	f, err := os.Open(srcPath)
	if err != nil {
		ui.Error("could not open file: " + err.Error())
		return
	}
	defer f.Close()

	gr, err := gzip.NewReader(f)
	if err != nil {
		ui.Error("not a valid gzip archive: " + err.Error())
		return
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	noteCount := 0

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			ui.Error("error reading archive: " + err.Error())
			return
		}

		var destPath string
		switch {
		case strings.HasPrefix(hdr.Name, "notes/"):
			rel := strings.TrimPrefix(hdr.Name, "notes/")
			if rel == "" {
				continue
			}
			destPath = filepath.Join(noteDir, filepath.FromSlash(rel))
		case strings.HasPrefix(hdr.Name, "gote/"):
			rel := strings.TrimPrefix(hdr.Name, "gote/")
			if rel == "" {
				continue
			}
			destPath = filepath.Join(goteDir, filepath.FromSlash(rel))
		default:
			continue
		}

		if hdr.Typeflag == tar.TypeDir {
			os.MkdirAll(destPath, 0755)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			ui.Error("could not create directory: " + err.Error())
			return
		}

		out, err := os.Create(destPath)
		if err != nil {
			ui.Error("could not create file: " + err.Error())
			return
		}
		_, copyErr := io.Copy(out, tr)
		out.Close()
		if copyErr != nil {
			ui.Error("error writing file: " + copyErr.Error())
			return
		}

		if strings.HasPrefix(hdr.Name, "notes/") && strings.HasSuffix(hdr.Name, ".md") {
			noteCount++
		}
	}

	// Patch config: update NoteDir to destination path
	importedCfg, err := data.LoadConfig()
	oldNoteDir := importedCfg.NoteDir
	if err == nil && oldNoteDir != noteDir {
		importedCfg.NoteDir = noteDir
		data.SaveConfig(importedCfg)
	}

	// Patch index: update FilePaths if NoteDir changed
	if oldNoteDir != "" && oldNoteDir != noteDir {
		if idx, err := data.LoadIndex(); err == nil {
			for title, meta := range idx {
				if strings.HasPrefix(meta.FilePath, oldNoteDir) {
					meta.FilePath = noteDir + meta.FilePath[len(oldNoteDir):]
					idx[title] = meta
				}
			}
			data.SaveIndex(idx)
		}
	}

	ui.Success(fmt.Sprintf("Imported %d notes.", noteCount))
}

package cli

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"

	"gote/src/data"
)

func ExportCommand(rawArgs []string) {
	args := ParseArgs(rawArgs)
	cfg, ui, ok := LoadConfigAndUI()
	if !ok {
		return
	}

	outPath := args.First()
	if outPath == "" {
		outPath = "gote-export.tar.gz"
	}

	f, err := os.Create(outPath)
	if err != nil {
		ui.Error("could not create output file: " + err.Error())
		return
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	if err := addDirToTar(tw, data.GoteDir(), "gote"); err != nil {
		ui.Error("export failed: " + err.Error())
		return
	}

	if _, statErr := os.Stat(cfg.NoteDir); statErr == nil {
		if err := addDirToTar(tw, cfg.NoteDir, "notes"); err != nil {
			ui.Error("export failed: " + err.Error())
			return
		}
	}

	if err := tw.Close(); err != nil {
		ui.Error("export failed: " + err.Error())
		return
	}
	if err := gw.Close(); err != nil {
		ui.Error("export failed: " + err.Error())
		return
	}

	absPath, _ := filepath.Abs(outPath)
	ui.Success("Exported to " + absPath)
}

func addDirToTar(tw *tar.Writer, srcDir, prefix string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if relPath == "." {
			return nil
		}

		tarName := prefix + "/" + filepath.ToSlash(relPath)

		if info.IsDir() {
			return tw.WriteHeader(&tar.Header{
				Name:     tarName + "/",
				Typeflag: tar.TypeDir,
				Mode:     int64(info.Mode()),
				ModTime:  info.ModTime(),
			})
		}

		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = tarName

		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()

		_, err = io.Copy(tw, src)
		return err
	})
}

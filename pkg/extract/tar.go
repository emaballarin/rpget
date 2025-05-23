package extract

import (
	"archive/tar"
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/emaballarin/rpget/pkg/logging"
)

var ErrZipSlip = errors.New("archive (tar) file contains file outside of target directory")
var ErrEmptyHeaderName = errors.New("tar file contains entry with empty name")

type link struct {
	linkType byte
	oldName  string
	newName  string
}

func TarFile(r *bufio.Reader, destDir string, overwrite bool) error {
	var links []*link
	var reader io.Reader = r

	log := logging.GetLogger()

	startTime := time.Now()
	peekData, err := r.Peek(peekSize)
	if err != nil {
		return fmt.Errorf("error reading peek data: %w", err)
	}
	if decompressor := detectFormat(peekData); decompressor != nil {
		reader, err = decompressor.decompress(reader)
		if err != nil {
			return fmt.Errorf("error creating decompressed stream: %w", err)
		}
		log.Info().
			Str("decompressor", fmt.Sprintf("%T", decompressor)).
			Msg("Tar Compression Detected: Compression can significantly slowdown rpget (e.g. for model weights)")
	}
	tarReader := tar.NewReader(reader)
	logger := logging.GetLogger()

	logger.Debug().
		Str("extractor", "tar").
		Str("status", "starting").
		Msg("Extract")
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, header.Name)
		targetDir := filepath.Dir(target)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return err
		}

		if err := guardAgainstZipSlip(header, destDir); err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeXGlobalHeader:
			// This is a global pax header, which we can skip as it's mostly handled by the underlying implementation
			// NOTE: the global header is not persisted across subsequent calls to Next() and therefore could indicate
			// that we are processing a tar file in an unintended manner. This is a limitation of archive/tar.
			continue
		case tar.TypeDir:
			logger.Debug().
				Str("target", target).
				Str("perms", fmt.Sprintf("%o", header.Mode)).
				Msg("Tar: Directory")
			if err := os.MkdirAll(target, cleanFileMode(os.FileMode(header.Mode))); err != nil {
				return err
			}
		case tar.TypeReg:
			openFlags := os.O_CREATE | os.O_WRONLY
			if overwrite {
				openFlags |= os.O_TRUNC
			}
			logger.Debug().
				Str("target", target).
				Str("perms", fmt.Sprintf("%o", header.Mode)).
				Msg("Tar: File")
			targetFile, err := os.OpenFile(target, openFlags, cleanFileMode(os.FileMode(header.Mode)))
			if err != nil {
				return err
			}
			if _, err := io.Copy(targetFile, tarReader); err != nil {
				targetFile.Close()
				return err
			}
			if err := targetFile.Close(); err != nil {
				return fmt.Errorf("error closing file %s: %w", target, err)
			}
		case tar.TypeSymlink, tar.TypeLink:
			// Defer creation of
			logger.Debug().Str("link_type", string(header.Typeflag)).
				Str("old_name", header.Linkname).
				Str("new_name", target).
				Msg("Tar: (Defer) Link")
			links = append(links, &link{linkType: header.Typeflag, oldName: header.Linkname, newName: target})
		default:
			return fmt.Errorf("unsupported file type for %s, typeflag %s", header.Name, string(header.Typeflag))
		}
	}

	if err := createLinks(links, destDir, overwrite); err != nil {
		return fmt.Errorf("error creating links: %w", err)
	}

	// Read the rest of the bytes from the archive and verify they are all null bytes
	// This is for validation that the byte count is correct
	padding, err := io.ReadAll(r)
	if err != nil {
		return fmt.Errorf("error reading padding bytes: %w", err)
	}
	for _, b := range padding {
		if b != 0x00 {
			return fmt.Errorf("unexpected non-null byte in padding: %x", b)
		}
	}

	elapsed := time.Since(startTime).Seconds()
	logger.Debug().
		Str("extractor", "tar").
		Float64("elapsed_time", elapsed).
		Str("status", "complete").
		Msg("Extract")
	return nil
}

func createLinks(links []*link, destDir string, overwrite bool) error {
	logger := logging.GetLogger()
	for _, link := range links {
		targetDir := filepath.Dir(link.newName)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return err
		}
		switch link.linkType {
		case tar.TypeLink:
			oldPath := filepath.Join(destDir, link.oldName)
			logger.Debug().
				Str("old_path", oldPath).
				Str("new_path", link.newName).
				Msg("Tar: creating hard link")
			if err := createHardLink(oldPath, link.newName, overwrite); err != nil {
				return fmt.Errorf("error creating hard link from %s to %s: %w", oldPath, link.newName, err)
			}
		case tar.TypeSymlink:
			logger.Debug().
				Str("old_path", link.oldName).
				Str("new_path", link.newName).
				Msg("Tar: creating symlink")
			if err := createSymlink(link.oldName, link.newName, overwrite); err != nil {
				return fmt.Errorf("error creating symlink from %s to %s: %w", link.oldName, link.newName, err)
			}
		default:
			return fmt.Errorf("unsupported link type %s", string(link.linkType))
		}
	}
	return nil
}

func createHardLink(oldName, newName string, overwrite bool) error {
	if overwrite {
		err := os.Remove(newName)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("error removing existing file: %w", err)
		}
	}
	return os.Link(oldName, newName)
}

func createSymlink(oldName, newName string, overwrite bool) error {
	if overwrite {
		err := os.Remove(newName)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("error removing existing symlink/file: %w", err)
		}
	}
	return os.Symlink(oldName, newName)
}

func guardAgainstZipSlip(header *tar.Header, destDir string) error {
	if header.Name == "" {
		return ErrEmptyHeaderName
	}
	target, err := filepath.Abs(filepath.Join(destDir, header.Name))
	if err != nil {
		return fmt.Errorf("error getting absolute path of destDir %s: %w", header.Name, err)
	}
	destAbs, err := filepath.Abs(destDir)
	if err != nil {
		return fmt.Errorf("error getting absolute path of %s: %w", destDir, err)
	}
	if !strings.HasPrefix(target, destAbs) {
		return fmt.Errorf("%w: `%s` outside of `%s`", ErrZipSlip, target, destAbs)
	}
	return nil
}

func cleanFileMode(mode os.FileMode) os.FileMode {
	mask := os.ModeSticky | os.ModeSetuid | os.ModeSetgid
	return mode &^ mask
}

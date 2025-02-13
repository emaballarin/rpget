package multifile

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	netUrl "net/url"
	"os"
	"strings"

	"github.com/spf13/viper"

	rpget "github.com/emaballarin/rpget/pkg"
	"github.com/emaballarin/rpget/pkg/cli"
	"github.com/emaballarin/rpget/pkg/config"
	"github.com/emaballarin/rpget/pkg/logging"
)

// A manifest is a file consisting of pairs of URLs and paths:
//
// http://example.com/foo/bar.txt     foo/bar.txt
// http://example.com/foo/bar/baz.txt foo/bar/baz.txt
//
// A manifest may contain blank lines.
// The pairs are separated by arbitrary whitespace.
//
// When we parse a manifest, we group by URL base (ie scheme://hostname) so that
// all URLs that may share a connection are grouped.

var errDupeURLDestCombo = errors.New("duplicate destination with different URLs")

func manifestFile(manifestPath string) (*os.File, error) {
	if manifestPath == "-" {
		return os.Stdin, nil
	}
	if _, err := os.Stat(manifestPath); errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("manifest file %s does not exist", manifestPath)
	}
	file, err := os.Open(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("error opening manifest file %s: %w", manifestPath, err)
	}
	return file, err
}

func parseLine(line string) (url, dest string, err error) {
	fields := strings.Fields(line)
	if len(fields) != 2 {
		return "", "", fmt.Errorf("error parsing manifest invalid line format `%s`", line)
	}
	return fields[0], fields[1], nil
}

func checkSeenDestinations(destinations map[string]string, dest string, url string) error {
	if seenURL, ok := destinations[dest]; ok {
		if seenURL != url {
			return fmt.Errorf("duplicate destination %s with different urls: %s and %s", dest, seenURL, url)
		} else {
			return errDupeURLDestCombo
		}
	}
	return nil
}

func parseManifest(file io.Reader) (rpget.Manifest, error) {
	logger := logging.GetLogger()
	seenDestinations := make(map[string]string)
	manifest := make(rpget.Manifest, 0)

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		url, dest, err := parseLine(line)
		if err != nil {
			return nil, err
		}

		if _, err := netUrl.Parse(url); err != nil {
			return nil, err

		}

		// THIS IS A BODGE - FIX ME MOVE THESE THINGS TO RPGET
		// and make the consumer responsible for knowing if this
		// is allowed/not allowed/etc
		consumer := viper.GetString(config.OptOutputConsumer)
		if consumer != config.ConsumerNull {
			err = checkSeenDestinations(seenDestinations, dest, url)
			if err != nil {
				if errors.Is(err, errDupeURLDestCombo) {
					logger.Warn().
						Str("url", url).
						Str("destination", dest).
						Msg("Parse Manifest: Skip Duplicate URL/Destination")
					continue
				}
				return nil, err
			}
			seenDestinations[dest] = url

			err = cli.EnsureDestinationNotExist(dest)
			if err != nil {
				return nil, err
			}
		}
		manifest = manifest.AddEntry(url, dest)
	}

	return manifest, nil
}

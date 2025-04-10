# Rpget - Parallel File Downloader & Extractor

Rpget is a high performance, concurrent file downloader built in Go. It is designed to speed up and optimize file downloads from cloud storage services such as Amazon S3 and Google Cloud Storage.

The primary advantage of Rpget is its ability to download files in parallel using multiple threads. By dividing the file into chunks and downloading multiple chunks simultaneously, Rpget significantly reduces the total download time for large files.

If the downloaded file is a tar archive, Rpget can automatically extract the contents of the archive in memory, thus removing the need for an additional extraction step.

The efficiency of Rpget's tar extraction lies in its approach to handling data. Instead of writing the downloaded tar file to disk and then reading it back into memory for extraction, Rpget conducts the extraction directly from the in-memory download buffer. This method avoids unnecessary memory copies and disk I/O, leading to an increase in performance, especially when dealing with large tar files. This makes Rpget not just a parallel downloader, but also an efficient file extractor, providing a streamlined solution for fetching and unpacking files.

> [!NOTE]  
> This project is not related to [Code-Hex/rpget](https://github.com/Code-Hex/rpget). The two projects share the same name and similar goals, but are completely different codebases with different capabilities.

## Install  

You can download and install the latest release of Rpget directly from GitHub by running the following commands in a terminal:

```console
sudo curl -o /usr/local/bin/rpget -L "https://github.com/emaballarin/rrpget/releases/latest/download/rpget_$(uname -s)_$(uname -m)"
sudo chmod +x /usr/local/bin/rpget
```

If you're using macOS, you can install Rpget with Homebrew:

```console
brew tap replicate/tap
brew install replicate/tap/pget
```

Or you can build from source and install it with these commands
(requires Go 1.19 or later):

```console
make
sudo make install
```

This builds a static binary that can work inside containers.

## Usage

### Default Mode

    rpget <url> <dest> [-c concurrency] [-x]

#### Parameters

- \<url\>: The URL of the file to download.
- \<dest\>: The destination where the downloaded file will be stored.
- -c concurrency: The number of concurrent downloads. Default is 4 times the number of cores.
- -x: Extract the tar file after download. If not set, the downloaded file will be saved as is.

#### Default-Mode Command-Line Options

- `-x`, `--extract`
  - Extract archive after download
  - Type: `bool`
  - Default: `false`

#### Example

    rpget https://storage.googleapis.com/replicant-misc/sd15.tar ./sd15 -x

This command will download Stable Diffusion 1.5 weights to the path ./sd15 with high concurrency. After the file is downloaded, it will be automatically extracted.

### Multi-File Mode

    rpget multifile <manifest-file>

#### Parameters

- \<manifest-file\>: A path to a manifest file containing (new line delimited) pairs of URLs and local destination file paths. The use of `-` allows for reading from STDIN

#### Examples

Read the manifest file from a path on disk:

    rpget multifile /path/to/manifest.txt

Read the manifest file from STDIN:

    rpget multifile - < manifest.txt

Pipe to multifile form from another command:

    cat manifest.txt | rpget multifile -

An example `manifest.txt` file might look like this:

```txt
https://example.com/image1.jpg /local/path/to/image1.jpg
https://example.com/document.pdf /local/path/to/document.pdf
https://example.com/music.mp3 /local/path/to/music.mp3
```

#### Multi-file specific options

- `--max-concurrent-files`
  - Maximum number of files to download concurrently
  - Default: `40`
  - Type `Integer`
- `--max-conn-per-host`
  - Maximum number of (global) concurrent connections per host
  - Default: `40`
  - Type `Integer`

### Global Command-Line Options

- `--concurrency`
  - Maximum number of chunks to download in parallel for a given file
  - Type: `Integer`
  - Default: `4 * runtime.NumCPU()`
- `--connect-timeout`
  - Timeout for establishing a connection, format is <number><unit>, e.g. 10s
  - Type: `Duration`
  - Default: `5s`
- `-f`, `--force`
  - Force download, overwriting existing file
  - Type: `bool`
  - Default: `false`
- `--log-level`
  - Log level (debug, info, warn, error)
  - Type: `string`
  - Default: `info`
- `-m`, `--chunk-size string`
  - Chunk size (in bytes) to use when downloading a file (e.g. 10M)
  - Type: `string`
  - Default: `125M`
- `--resolve`
  - Resolve hostnames to specific IPs, can be specified multiple times, format <hostname>:<port>:<ip> (e.g. example.com:443:127.0.0.1)
  - Type: `string
- `-r`, `--retries`
  - Number of retries when attempting to retrieve a file
  - Type: `Integer`
  - Default: `5`
- `-v`, `--verbose`
  - Verbose mode (equivalent to `--log-level debug`)
  - Type: `bool`
  - Default: `false`

#### Deprecated

- `--max-chunks` (deprecated, use `--concurrency` instead)
  - Maximum number of chunks for downloading a given file
  - Type: `Integer`
  - Default: `4 * runtime.NumCPU()`
- `-m`, `--minimum-chunk-size string` (deprecated, use `--chunk-size` instead)
  - Minimum chunk size (in bytes) to use when downloading a file (e.g. 10M)
  - Type: `string`
  - Default: `16M`

## Error Handling

Rpget includes some error handling:

1. If a download any chunks fails, it will automatically retry up to 5 times before giving up.
2. If the downloaded file size does not match the expected size, it will also retry the download.

## Future Improvements

- as chunks are downloaded, start either writing to disk or extracting
- can we check the content hash of the file in the background?
- support for zip files?

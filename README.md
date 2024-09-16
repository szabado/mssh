# mssh

A tool for running commands over multiple servers via SSH in parallel, and easily collecting the results. This tool is based on 
Square's [mssh](https://github.com/square/mssh) but is written in Go instead of Ruby.

## Usage

```
Usage:
  mssh [command] [flags]

Flags:
  -c, --collapse             Collapse similar output.
  -d, --debug                Debug output (DEBUG level).
  -f, --file string          List of hostnames in a file (/dev/stdin for reading from stdin). Host names can be separated by commas or whitespace.
  -h, --help                 help for mssh
      --hosts string         Comma separated list of hostnames to execute on (format [user@]host[:port]). User defaults to the current user. Port defaults to 22.
  -m, --maxflight int        Maximum number of concurrent connections. (default 50)
  -t, --timeout int          How many seconds may each individual call take? 0 for no timeout. (default 60)
  -g, --timeout_global int   How many seconds for all calls to take? 0 for no timeout. (default 600)
  -v, --verbose              Verbose output (INFO level).
```

## Installation

### Homebrew
1. `brew tap szabado/tools`
2. `brew install szabado/tools/mssh`

### Build from source

This tool is built using [Hermit](https://github.com/cashapp/hermit), and fetches its own build tools as part of its build process. To build this from source:
1. Download the source.
2. Run `./bin/go build`
3. Put the resultant `mssh` binary in your `PATH`

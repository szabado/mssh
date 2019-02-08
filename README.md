# mssh

A tool for running multiple commands and ssh jobs in parallel, and easily collecting the results. This tool is based on 
Square's [tool of the same name](https://github.com/square/mssh) but is written in Go instead of Ruby.

## Usage

```
A tool for running multiple commands and ssh jobs in parallel, and easily collecting the results

Usage:
  mssh [command] [flags]

Flags:
  -c, --collapse             Collapse similar output.
  -d, --debug                Debug output (DEBUG level).
  -f, --file string          List of hostnames in a file (/dev/stdin for reading from stdin).
  -h, --help                 help for mssh
      --hosts string         Comma separated list of hostnames to execute on (format [user@]host[:port]). User defaults to the current user. Port defaults to 22.
  -m, --maxflight int        Maximum number of concurrent connections. (default 50)
  -t, --timeout int          How many seconds may each individual call take? 0 for no timeout. (default 60)
  -g, --timeout_global int   How many seconds for all calls to take? 0 for no timeout. (default 600)
  -v, --verbose              Verbose output (INFO level).
```

## Installing

You can do it manually, or you can install it from ~~my homebrew tap~~ (coming soon).  It requires go 1.11 as it uses
modules to manage its dependencies.

## TODOs

Not all of the flags are functional yet:
- `--timeout`, `--timeout_global`: don't work at all.
- `--range`: not even present in the codebase yet. I need to port the logic over from [the original](https://github.com/square/rangeclient).

Ping me or open an Issue if you actually need some of them implemented.



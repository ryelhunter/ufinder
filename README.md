# UFinder

UFinder is a Go-based URL discovery and aggregation tool for security researchers and bug bounty hunters. It orchestrates multiple URL collection tools, stores each tool output separately, and keeps a deduplicated master list for incremental recon runs.

## Features

- Run multiple URL discovery tools from a single command
- Execute both `gau` and `gau --subs` by default
- Deduplicate all collected URLs into a single `urls.txt`
- Preserve per-tool output files for comparison and follow-up analysis
- Re-run scans incrementally and highlight newly discovered URLs
- Process either a single domain or a `.txt` file with one target per line

## Supported Tools

- `waymore`
- `waybackurls`
- `gau`
- `gau --subs`
- `xurlfind3r`
- `urlscan`
- `urlfinder`
- `ducker`

## Requirements

- Go 1.16+
- Python 3.x
- `jq`
- The integrated Go and Python tools listed below

## Installation

```bash
git clone https://github.com/yourusername/ufinder.git
cd ufinder
go build -o ufinder
chmod +x ufinder
sudo mv ufinder /usr/local/bin/
```

## Install Go Tools

```bash
go install github.com/lc/gau/v2/cmd/gau@latest
go install github.com/tomnomnom/waybackurls@latest
go install github.com/hueristiq/xurlfind3r/cmd/xurlfind3r@latest
go install github.com/projectdiscovery/urlfinder/cmd/urlfinder@latest
go install github.com/gilsgil/ducker@latest
```

## Install Python Tools

```bash
pip install waymore
```

## Install jq

### macOS

```bash
brew install jq
```

### Ubuntu/Debian

```bash
sudo apt install jq
```

## Install chromedriver

### macOS

```bash
brew install chromedriver
```

### Ubuntu/Debian

```bash
sudo apt install chromedriver
```

## Add Go Binaries to PATH

### zsh

```bash
echo 'export PATH=$PATH:$HOME/go/bin' >> ~/.zshrc
source ~/.zshrc
```

### bash

```bash
echo 'export PATH=$PATH:$HOME/go/bin' >> ~/.bashrc
source ~/.bashrc
```

## Verify Installation

```bash
which gau
which waybackurls
which xurlfind3r
which urlfinder
which waymore
which jq
which ducker
chromedriver --version
```

## Usage

### Single target

```bash
ufinder -d example.com -f output_directory
```

### Target list

```bash
ufinder -l targets.txt -f batch_output
```

`targets.txt` should contain one target per line, such as domains or subdomains. Empty lines, duplicate entries, and lines starting with `#` are ignored.

## Command Line Options

| Flag | Description |
|------|-------------|
| `-d` | Single target domain |
| `-l` | Text file containing one target per line |
| `-f` | Output folder |
| `-m`, `--merge-targets` | When using `-l`, write all targets into the same output folder |
| `-t` | Comma-separated tool list, for example `waymore,gau,gau_subs,urlscan` |
| `-j`, `--extract-js` | Extract JavaScript URLs from `urls.txt` into `js.txt` |
| `-q`, `--quiet` | Hide the banner and the final per-URL list of new findings |
| `-v` | Verbose mode |

Use either `-d` or `-l`.

## Examples

Run all tools against one target:

```bash
ufinder -d example.com -f example_recon
```

Run selected tools only:

```bash
ufinder -d example.com -f example_recon -t gau,gau_subs,urlscan
```

Run a batch from a text file:

```bash
ufinder -l targets.txt -f batch_recon
```

Merge all targets from a list into the same output folder:

```bash
ufinder -l targets.txt -f . -m
```

Merge all targets into the current directory, run in quiet mode, and extract JavaScript URLs:

```bash
ufinder -l targets.txt -f . -q -j -m
```

Run in quiet mode:

```bash
ufinder -d example.com -f example_recon -q
```

Extract JavaScript URLs after aggregation:

```bash
ufinder -d example.com -f example_recon -j
```

Use both shortcuts together:

```bash
ufinder -d example.com -f example_recon -q -j
```

## Output Structure

### Single target

```text
output_directory/
└── endpoints/
    ├── urls.txt                # Master file with all unique URLs
    ├── js.txt                  # JavaScript URLs extracted from urls.txt
    ├── waymore.txt             # URLs found by waymore
    ├── waybackurls.txt         # URLs found by waybackurls
    ├── gau.txt                 # URLs found by gau
    ├── gau_subs.txt            # URLs found by gau --subs
    ├── xurlfind3r.txt          # URLs found by xurlfind3r
    ├── urlscan.txt             # URLs found by urlscan
    ├── urlfinder.txt           # URLs found by urlfinder
    ├── ducker.txt              # URLs found by ducker
    └── last_results.txt        # New URLs found in the latest run
```

### Target list

```text
batch_output/
├── example.com/
│   └── endpoints/
│       └── ...
└── api.example.com/
    └── endpoints/
        └── ...
```

### Target list with merged output

```text
output_directory/
└── endpoints/
    ├── urls.txt                # Master file with all unique URLs
    ├── js.txt                  # JavaScript URLs extracted from urls.txt
    ├── waymore.txt             # URLs found by waymore
    ├── waybackurls.txt         # URLs found by waybackurls
    ├── gau.txt                 # URLs found by gau
    ├── gau_subs.txt            # URLs found by gau --subs
    ├── xurlfind3r.txt          # URLs found by xurlfind3r
    ├── urlscan.txt             # URLs found by urlscan
    ├── urlfinder.txt           # URLs found by urlfinder
    ├── ducker.txt              # URLs found by ducker
    └── last_results.txt        # New URLs found in the latest run
```

## Environment Variables

For URLScan.io integration, set your API key before running `urlscan`:

```bash
export URLSCAN="your_urlscan_api_key"
```

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE) for details.

## Acknowledgments

- Created by Gilson Oliveira
- Thanks to the developers of all integrated tools

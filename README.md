# UFinder

UFinder is a powerful Go-based URL discovery and aggregation tool designed for security researchers and bug bounty hunters. It combines and orchestrates multiple URL discovery tools to find web endpoints efficiently, eliminate duplicates, and provide a comprehensive view of a target's attack surface.

## Features

- **Multi-tool Orchestration**: Run multiple URL discovery tools from a single command
- **Dual GAU Coverage**: Execute both `gau` and `gau --subs` by default
- **Automatic Deduplication**: Filter and maintain unique URL collections in `urls.txt`
- **Incremental Discovery**: Track new URLs discovered across multiple scans
- **Batch Resume Support**: Resume target list runs automatically from the last completed target
- **Comparative Analysis**: Preserve per-tool output files for comparison and follow-up analysis
- **Flexible Target Input**: Process either a single domain or a `.txt` file with one target per line
- **Organized Output**: Results are saved in a structured directory format

## Supported Tools

- **Waymore**: Discover URLs from the Wayback Machine
- **Waybackurls**: Extract URLs from the Wayback Machine archive
- **GAU**: Get All URLs from various sources
- **GAU --subs**: Extend GAU discovery to include subdomains
- **XURLFinder**: Advanced URL discovery with subdomain support
- **URLScan**: Retrieve URLs from the URLScan.io API
- **URLFinder**: Find URLs using custom patterns
- **Ducker**: Extract URLs from search engine results

## Requirements

- Go 1.16+
- The integrated tools (`waymore`, `waybackurls`, `gau`, `xurlfind3r`, `urlfinder`)
- Python 3.x
- `jq` (for URLScan results processing)

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
| `-d`, `--domain` | Single target domain |
| `-l`, `--list` | Text file containing one target per line |
| `-J`, `--extract-js-from` | Read an existing URLs file and generate `js.txt` and `js_unique.txt` in the same directory |
| `-f`, `--output` | Output folder |
| `-m`, `--merge-targets` | When using `-l`, write all targets into the same output folder |
| `-t`, `--tools` | Comma-separated tool list, for example `waymore,gau,gau_subs,urlscan` |
| `-r`, `--resume` | Resume a `-l` batch using `.ufinder-resume.json` from the output folder |
| `-R`, `--restart` | Restart a `-l` batch from the beginning and reset `.ufinder-resume.json` |
| `-s`, `--extract-subdomains` | Extract discovered subdomains into `subdomains.txt` and newly discovered ones into `subdomains_new.txt` |
| `-u`, `--extract-normalized-urls` | Extract normalized unique URLs into `urls_unique.txt` |
| `-j`, `--extract-js` | Extract JavaScript URLs into `js.txt` and normalized entries into `js_unique.txt` |
| `-q`, `--quiet` | Hide the banner and the final per-URL list of new findings |
| `-v`, `--verbose` | Verbose mode |

Use either `-d`, `-l`, or `-J`.

## Examples

Run all tools against one target:

```bash
ufinder -d example.com -f example_recon
```

Run selected tools only:

```bash
ufinder -d example.com -f example_recon -t waymore,gau,gau_subs,urlscan
```

Run a batch from a text file:

```bash
ufinder -l targets.txt -f batch_recon
```

Generate `js.txt` and `js_unique.txt` later from an existing URLs file:

```bash
ufinder -J batch_recon/endpoints/urls.txt
```

Resume a previous batch from the same output folder:

```bash
ufinder -l targets.txt -f batch_recon -r
```

Resume a previous batch using the long flag:

```bash
ufinder -l targets.txt -f batch_recon --resume
```

Restart a batch from the beginning and reset the saved state:

```bash
ufinder -l targets.txt -f batch_recon -R
```

Restart a batch using the long flag:

```bash
ufinder -l targets.txt -f batch_recon --restart
```

Merge all targets from a list into the same output folder:

```bash
ufinder -l targets.txt -f . -m
```

Merge all targets into the current directory, run in quiet mode, and extract JavaScript URLs:

```bash
ufinder -l targets.txt -f . -q -s -j -u -m
```

Run in quiet mode:

```bash
ufinder -d example.com -f example_recon -q
```

Extract JavaScript URLs after aggregation:

```bash
ufinder -d example.com -f example_recon -j
```

Extract JavaScript URLs from a previously generated `urls.txt` without rerunning discovery:

```bash
ufinder --extract-js-from example_recon/endpoints/urls.txt
```

Extract normalized unique URLs after aggregation:

```bash
ufinder -d example.com -f example_recon -u
```

Extract discovered subdomains after aggregation:

```bash
ufinder -l targets.txt -f batch_recon -s
```

Use both shortcuts together:

```bash
ufinder -d example.com -f example_recon -q -j -s
```

## Resume Behavior

When you run `ufinder` with `-l` and `-r`, it stores batch progress in the output folder root as `.ufinder-resume.json`.

- Targets are marked as completed only after the full processing for that target finishes.
- If the machine stops in the middle of a target, that target is processed again on the next `-r` run.
- If the resume file does not exist yet, `ufinder` starts fresh and creates it automatically.
- Running with `-R` resets the saved progress file and starts the batch from the first target again.
- Running without `-r` processes the full list again, even if the resume file already exists.

### Practical Flow

Start a batch normally:

```bash
ufinder -l targets.txt -f batch_recon
```

Continue that same batch later from the same output folder:

```bash
ufinder -l targets.txt -f batch_recon --resume
```

Reset the saved progress and start that same batch again from the beginning:

```bash
ufinder -l targets.txt -f batch_recon --restart
```

## Output Structure

### Single target

```text
output_directory/
└── endpoints/
    ├── urls.txt                # Master file with all unique URLs
    ├── subdomains.txt          # Discovered subdomains extracted from urls.txt
    ├── subdomains_new.txt      # Discovered subdomains that were not part of the input target set
    ├── urls_unique.txt         # Normalized URLs without scheme, query strings, fragments, or default ports
    ├── js.txt                  # JavaScript URLs extracted from urls.txt, including query-string variants
    ├── js_unique.txt           # Normalized JavaScript URLs without query strings or fragments
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
├── .ufinder-resume.json
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
├── .ufinder-resume.json
└── endpoints/
    ├── urls.txt                # Master file with all unique URLs
    ├── subdomains.txt          # Discovered subdomains extracted from urls.txt
    ├── subdomains_new.txt      # Discovered subdomains that were not part of the input target set
    ├── urls_unique.txt         # Normalized URLs without scheme, query strings, fragments, or default ports
    ├── js.txt                  # JavaScript URLs extracted from urls.txt, including query-string variants
    ├── js_unique.txt           # Normalized JavaScript URLs without query strings or fragments
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

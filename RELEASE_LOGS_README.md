# Azure Storage Fuse Release Logs

This directory contains tools to view and analyze release logs for Azure Storage Fuse (Blobfuse2).

## Quick Start

The simplest way to view release logs is using the `release-logs` script:

```bash
# Show latest 5 releases
./release-logs

# Show latest 10 releases  
./release-logs latest 10

# Show all releases
./release-logs all

# Show specific version
./release-logs version 2.5.0

# Show in table format
./release-logs table
```

## Tools Available

### 1. `release-logs` (Bash Script)
A convenient wrapper script that provides common use cases with simple commands.

**Usage:**
```bash
./release-logs [COMMAND] [OPTIONS]
```

**Commands:**
- `latest [N]` - Show latest N releases (default: 5)
- `all` - Show all releases
- `version VERSION` - Show specific version (e.g., 2.5.0)
- `table [N]` - Show releases in table format (default: 10)
- `markdown [N]` - Show releases in markdown format (default: 10)
- `json [N]` - Show releases in JSON format (default: 10)

### 2. `show_release_logs.py` (Python Script)
A comprehensive Python script with advanced options for viewing release logs.

**Usage:**
```bash
python3 show_release_logs.py [OPTIONS]
```

**Options:**
- `--format {table,json,markdown,detailed}` - Output format (default: detailed)
- `--source {changelog,github,both}` - Data source (default: both)
- `--version VERSION` - Filter by specific version
- `--limit N` - Limit to N most recent releases (default: 10)
- `--include-assets` - Include download assets information (GitHub only)
- `--include-stats` - Include download statistics (GitHub only)

## Examples

### View Latest Releases
```bash
# Default - show latest 5 releases
./release-logs

# Show latest 10 releases with detailed information
./release-logs latest 10
```

### Filter by Version
```bash
# Show all 2.5.x releases
./release-logs version 2.5.0

# Show specific version with Python script
python3 show_release_logs.py --version 2.4.2
```

### Different Output Formats

#### Table Format
```bash
./release-logs table 5
```
Output:
```
Version              Date         Features   Bug Fixes  Downloads   
--------------------------------------------------------------------------
2.6.0                Unreleased   0          1          N/A         
2.5.0~preview.1      2025-04-30   2          1          N/A         
2.5.0                2025-07-17   0          7          N/A         
```

#### Markdown Format
```bash
./release-logs markdown 3
```
Output:
```markdown
# Azure Storage Fuse Release Logs

## Version 2.6.0
**Released:** Unreleased

### Bug Fixes
- Fail file open operation if the file being downloaded by file-cache...
```

#### JSON Format
```bash
./release-logs json 2
```
Output:
```json
[
  {
    "version": "2.6.0",
    "date": "Unreleased",
    "features": [],
    "bug_fixes": ["Fail file open operation..."],
    "other_changes": [],
    "download_count": 0,
    "assets": []
  }
]
```

### Advanced Usage with Python Script

#### Include GitHub Release Statistics
```bash
python3 show_release_logs.py --include-stats --include-assets --limit 5
```

#### Use Only Changelog Data
```bash
python3 show_release_logs.py --source changelog --format detailed
```

#### Get All Releases in JSON
```bash
python3 show_release_logs.py --source changelog --format json > releases.json
```

## Data Sources

### 1. CHANGELOG.md
- Structured release notes maintained by the development team
- Contains features, bug fixes, and other changes
- Always available and reliable
- Source of truth for release content

### 2. GitHub Releases API
- Official GitHub releases with download statistics
- Includes binary assets and download counts
- May have rate limiting in some environments
- Provides additional metadata like publication dates

## Output Information

Each release entry includes:

- **Version**: Release version number (e.g., 2.5.0, 2.5.0~preview.1)
- **Date**: Release date or "Unreleased" for upcoming versions
- **Features**: New features and capabilities added
- **Bug Fixes**: Issues resolved in this release
- **Other Changes**: Additional changes, optimizations, and updates
- **Download Statistics**: (GitHub only) Total downloads and asset information

## Installation Requirements

- **Python 3.6+** for the Python script
- **Bash** for the wrapper script
- **Internet connection** (optional, for GitHub API access)

## Troubleshooting

### GitHub API Rate Limiting
If you see "HTTP Error 403: Forbidden" when fetching GitHub releases:
- Use `--source changelog` to only use the CHANGELOG.md file
- Or use the bash wrapper which defaults to changelog-only mode

### Version Filtering
Version filtering supports partial matches:
- `--version 2.5` will match both `2.5.0` and `2.5.0~preview.1`
- `--version preview` will match all preview releases

### Python Script Not Found
Make sure you're running the script from the correct directory:
```bash
cd /path/to/azure-storage-fuse
python3 show_release_logs.py
```

## Contributing

To add new features or fix issues:

1. Modify the Python script for core functionality
2. Update the bash wrapper if needed for new commands
3. Update this README with new examples
4. Test with various options and formats

## License

These scripts are part of the Azure Storage Fuse project and follow the same MIT license.
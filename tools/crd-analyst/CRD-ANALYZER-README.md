# CRD API Version Analyzer

This tool analyzes Kubernetes Custom Resource Definitions (CRDs) and their API versions across different releases of the NNF (Near Node Flash) project.

## Features

- **Analyze CRDs** for a single release
- **Compare CRDs** between two releases
- **Multiple output formats**: JSON, human-readable summaries, Markdown reports
- **Detailed change analysis**: Shows added/removed CRDs and API version changes
- **Served/Storage status**: Identifies which API versions are served and which is the storage version

## Setup

Run the setup script to install dependencies and activate the virtual environment:

```bash
source tools/crd-analyst/setup-crd-analyzer.sh
```

This will:

- Create a Python virtual environment (if it doesn't exist)
- Install required dependencies (PyYAML)
- Activate the virtual environment in your current shell

**Alternative:** If you prefer to set up without activating:

```bash
./tools/crd-analyst/setup-crd-analyzer.sh
source tools/crd-analyst/venv/bin/activate
```

## Usage

### Analyze a Single Release

```bash
./tools/crd-analyst/crd-analyzer.py analyze v0.1.23
```

This will:

- Download the release manifests (if not already present)
- Scan for all CRD files
- Extract API version information
- Generate JSON and human-readable reports

### Compare Two Releases

```bash
./tools/crd-analyst/crd-analyzer.py compare v0.1.20 v0.1.23
```

This will:

- Analyze both releases
- Generate detailed comparison showing:
  - Added/removed CRDs
  - API version changes
  - Served/storage status changes

### Options

- `-w, --workspace`: Specify working directory (default: `workingspace`)
- `-u, --url`: Specify NNF repository URL (default: `git@github.com:NearNodeFlash/nnf-deploy.git`)

## Output Files

### Single Release Analysis

- `crds-{version}.json`: Machine-readable CRD data
- `crds-{version}-summary.txt`: Human-readable summary organized by API group

### Comparison

- `crd-comparison-{v1}-to-{v2}.md`: Detailed Markdown comparison report

## Example Output

### Terminal Summary

```shell
=== COMPARISON SUMMARY ===
  Added CRDs: 0
  Removed CRDs: 0
  Changed CRDs: 2

  Changed:
    ~ clientmounts.dataworkflowservices.github.io
    ~ nnfaccesses.nnf.cray.hpe.com
```

### CRD Summary Format

```shell
Group: nnf.cray.hpe.com
----------------------------------------
  nnfstorageprofiles.nnf.cray.hpe.com
    Scope: Namespaced
    File: v0.1.23/nnf-sos/nnf-sos-crds.yaml
    API Versions:
      v1alpha6        served: ❌  storage:   
      v1alpha7        served: ✅  storage:   
      v1alpha8        served: ✅  storage: 📦
```

## Requirements

- Python 3.x
- GitHub CLI (`gh`) - for downloading releases
- `tar` - for extracting manifests
- PyYAML - automatically installed by setup script

## Troubleshooting

### Missing Dependencies

If you see errors about missing tools:

```bash
# Install GitHub CLI
brew install gh

# Authenticate with GitHub
gh auth login
```

### Permission Errors

Make sure the scripts are executable:

```bash
chmod +x tools/crd-analyst/crd-analyzer.py
chmod +x tools/crd-analyst/setup-crd-analyzer.sh
```

### Python Environment Issues

If you encounter Python environment issues, try:

```bash
# Clean up and recreate virtual environment
rm -rf tools/crd-analyst/venv
./tools/crd-analyst/setup-crd-analyzer.sh
```

## Understanding the Output

### API Version Status Icons

- ✅ `served: true` - This API version is accessible via the Kubernetes API
- ❌ `served: false` - This API version is not accessible (deprecated/disabled)
- 📦 `storage: true` - This is the version used for storing objects in etcd
- (empty) `storage: false` - Not the storage version

### Version Changes

- ➕ Added version - New API version introduced
- ➖ Removed version - API version no longer defined
- 🔄 Changed status - Served or storage status changed

This information is crucial for understanding API compatibility and upgrade paths between NNF releases.

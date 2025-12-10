#!/usr/bin/env python3

# Copyright 2025 Hewlett Packard Enterprise Development LP
# Other additional copyright holders may be indicated within.
#
# The entirety of this work is licensed under the Apache License,
# Version 2.0 (the "License"); you may not use this file except
# in compliance with the License.
#
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""
CRD API Version Analyzer and Comparator

This tool analyzes Kubernetes Custom Resource Definitions (CRDs) and their
API versions across different releases of the NNF project.
"""

import argparse
import json
import os
import subprocess
import sys
import tempfile
import yaml
from collections import defaultdict
from pathlib import Path
from typing import Dict, List, Tuple, Optional

class Colors:
    """ANSI color codes for terminal output"""
    BOLD = '\033[1m'
    RED = '\033[91m'
    GREEN = '\033[92m'
    YELLOW = '\033[93m'
    BLUE = '\033[94m'
    MAGENTA = '\033[95m'
    CYAN = '\033[96m'
    RESET = '\033[0m'

def msg(text: str, color: str = Colors.BOLD):
    """Print a styled message"""
    print(f"{color}{text}{Colors.RESET}")

def do_fail(text: str):
    """Print error and exit"""
    msg(text, Colors.RED)
    sys.exit(1)

class CRDAnalyzer:
    def __init__(self, workspace: str = "workingspace", nnf_url: str = "git@github.com:NearNodeFlash/nnf-deploy.git"):
        self.workspace = Path(workspace)
        self.nnf_url = nnf_url
        self.workspace.mkdir(exist_ok=True)
    
    def check_dependencies(self):
        """Check if required tools are available"""
        required_tools = ["gh", "tar"]
        
        for tool in required_tools:
            if not self._command_exists(tool):
                do_fail(f"Required tool '{tool}' is not installed or not in PATH")
        
        # Check if we can import yaml
        try:
            import yaml
        except ImportError:
            do_fail("Python 'pyyaml' package is required. Install with: pip install pyyaml")
    
    def _command_exists(self, command: str) -> bool:
        """Check if a command exists in PATH"""
        try:
            subprocess.run([command, "--help"], capture_output=True, check=False)
            return True
        except FileNotFoundError:
            return False
    
    def prep_for_manifest(self, version: str) -> Tuple[Path, str]:
        """Prepare workspace for manifest download"""
        version_dir = self.workspace / version
        tar_file = f"manifests-{version}.tar"
        tar_path = self.workspace / tar_file
        
        # Clean up existing files
        if tar_path.exists():
            tar_path.unlink()
        
        if version_dir.exists():
            import shutil
            shutil.rmtree(version_dir)
        
        # Check if release exists
        result = subprocess.run(
            ["gh", "release", "view", version],
            capture_output=True,
            text=True
        )
        
        if result.returncode != 0:
            do_fail(f"Release {version} does not exist")
        
        return version_dir, tar_file
    
    def fetch_and_unpack_manifest(self, version: str, tar_file: str) -> Path:
        """Download and extract manifest for a version"""
        version_dir = self.workspace / version
        
        msg(f"Downloading manifests for {version}...")
        
        # Download manifests.tar
        result = subprocess.run([
            "gh", "release", "download", 
            "-R", self.nnf_url,
            "-O", tar_file,
            "-p", "manifests.tar",
            version
        ], cwd=self.workspace, capture_output=True, text=True)
        
        if result.returncode != 0:
            do_fail(f"Unable to find manifests.tar for {version}: {result.stderr}")
        
        # Create directory and extract
        version_dir.mkdir()
        result = subprocess.run([
            "tar", "xfo", f"../{tar_file}"
        ], cwd=version_dir, capture_output=True, text=True)
        
        if result.returncode != 0:
            do_fail(f"Unable to extract tar {tar_file}: {result.stderr}")
        
        msg(f"Extracted manifests for {version}")
        return version_dir
    
    def download_manifest(self, version: str) -> Path:
        """Complete workflow to download a manifest"""
        version_dir, tar_file = self.prep_for_manifest(version)
        return self.fetch_and_unpack_manifest(version, tar_file)
    
    def find_crd_files(self, release_dir: Path) -> List[Path]:
        """Find all CRD YAML files in a release directory"""
        msg(f"Scanning for CRD files in {release_dir.name}...")
        crd_files = []
        
        # Look for YAML files
        for pattern in ["*.yaml", "*.yml"]:
            for yaml_file in release_dir.rglob(pattern):
                try:
                    with open(yaml_file, 'r', encoding='utf-8') as f:
                        content = f.read()
                        
                    # Quick check for CRD without full parsing
                    if 'kind: CustomResourceDefinition' in content:
                        # Now parse to confirm
                        try:
                            docs = list(yaml.safe_load_all(content))
                            for doc in docs:
                                if doc and doc.get('kind') == 'CustomResourceDefinition':
                                    crd_files.append(yaml_file)
                                    break
                        except yaml.YAMLError as e:
                            msg(f"Warning: YAML parsing error in {yaml_file}: {e}", Colors.YELLOW)
                            
                except Exception as e:
                    msg(f"Warning: Could not read {yaml_file}: {e}", Colors.YELLOW)
                    continue
        
        msg(f"Found {len(crd_files)} CRD files", Colors.GREEN)
        return crd_files
    
    def extract_crd_info(self, crd_file: Path) -> List[Dict]:
        """Extract CRD information including API versions"""
        crds = []
        try:
            with open(crd_file, 'r', encoding='utf-8') as f:
                docs = list(yaml.safe_load_all(f))
                
            for doc in docs:
                if doc and doc.get('kind') == 'CustomResourceDefinition':
                    metadata = doc.get('metadata', {})
                    crd_name = metadata.get('name', 'unknown')
                    spec = doc.get('spec', {})
                    
                    versions = []
                    
                    # Handle both old and new CRD formats
                    if 'versions' in spec:
                        # New format (multiple versions)
                        for version in spec['versions']:
                            versions.append({
                                'name': version.get('name', 'unknown'),
                                'served': version.get('served', True),
                                'storage': version.get('storage', False)
                            })
                    elif 'version' in spec:
                        # Old format (single version)
                        versions.append({
                            'name': spec['version'],
                            'served': True,
                            'storage': True
                        })
                    
                    crds.append({
                        'name': crd_name,
                        'group': spec.get('group', 'unknown'),
                        'scope': spec.get('scope', 'unknown'),
                        'versions': versions,
                        'file': str(crd_file.relative_to(crd_file.parents[2])) if len(crd_file.parents) > 1 else str(crd_file.name)
                    })
        except Exception as e:
            msg(f"Error processing {crd_file}: {e}", Colors.YELLOW)
            
        return crds
    
    def analyze_release(self, version: str) -> Dict[str, Dict]:
        """Analyze all CRDs in a release"""
        msg(f"Analyzing CRDs for release {version}...")
        
        # Check if already downloaded
        version_dir = self.workspace / version
        if not version_dir.exists():
            release_dir = self.download_manifest(version)
        else:
            msg(f"Using existing manifests for {version}")
            release_dir = version_dir
            
        crd_files = self.find_crd_files(release_dir)
        
        crds = {}
        for crd_file in crd_files:
            crd_infos = self.extract_crd_info(crd_file)
            for crd_info in crd_infos:
                if crd_info:
                    crds[crd_info['name']] = crd_info
        
        msg(f"Analyzed {len(crds)} CRDs in release {version}", Colors.GREEN)
        return crds
    
    def save_analysis(self, version: str, crds: Dict[str, Dict]):
        """Save CRD analysis to files"""
        # JSON output
        json_file = self.workspace / f"crds-{version}.json"
        with open(json_file, 'w', encoding='utf-8') as f:
            json.dump(crds, f, indent=2, sort_keys=True)
        
        # Human-readable summary
        summary_file = self.workspace / f"crds-{version}-summary.txt"
        with open(summary_file, 'w', encoding='utf-8') as f:
            f.write(f"CRD API Versions for Release {version}\n")
            f.write("=" * 60 + "\n\n")
            
            f.write(f"Total CRDs: {len(crds)}\n\n")
            
            # Group by API group
            by_group = defaultdict(list)
            for crd_name, crd in crds.items():
                by_group[crd['group']].append(crd_name)
            
            for group in sorted(by_group.keys()):
                f.write(f"Group: {group}\n")
                f.write("-" * 40 + "\n")
                f.write(f"  CRDs in group: {len(by_group[group])}\n\n")
                
                for crd_name in sorted(by_group[group]):
                    crd = crds[crd_name]
                    f.write(f"  {crd_name}\n")
                    f.write(f"    Scope: {crd['scope']}\n")
                    f.write(f"    File: {crd['file']}\n")
                    f.write(f"    API Versions:\n")
                    
                    for version_info in crd['versions']:
                        served_icon = "✅" if version_info['served'] else "❌"
                        storage_icon = "📦" if version_info['storage'] else "  "
                        f.write(f"      {version_info['name']:<15} served: {served_icon}  storage: {storage_icon}\n")
                    f.write("\n")
                f.write("\n")
        
        msg(f"Analysis saved to {json_file.name} and {summary_file.name}", Colors.GREEN)
    
    def compare_releases(self, version1: str, version2: str):
        """Compare CRDs between two releases"""
        msg(f"Comparing CRDs between {version1} and {version2}...")
        
        crds1 = self.analyze_release(version1)
        crds2 = self.analyze_release(version2)
        
        self.save_analysis(version1, crds1)
        self.save_analysis(version2, crds2)
        
        # Generate detailed comparison
        self._generate_comparison_report(version1, version2, crds1, crds2)

    def analyze_local_component(self, component_path: str) -> Dict[str, Dict]:
        """Analyze CRDs in a local component directory."""
        msg(f"Analyzing CRDs for local component at '{component_path}'...")
        
        # Assume the script is run from the repo root.
        # The component path is relative to the repo root.
        repo_root = Path.cwd()
        component_dir = repo_root / component_path
        
        if not component_dir.is_dir():
            do_fail(f"Component directory not found: {component_dir}")
            
        crd_files = self.find_crd_files(component_dir)
        
        crds = {}
        for crd_file in crd_files:
            crd_infos = self.extract_crd_info(crd_file)
            for crd_info in crd_infos:
                if crd_info:
                    crds[crd_info['name']] = crd_info
        
        msg(f"Analyzed {len(crds)} CRDs in local component '{component_path}'", Colors.GREEN)
        return crds

    def compare_local_to_release(self, component_path: str, version: str):
        """Compare a local component's CRDs to a release version."""
        msg(f"Comparing local component '{component_path}' against release {version}...")
        
        # Analyze the local component directory
        crds1 = self.analyze_local_component(component_path)
        
        # Analyze the release
        crds2 = self.analyze_release(version)

        # We only care about CRDs present in the local component analysis.
        # Filter the release CRDs to only those that are relevant to the component.
        # This is a simple heuristic: if a CRD from the release is in a path that contains the component name, we keep it.
        # A better approach might be to check if the CRD name from crds1 exists in crds2.
        
        crds1_names = set(crds1.keys())
        
        # Filter crds2 to only include CRDs that are either in crds1 or whose files are in a path matching the component name.
        # This ensures we can see changes to existing CRDs and also detect if a CRD was moved out of the component.
        filtered_crds2 = {}
        for name, crd in crds2.items():
            # A bit of a heuristic: if the file path contains the component name, it's probably relevant.
            if f"/{component_path}/" in crd.get('file', ''):
                 filtered_crds2[name] = crd
            elif name in crds1_names: # Also include it if it's a CRD we have locally, it might have moved.
                 filtered_crds2[name] = crd


        # The comparison report needs all CRDs from both sets to show added/removed correctly.
        # Let's refine the sets for comparison.
        # `crds1` is the set of CRDs in the local component directory.
        # We want to compare this to the set of CRDs that BELONGED to that component in the target release.
        
        # So, we need to identify which CRDs in the release belong to the component.
        release_component_crds = {
            name: crd for name, crd in crds2.items()
            if f"/{component_path}/" in crd.get('file', '')
        }

        msg(f"Found {len(release_component_crds)} CRDs for component '{component_path}' in release '{version}'")

        self.save_analysis(f"local-{component_path.replace('/', '-')}", crds1)
        self.save_analysis(f"{version}-{component_path.replace('/', '-')}", release_component_crds)
        
        # Generate detailed comparison (release -> local)
        self._generate_comparison_report(version, f"local-{component_path.replace('/', '-')}", release_component_crds, crds1)
    
    def _generate_comparison_report(self, v1: str, v2: str, crds1: Dict, crds2: Dict):
        """Generate a detailed comparison report"""
        comparison_file = self.workspace / f"crd-comparison-{v1}-to-{v2}.md"
        
        all_crds = set(crds1.keys()) | set(crds2.keys())
        added = set(crds2.keys()) - set(crds1.keys())
        removed = set(crds1.keys()) - set(crds2.keys())
        common = set(crds1.keys()) & set(crds2.keys())
        
        # Analyze version changes in common CRDs
        changed_crds = []
        unchanged_crds = []
        
        for crd_name in common:
            crd1 = crds1[crd_name]
            crd2 = crds2[crd_name]
            
            v1_versions = {v['name']: v for v in crd1['versions']}
            v2_versions = {v['name']: v for v in crd2['versions']}
            
            if v1_versions != v2_versions:
                changed_crds.append(crd_name)
            else:
                unchanged_crds.append(crd_name)
        
        with open(comparison_file, 'w', encoding='utf-8') as f:
            f.write(f"# CRD API Version Comparison: {v1} → {v2}\n\n")
            
            f.write("## Quick Summary\n\n")
            f.write(f"| Status | Count | CRDs |\n")
            f.write(f"|--------|-------|------|\n")
            f.write(f"| ➕ **Added** | {len(added)} | {', '.join(f'`{crd}`' for crd in sorted(added)) if added else 'None'} |\n")
            f.write(f"| ➖ **Removed** | {len(removed)} | {', '.join(f'`{crd}`' for crd in sorted(removed)) if removed else 'None'} |\n")
            f.write(f"| 🔄 **Changed** | {len(changed_crds)} | {', '.join(f'`{crd}`' for crd in sorted(changed_crds)) if changed_crds else 'None'} |\n")
            f.write(f"| ✅ **Unchanged** | {len(unchanged_crds)} | {', '.join(f'`{crd}`' for crd in sorted(unchanged_crds)) if unchanged_crds else 'None'} |\n")
            f.write("\n")
            
            # All CRDs overview table
            f.write("## All CRDs Overview\n\n")
            f.write(f"| CRD Name | Group | {v1} | {v2} | Status |\n")
            f.write(f"|----------|-------|{'-' * len(v1)}|{'-' * len(v2)}|--------|\n")
            
            for crd_name in sorted(all_crds):
                crd1 = crds1.get(crd_name)
                crd2 = crds2.get(crd_name)
                
                if crd1 is None:
                    # Added - crd2 must exist
                    crd2_data = crds2[crd_name]
                    group = crd2_data['group']
                    v1_versions_str = "-"
                    v2_versions_str = ", ".join([f"**{v['name']}**" if v['storage'] else v['name'] for v in crd2_data['versions']])
                    status = "➕ NEW"
                elif crd2 is None:
                    # Removed - crd1 must exist
                    crd1_data = crds1[crd_name]
                    group = crd1_data['group']
                    v1_versions_str = ", ".join([f"**{v['name']}**" if v['storage'] else v['name'] for v in crd1_data['versions']])
                    v2_versions_str = "-"
                    status = "➖ REMOVED"
                else:
                    # Common - check if changed
                    group = crd1['group']
                    v1_versions_str = ", ".join([f"**{v['name']}**" if v['storage'] else v['name'] for v in crd1['versions']])
                    v2_versions_str = ", ".join([f"**{v['name']}**" if v['storage'] else v['name'] for v in crd2['versions']])
                    
                    if crd_name in changed_crds:
                        status = "🔄 CHANGED"
                    else:
                        status = "✅ Same"
                
                f.write(f"| `{crd_name}` | {group} | {v1_versions_str} | {v2_versions_str} | {status} |\n")
            
            f.write("\n**Legend**: **Bold** = storage version, Regular = served version\n\n")
            
            if changed_crds:
                f.write("## Changed CRDs Details\n\n")
            
            f.write("## Detailed Changes\n\n")
            
            # Process each CRD
            for crd_name in sorted(all_crds):
                crd1 = crds1.get(crd_name)
                crd2 = crds2.get(crd_name)
                
                if crd1 is None:
                    # New CRD - crd2 must exist
                    crd2_data = crds2[crd_name]
                    f.write(f"### ➕ {crd_name} (NEW)\n")
                    f.write(f"**Group**: {crd2_data['group']}\n")
                    f.write(f"**Scope**: {crd2_data['scope']}\n")
                    f.write(f"**File**: {crd2_data['file']}\n\n")
                    f.write("**API Versions**:\n")
                    self._write_crd_versions(f, crd2_data)
                    
                elif crd2 is None:
                    # Removed CRD - crd1 must exist
                    crd1_data = crds1[crd_name]
                    f.write(f"### ➖ {crd_name} (REMOVED)\n")
                    f.write(f"**Group**: {crd1_data['group']}\n")
                    f.write(f"**Scope**: {crd1_data['scope']}\n")
                    f.write(f"**File**: {crd1_data['file']}\n\n")
                    f.write("**API Versions**:\n")
                    self._write_crd_versions(f, crd1_data)
                    
                else:
                    # Compare versions
                    v1_versions = {v['name']: v for v in crd1['versions']}
                    v2_versions = {v['name']: v for v in crd2['versions']}
                    
                    if v1_versions != v2_versions:
                        f.write(f"### 🔄 {crd_name} (CHANGED)\n")
                        f.write(f"**Group**: {crd1['group']}\n\n")
                        
                        f.write(f"**{v1} versions**:\n")
                        self._write_crd_versions(f, crd1)
                        
                        f.write(f"**{v2} versions**:\n")
                        self._write_crd_versions(f, crd2)
                        
                        # Analyze the specific changes
                        added_versions = set(v2_versions.keys()) - set(v1_versions.keys())
                        removed_versions = set(v1_versions.keys()) - set(v2_versions.keys())
                        
                        if added_versions or removed_versions:
                            f.write("**Changes**:\n")
                            for v in sorted(added_versions):
                                f.write(f"- ➕ Added version `{v}`\n")
                            for v in sorted(removed_versions):
                                f.write(f"- ➖ Removed version `{v}`\n")
                            
                            # Check for served/storage changes
                            for v_name in set(v1_versions.keys()) & set(v2_versions.keys()):
                                v1_info = v1_versions[v_name]
                                v2_info = v2_versions[v_name]
                                
                                if v1_info['served'] != v2_info['served']:
                                    change = "enabled" if v2_info['served'] else "disabled"
                                    f.write(f"- 🔄 Version `{v_name}` serving {change}\n")
                                
                                if v1_info['storage'] != v2_info['storage']:
                                    change = "enabled" if v2_info['storage'] else "disabled"
                                    f.write(f"- 🔄 Version `{v_name}` storage {change}\n")
                        
                        f.write("\n")
        
        msg(f"Comparison report saved to {comparison_file.name}", Colors.GREEN)
        
        # Also create a simple summary for terminal output
        self._print_comparison_summary(v1, v2, added, removed, changed_crds)
    
    def _write_crd_versions(self, f, crd):
        """Helper to write CRD version info to file"""
        for version in crd['versions']:
            served_icon = "✅" if version['served'] else "❌"
            storage_icon = "📦" if version['storage'] else "  "
            f.write(f"- `{version['name']}` - served: {served_icon} storage: {storage_icon}\n")
        f.write("\n")
    
    def _print_comparison_summary(self, v1: str, v2: str, added: set, removed: set, changed: list):
        """Print a quick summary to terminal"""
        print()
        msg("=== COMPARISON SUMMARY ===", Colors.CYAN)
        print(f"  Added CRDs: {len(added)}")
        print(f"  Removed CRDs: {len(removed)}")
        print(f"  Changed CRDs: {len(changed)}")
        
        if added:
            print(f"\n  {Colors.GREEN}Added:{Colors.RESET}")
            for crd in sorted(list(added)[:5]):  # Show first 5
                print(f"    + {crd}")
            if len(added) > 5:
                print(f"    ... and {len(added) - 5} more")
        
        if removed:
            print(f"\n  {Colors.RED}Removed:{Colors.RESET}")
            for crd in sorted(list(removed)[:5]):  # Show first 5
                print(f"    - {crd}")
            if len(removed) > 5:
                print(f"    ... and {len(removed) - 5} more")
        
        if changed:
            print(f"\n  {Colors.YELLOW}Changed:{Colors.RESET}")
            for crd in sorted(changed[:5]):  # Show first 5
                print(f"    ~ {crd}")
            if len(changed) > 5:
                print(f"    ... and {len(changed) - 5} more")

def main():
    parser = argparse.ArgumentParser(
        description="Analyze and compare CRD API versions between NNF releases",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  %(prog)s analyze v0.1.23
  %(prog)s compare v0.1.20 v0.1.23
  %(prog)s -w /tmp/analysis compare v0.1.20 v0.1.23
        """
    )
    
    parser.add_argument("-w", "--workspace", default="workingspace", 
                       help="Working directory (default: workingspace)")
    parser.add_argument("-u", "--url", default="git@github.com:NearNodeFlash/nnf-deploy.git",
                       help="NNF deploy repository URL")
    
    subparsers = parser.add_subparsers(dest="command", help="Available commands")
    
    # Analyze command
    analyze_parser = subparsers.add_parser("analyze", help="Analyze CRDs for a single release")
    analyze_parser.add_argument("version", help="Release version to analyze (e.g., v0.1.23)")
    
    # Compare command
    compare_parser = subparsers.add_parser("compare", help="Compare CRDs between two releases")
    compare_parser.add_argument("version1", help="First release version (e.g., v0.1.20)")
    compare_parser.add_argument("version2", help="Second release version (e.g., v0.1.23)")

    # Compare local command
    compare_local_parser = subparsers.add_parser("compare-local", help="Compare a local component's CRDs against a release")
    compare_local_parser.add_argument("component", help="Name of the local component directory to analyze (e.g., dws)")
    compare_local_parser.add_argument("version", help="Release version to compare against (e.g., v0.1.23)")
    
    args = parser.parse_args()
    
    if not args.command:
        parser.print_help()
        return 1
    
    try:
        analyzer = CRDAnalyzer(args.workspace, args.url)
        analyzer.check_dependencies()
        
        if args.command == "analyze":
            crds = analyzer.analyze_release(args.version)
            analyzer.save_analysis(args.version, crds)
            msg(f"Analysis complete! Check {analyzer.workspace} for results.", Colors.GREEN)
        
        elif args.command == "compare":
            analyzer.compare_releases(args.version1, args.version2)
            msg(f"Comparison complete! Check {analyzer.workspace} for detailed reports.", Colors.GREEN)

        elif args.command == "compare-local":
            analyzer.compare_local_to_release(args.component, args.version)
            msg(f"Local comparison complete! Check {analyzer.workspace} for detailed reports.", Colors.GREEN)
        
        return 0
    
    except KeyboardInterrupt:
        msg("\nOperation cancelled by user", Colors.YELLOW)
        return 1
    except Exception as e:
        msg(f"Error: {e}", Colors.RED)
        return 1

if __name__ == "__main__":
    sys.exit(main())
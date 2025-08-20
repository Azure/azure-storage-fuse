#!/usr/bin/env python3
"""
Azure Storage Fuse Release Logs Display Tool

This script provides a comprehensive way to view release logs for Azure Storage Fuse.
It can display information from both the CHANGELOG.md file and GitHub releases API.

Usage:
    python show_release_logs.py [options]

Options:
    --format=FORMAT      Output format: table, json, markdown, or detailed (default: detailed)
    --source=SOURCE      Data source: changelog, github, or both (default: both)
    --version=VERSION    Filter by specific version (e.g., 2.5.0)
    --limit=N           Limit to N most recent releases (default: 10)
    --include-assets     Include download assets information (GitHub only)
    --include-stats      Include download statistics (GitHub only)
"""

import os
import sys
import json
import re
import argparse
from datetime import datetime
from typing import List, Dict, Any, Optional
import urllib.request
import urllib.error

class ReleaseInfo:
    """Container for release information"""
    def __init__(self, version: str, date: str = None, description: str = None):
        self.version = version
        self.date = date
        self.description = description
        self.features = []
        self.bug_fixes = []
        self.other_changes = []
        self.assets = []
        self.download_count = 0
        
    def add_feature(self, feature: str):
        self.features.append(feature)
        
    def add_bug_fix(self, bug_fix: str):
        self.bug_fixes.append(bug_fix)
        
    def add_other_change(self, change: str):
        self.other_changes.append(change)

class ChangelogParser:
    """Parser for CHANGELOG.md file"""
    
    def __init__(self, changelog_path: str):
        self.changelog_path = changelog_path
        
    def parse(self) -> List[ReleaseInfo]:
        """Parse the changelog file and return list of releases"""
        releases = []
        
        if not os.path.exists(self.changelog_path):
            print(f"Warning: CHANGELOG.md not found at {self.changelog_path}")
            return releases
            
        with open(self.changelog_path, 'r', encoding='utf-8') as f:
            content = f.read()
            
        # Split into release sections
        release_sections = re.split(r'^## ', content, flags=re.MULTILINE)
        
        for section in release_sections[1:]:  # Skip first empty section
            release = self._parse_release_section(section)
            if release:
                releases.append(release)
                
        return releases
    
    def _parse_release_section(self, section: str) -> Optional[ReleaseInfo]:
        """Parse a single release section"""
        lines = section.strip().split('\n')
        if not lines:
            return None
            
        # Parse version and date from first line
        header_match = re.match(r'(.+?)\s*\(([^)]+)\)', lines[0])
        if not header_match:
            return None
            
        version = header_match.group(1).strip()
        date_str = header_match.group(2).strip()
        
        release = ReleaseInfo(version, date_str)
        
        current_section = None
        
        for line in lines[1:]:
            line = line.strip()
            if not line:
                continue
                
            # Check for section headers
            if line.startswith('**') and line.endswith('**'):
                section_name = line[2:-2].lower()
                if 'feature' in section_name:
                    current_section = 'features'
                elif 'bug' in section_name or 'fix' in section_name:
                    current_section = 'bug_fixes'
                elif 'change' in section_name or 'other' in section_name:
                    current_section = 'other_changes'
                else:
                    current_section = 'other_changes'
            elif line.startswith('- '):
                # This is a list item
                item = line[2:].strip()
                if current_section == 'features':
                    release.add_feature(item)
                elif current_section == 'bug_fixes':
                    release.add_bug_fix(item)
                elif current_section == 'other_changes':
                    release.add_other_change(item)
                else:
                    # Default to other changes if no section specified
                    release.add_other_change(item)
                    
        return release

class GitHubReleasesAPI:
    """Interface to GitHub Releases API"""
    
    def __init__(self, repo_owner: str = "Azure", repo_name: str = "azure-storage-fuse"):
        self.repo_owner = repo_owner
        self.repo_name = repo_name
        self.base_url = f"https://api.github.com/repos/{repo_owner}/{repo_name}"
        
    def get_releases(self, limit: int = 10) -> List[ReleaseInfo]:
        """Fetch releases from GitHub API"""
        releases = []
        
        try:
            url = f"{self.base_url}/releases?per_page={limit}"
            
            with urllib.request.urlopen(url) as response:
                data = json.loads(response.read().decode())
                
            for release_data in data:
                release = self._parse_github_release(release_data)
                if release:
                    releases.append(release)
                    
        except urllib.error.URLError as e:
            print(f"Warning: Could not fetch GitHub releases: {e}")
            
        return releases
    
    def _parse_github_release(self, data: Dict[str, Any]) -> Optional[ReleaseInfo]:
        """Parse GitHub release data"""
        tag_name = data.get('tag_name', '')
        name = data.get('name', '')
        published_at = data.get('published_at', '')
        body = data.get('body', '')
        assets = data.get('assets', [])
        
        # Extract version from tag name
        version = tag_name.replace('blobfuse2-', '') if tag_name.startswith('blobfuse2-') else tag_name
        
        # Parse date
        date_str = ''
        if published_at:
            try:
                dt = datetime.fromisoformat(published_at.replace('Z', '+00:00'))
                date_str = dt.strftime('%Y-%m-%d')
            except ValueError:
                date_str = published_at[:10]  # Fallback to first 10 chars
                
        release = ReleaseInfo(version, date_str, body)
        
        # Parse release body for structured information
        self._parse_release_body(release, body)
        
        # Add assets information
        total_downloads = 0
        for asset in assets:
            asset_info = {
                'name': asset.get('name', ''),
                'size': asset.get('size', 0),
                'download_count': asset.get('download_count', 0),
                'browser_download_url': asset.get('browser_download_url', '')
            }
            release.assets.append(asset_info)
            total_downloads += asset.get('download_count', 0)
            
        release.download_count = total_downloads
        
        return release
    
    def _parse_release_body(self, release: ReleaseInfo, body: str):
        """Parse the release body text for features, bug fixes, etc."""
        if not body:
            return
            
        current_section = None
        
        for line in body.split('\n'):
            line = line.strip()
            if not line:
                continue
                
            # Check for section headers
            if line.startswith('**') and line.endswith('**'):
                section_name = line[2:-2].lower()
                if 'feature' in section_name:
                    current_section = 'features'
                elif 'bug' in section_name or 'fix' in section_name:
                    current_section = 'bug_fixes'
                elif 'change' in section_name or 'other' in section_name:
                    current_section = 'other_changes'
                else:
                    current_section = 'other_changes'
            elif line.startswith('- '):
                # This is a list item
                item = line[2:].strip()
                if current_section == 'features':
                    release.add_feature(item)
                elif current_section == 'bug_fixes':
                    release.add_bug_fix(item)
                elif current_section == 'other_changes':
                    release.add_other_change(item)

class ReleaseLogDisplay:
    """Display release logs in various formats"""
    
    def __init__(self, releases: List[ReleaseInfo]):
        self.releases = releases
        
    def display_detailed(self, include_assets: bool = False, include_stats: bool = False):
        """Display detailed release information"""
        for i, release in enumerate(self.releases):
            if i > 0:
                print("\n" + "="*80 + "\n")
                
            print(f"Version: {release.version}")
            if release.date:
                print(f"Date: {release.date}")
            if include_stats and release.download_count > 0:
                print(f"Total Downloads: {release.download_count:,}")
            print()
            
            if release.features:
                print("Features:")
                for feature in release.features:
                    print(f"  • {feature}")
                print()
                
            if release.bug_fixes:
                print("Bug Fixes:")
                for bug_fix in release.bug_fixes:
                    print(f"  • {bug_fix}")
                print()
                
            if release.other_changes:
                print("Other Changes:")
                for change in release.other_changes:
                    print(f"  • {change}")
                print()
                
            if include_assets and release.assets:
                print("Available Downloads:")
                for asset in release.assets:
                    size_mb = asset['size'] / (1024 * 1024) if asset['size'] > 0 else 0
                    downloads = asset.get('download_count', 0)
                    print(f"  • {asset['name']} ({size_mb:.1f} MB, {downloads:,} downloads)")
                print()
    
    def display_table(self):
        """Display releases in table format"""
        print(f"{'Version':<20} {'Date':<12} {'Features':<10} {'Bug Fixes':<10} {'Downloads':<12}")
        print("-" * 74)
        
        for release in self.releases:
            features_count = len(release.features)
            bug_fixes_count = len(release.bug_fixes)
            downloads = f"{release.download_count:,}" if release.download_count > 0 else "N/A"
            
            print(f"{release.version:<20} {release.date:<12} {features_count:<10} {bug_fixes_count:<10} {downloads:<12}")
    
    def display_markdown(self):
        """Display releases in markdown format"""
        print("# Azure Storage Fuse Release Logs\n")
        
        for release in self.releases:
            print(f"## Version {release.version}")
            if release.date:
                print(f"**Released:** {release.date}")
            if release.download_count > 0:
                print(f"**Downloads:** {release.download_count:,}")
            print()
            
            if release.features:
                print("### Features")
                for feature in release.features:
                    print(f"- {feature}")
                print()
                
            if release.bug_fixes:
                print("### Bug Fixes")
                for bug_fix in release.bug_fixes:
                    print(f"- {bug_fix}")
                print()
                
            if release.other_changes:
                print("### Other Changes")
                for change in release.other_changes:
                    print(f"- {change}")
                print()
    
    def display_json(self):
        """Display releases in JSON format"""
        releases_data = []
        
        for release in self.releases:
            release_data = {
                'version': release.version,
                'date': release.date,
                'features': release.features,
                'bug_fixes': release.bug_fixes,
                'other_changes': release.other_changes,
                'download_count': release.download_count,
                'assets': release.assets
            }
            releases_data.append(release_data)
            
        print(json.dumps(releases_data, indent=2, ensure_ascii=False))

def main():
    parser = argparse.ArgumentParser(
        description='Display Azure Storage Fuse release logs',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__
    )
    
    parser.add_argument('--format', choices=['table', 'json', 'markdown', 'detailed'],
                        default='detailed', help='Output format')
    parser.add_argument('--source', choices=['changelog', 'github', 'both'],
                        default='both', help='Data source')
    parser.add_argument('--version', help='Filter by specific version')
    parser.add_argument('--limit', type=int, default=10,
                        help='Limit to N most recent releases')
    parser.add_argument('--include-assets', action='store_true',
                        help='Include download assets information')
    parser.add_argument('--include-stats', action='store_true',
                        help='Include download statistics')
    
    args = parser.parse_args()
    
    releases = []
    
    # Get current script directory to find CHANGELOG.md
    script_dir = os.path.dirname(os.path.abspath(__file__))
    changelog_path = os.path.join(script_dir, 'CHANGELOG.md')
    
    # Parse changelog if requested
    if args.source in ['changelog', 'both']:
        print("Parsing CHANGELOG.md...", file=sys.stderr)
        changelog_parser = ChangelogParser(changelog_path)
        changelog_releases = changelog_parser.parse()
        releases.extend(changelog_releases)
    
    # Fetch GitHub releases if requested
    if args.source in ['github', 'both']:
        print("Fetching GitHub releases...", file=sys.stderr)
        github_api = GitHubReleasesAPI()
        github_releases = github_api.get_releases(limit=args.limit)
        
        # Merge with changelog releases or add new ones
        if args.source == 'both':
            # Create a map of existing releases by version
            existing_versions = {r.version for r in releases}
            
            # Add GitHub releases that aren't already in changelog
            for gh_release in github_releases:
                if gh_release.version not in existing_versions:
                    releases.append(gh_release)
                else:
                    # Update existing release with GitHub data
                    for existing in releases:
                        if existing.version == gh_release.version:
                            existing.assets = gh_release.assets
                            existing.download_count = gh_release.download_count
                            if not existing.date:
                                existing.date = gh_release.date
                            break
        else:
            releases.extend(github_releases)
    
    # Filter by version if specified
    if args.version:
        releases = [r for r in releases if args.version in r.version]
    
    # Sort by version (newest first)
    def version_sort_key(release):
        # Extract version parts for proper sorting
        version = release.version
        # Remove common prefixes and suffixes
        version = version.replace('blobfuse2-', '').replace('~', '-')
        parts = version.split('.')
        try:
            # Convert to tuple of integers for proper numeric sorting
            return tuple(int(part.split('-')[0]) for part in parts if part.split('-')[0].isdigit())
        except (ValueError, IndexError):
            return (0, 0, 0)  # Fallback for unparseable versions
    
    releases.sort(key=version_sort_key, reverse=True)
    
    # Apply limit
    if args.limit and len(releases) > args.limit:
        releases = releases[:args.limit]
    
    if not releases:
        print("No releases found.")
        return
    
    print(f"\nFound {len(releases)} releases:\n", file=sys.stderr)
    
    # Display releases
    display = ReleaseLogDisplay(releases)
    
    if args.format == 'detailed':
        display.display_detailed(args.include_assets, args.include_stats)
    elif args.format == 'table':
        display.display_table()
    elif args.format == 'markdown':
        display.display_markdown()
    elif args.format == 'json':
        display.display_json()

if __name__ == '__main__':
    main()
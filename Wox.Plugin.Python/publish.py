#!/usr/bin/env python
import os
import sys
import subprocess
import re
from pathlib import Path

def run_command(command: str) -> int:
    """Run command and return exit code"""
    print(f"\n>>> Running: {command}")
    return subprocess.call(command, shell=True)

def update_version(version_type: str) -> str:
    """Update version number in setup.py, pyproject.toml and __init__.py
    version_type: major, minor, or patch
    """
    # Read setup.py
    setup_path = Path("setup.py")
    content = setup_path.read_text()
    
    # Find current version
    version_match = re.search(r'version="(\d+)\.(\d+)\.(\d+)"', content)
    if not version_match:
        print("Error: Could not find version in setup.py")
        sys.exit(1)
    
    major, minor, patch = map(int, version_match.groups())
    
    # Update version number
    if version_type == "major":
        major += 1
        minor = 0
        patch = 0
    elif version_type == "minor":
        minor += 1
        patch = 0
    else:  # patch
        patch += 1
    
    new_version = f"{major}.{minor}.{patch}"
    
    # Update setup.py
    new_content = re.sub(
        r'version="(\d+)\.(\d+)\.(\d+)"',
        f'version="{new_version}"',
        content
    )
    setup_path.write_text(new_content)
    
    # Update pyproject.toml
    pyproject_path = Path("pyproject.toml")
    pyproject_content = pyproject_path.read_text()
    new_pyproject_content = re.sub(
        r'version = "(\d+)\.(\d+)\.(\d+)"',
        f'version = "{new_version}"',
        pyproject_content
    )
    pyproject_path.write_text(new_pyproject_content)
    
    # Update __init__.py
    init_path = Path("wox_plugin/__init__.py")
    init_content = init_path.read_text()
    new_init_content = re.sub(
        r'__version__ = "(\d+)\.(\d+)\.(\d+)"',
        f'__version__ = "{new_version}"',
        init_content
    )
    init_path.write_text(new_init_content)
    
    return new_version

def main():
    # Check command line arguments
    if len(sys.argv) != 2 or sys.argv[1] not in ["major", "minor", "patch"]:
        print("Usage: python publish.py [major|minor|patch]")
        sys.exit(1)
    
    version_type = sys.argv[1]
    
    # Clean previous build files
    if run_command("rm -rf dist/ build/ *.egg-info"):
        print("Error: Failed to clean old build files")
        sys.exit(1)
    
    # Update version number
    new_version = update_version(version_type)
    print(f"Updated version to {new_version}")
    
    # Build package
    if run_command("python -m build"):
        print("Error: Build failed")
        sys.exit(1)
    
    # Upload to PyPI
    if run_command("python -m twine upload dist/*"):
        print("Error: Upload to PyPI failed")
        sys.exit(1)
    
    print(f"\nSuccessfully published version {new_version} to PyPI!")
    print("Package can be installed with:")
    print(f"pip install wox-plugin=={new_version}")

    # remove build directory
    if run_command("rm -rf dist wox_plugin_python.egg-info"):
        print("Error: Failed to remove build directory")
        sys.exit(1)

if __name__ == "__main__":
    main() 
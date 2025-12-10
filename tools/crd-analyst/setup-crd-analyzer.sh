#!/bin/bash

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


# CRD Analyzer Setup Script
# This script sets up the Python virtual environment and dependencies for the CRD analyzer
# When sourced (source setup-crd-analyzer.sh), it will activate the virtual environment
# When executed (./setup-crd-analyzer.sh), it will set up the environment and provide activation instructions

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VENV_DIR="$SCRIPT_DIR/venv"

echo "Setting up CRD Analyzer..."

# Check if Python 3 is available
if ! command -v python3 &> /dev/null; then
    echo "Error: python3 is required but not found"
    exit 1
fi

# Create virtual environment if it doesn't exist
if [[ ! -d "$VENV_DIR" ]]; then
    echo "Creating Python virtual environment..."
    python3 -m venv "$VENV_DIR"
fi

# Activate virtual environment and install dependencies
echo "Installing dependencies..."
source "$VENV_DIR/bin/activate"
pip install pyyaml

# Check if script is being sourced or executed
if [[ "${BASH_SOURCE[0]}" != "${0}" ]]; then
    # Script is being sourced - activate the virtual environment for the current shell
    echo "Virtual environment activated! You can now use the CRD analyzer."
    echo ""
    echo "Examples:"
    echo "  ./crd-analyzer.py analyze v0.1.23"
    echo "  ./crd-analyzer.py compare v0.1.20 v0.1.23"
    echo ""
    echo "To deactivate the virtual environment later, run: deactivate"
else
    # Script is being executed - provide instructions
    echo "Setup complete!"
    echo ""
    echo "To use the CRD analyzer, run:"
    echo "  source tools/crd-analyst/venv/bin/activate"
    echo "  ./tools/crd-analyst/crd-analyzer.py --help"
    echo ""
    echo "Examples:"
    echo "  ./tools/crd-analyst/crd-analyzer.py analyze v0.1.23"
    echo "  ./tools/crd-analyst/crd-analyzer.py compare v0.1.20 v0.1.23"
    echo ""
    echo "Or source this setup script to activate the environment:"
    echo "  source tools/crd-analyst/setup-crd-analyzer.sh"
fi
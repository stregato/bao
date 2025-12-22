#!/bin/bash

set -euo pipefail

# Resolve paths relative to this script so the build works from anywhere
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
PYTHON_BIN="${REPO_ROOT}/.venv/bin/python"
INSTALL_REQS_FILE="${SCRIPT_DIR}/requirements.txt"
SETUP_PY="${SCRIPT_DIR}/setup.py"
LIB_SOURCE_DIR="${REPO_ROOT}/build"
LIB_DEST_DIR="${SCRIPT_DIR}/baolib/_libs"

# Ensure the virtual environment interpreter is available
if [ ! -x "$PYTHON_BIN" ]; then
  echo "Error: Python binary $PYTHON_BIN not found. Activate the repo virtualenv before running this script."
  exit 1
fi

if [ ! -f "$INSTALL_REQS_FILE" ]; then
  echo "Error: Requirements file $INSTALL_REQS_FILE not found."
  exit 1
fi

if [ ! -f "$SETUP_PY" ]; then
  echo "Error: setup.py not found in ${SCRIPT_DIR}."
  exit 1
fi

cd "$SCRIPT_DIR"

$PYTHON_BIN -m pip install -r "$INSTALL_REQS_FILE"

platforms=("darwin_arm64" "linux_amd64" "windows_amd64")
platform_tags=("macosx_11_0_arm64" "manylinux1_x86_64" "win_amd64")

# Check if the source directory exists
if [ ! -d "$LIB_SOURCE_DIR" ]; then
  echo "Error: Directory $LIB_SOURCE_DIR does not exist."
  exit 1
fi

rm -rf dist/

for idx in "${!platforms[@]}"; do
  target="${platforms[$idx]}"
  platform_tag="${platform_tags[$idx]}"
  IFS="_" read -r os arch <<< "$target"
  src_dir="${LIB_SOURCE_DIR}/${os}"

  if [ ! -d "$src_dir" ]; then
    echo "Skipping ${target}: directory ${src_dir} does not exist."
    continue
  fi

  rm -rf build/
  rm -rf "${LIB_DEST_DIR}"
  mkdir -p "${LIB_DEST_DIR}"

  shopt -s nullglob
  files=(${src_dir}/*${arch}*)
  shopt -u nullglob

  if [ ${#files[@]} -eq 0 ]; then
    echo "Skipping ${target}: no artifacts for ${arch} in ${src_dir}."
    continue
  fi

  cp "${files[@]}" "${LIB_DEST_DIR}/"

  "$PYTHON_BIN" setup.py bdist_wheel --plat-name "${platform_tag}"
  if [ $? -ne 0 ]; then
    echo "Error: Failed to build package for ${target}. Skipping."
    continue
  fi
done

rm -rf "${LIB_DEST_DIR}"

echo "All packages built successfully."

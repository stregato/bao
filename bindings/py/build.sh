#!/bin/bash

pip install -r requirements.txt

# Directory containing platform-specific subdirectories (sibling of repository root)
LIB_SOURCE_DIR="../../build"
# Directory where the libraries should be copied to for packaging
LIB_DEST_DIR="pbao/_libs"

declare -A platform_tags=(
  [darwin_arm64]="macosx_11_0_arm64"
  [linux_amd64]="manylinux1_x86_64"
  [windows_amd64]="win_amd64"
)

# Check if the source directory exists
if [ ! -d "$LIB_SOURCE_DIR" ]; then
  echo "Error: Directory $LIB_SOURCE_DIR does not exist."
  exit 1
fi

rm -rf dist/

for target in "${!platform_tags[@]}"; do
  IFS="_" read -r os arch <<< "$target"
  src_dir="${LIB_SOURCE_DIR}/${os}"
  platform_tag="${platform_tags[$target]}"

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

  python3 setup.py bdist_wheel --plat-name "${platform_tag}"
  if [ $? -ne 0 ]; then
    echo "Error: Failed to build package for ${target}. Skipping."
    continue
  fi
done

rm -rf "${LIB_DEST_DIR}"

echo "All packages built successfully."

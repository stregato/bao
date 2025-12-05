#!/bin/bash

# Set GitHub repository owner and name
REPO_OWNER="stregato"
REPO_NAME="bao"

# Function to download and extract the zip file for a specific OS
download_and_extract() {
  local os=$1
  local zip_name="bao_${os}.zip"
  local latest_url="https://api.github.com/repos/$REPO_OWNER/$REPO_NAME/releases/latest"
  local asset_url=$(curl -s $latest_url | grep "browser_download_url.*${zip_name}" | cut -d '"' -f 4)

  # Check if an asset URL was found
  if [ -z "$asset_url" ]; then
    echo "No zip file found for ${os} in the latest release."
    return 1
  fi

  echo "Downloading ${zip_name}..."
  
  # Download the zip file
  curl -L -o "${zip_name}" "$asset_url"

  # Create tmp directory if it doesn't exist
  mkdir -p "tmp_bao/${os}"

  # Unzip to the tmp directory
  unzip -q "${zip_name}" -d "tmp_bao/${os}"

  # Clean up
  rm "${zip_name}"
}

# MacOS libraries
if [ -d "macos" ]; then
  download_and_extract "darwin"
  rm -rf macos/Libraries
  mkdir -p macos/Libraries
  cp -f tmp_bao/darwin/*dylib macos/Libraries
  echo "MacOS libraries copied."
fi

# iOS libraries
if [ -d "ios" ]; then
  if [ ! -d "tmp_bao/darwin" ]; then
    download_and_extract "darwin"
  fi
  rm -rf ios/Libraries
  mkdir -p ios/Libraries
  cp -f tmp_bao/darwin/*dylib ios/Libraries
  echo "iOS libraries copied."
fi

# Android libraries
if [ -d "android" ]; then
  download_and_extract "android"
  rm -rf android/Libraries
  mkdir -p android/Libraries
  cp -f tmp_bao/android/*so android/Libraries
  echo "Android libraries copied."
fi

# Linux libraries
if [ -d "linux" ]; then
  download_and_extract "linux"
  rm -rf linux/Libraries
  mkdir -p linux/Libraries
  cp -f tmp_bao/linux/*so linux/Libraries
  echo "Linux libraries copied."
fi

# Windows libraries
if [ -d "windows" ]; then
  download_and_extract "windows"
  rm -rf windows/Libraries
  mkdir -p windows/Libraries
  cp -f tmp_bao/windows/*dll windows/Libraries
  echo "Windows libraries copied."
fi

# Clean up
rm -rf tmp_bao

echo "Downloaded and extracted to tmp folders."
.PHONY: all init clean lib py java package

BUILD_PATH=./build
LIBRARY_PATH=./lib
CLI_PATH=./cli
MAGPIE_PATH=../magpie
WEB_PORT?=8000
SDK=

# --- WASM (browser) ---
WASM_OUT := $(BUILD_PATH)/wasm
WASM_BIN := $(WASM_OUT)/bao.wasm
WASM_EXEC := $(WASM_OUT)/wasm_exec.js

GO_FILES := $(wildcard $(LIBRARY_PATH)/*.go)
CLI_GO_FILES := $(wildcard $(CLI_PATH)/*.go)

all: lib py java dart

init:
	echo "Running init"
	mkdir -p $(BUILD_PATH)

$(BUILD_PATH)/linux/libbao_amd64.so: init $(GO_FILES)
	echo "Building for Linux AMD64"
	GOOS=linux GOARCH=amd64 CC=x86_64-linux-gnu-gcc CXX=x86_64-linux-gnu-g++  MODE=c-shared $(MAKE) build_targets GOOS=linux GOARCH=amd64 LIBRARY_NAME=libbao_amd64.so CLI_NAME=bao_amd64

$(BUILD_PATH)/darwin/libbao_amd64.dylib: init $(GO_FILES)
	echo "Building for Darwin AMD64"
	GOOS=darwin GOARCH=amd64 MODE=c-shared $(MAKE) build_targets GOOS=darwin GOARCH=amd64 LIBRARY_NAME=libbao_amd64.dylib CLI_NAME=bao_amd64

$(BUILD_PATH)/darwin/libbao_arm64.dylib: init $(GO_FILES)
	echo "Building for Darwin ARM64"
	GOOS=darwin GOARCH=arm64 MODE=c-shared $(MAKE) build_targets GOOS=darwin GOARCH=arm64 LIBRARY_NAME=libbao_arm64.dylib CLI_NAME=bao_arm64

$(BUILD_PATH)/ios/bao.xcframework: init $(GO_FILES)
	echo "Building for static ARM64 (iOS simulator)"
	$(MAKE) build_ios_static SDK=iphonesimulator OUT=sim/bao.a SUFFIX=-simulator
	echo "Building for static ARM64 (iOS device)"
	$(MAKE) build_ios_static SDK=iphoneos OUT=ios/bao.a SUFFIX=

	$(MAKE) build_xcframework

# --- WASM (browser) ---
$(BUILD_PATH)/wasm/bao.wasm: init $(GO_FILES) $(LIBRARY_PATH)/jsdemo/main_js.go
	echo "Building WASM (browser)"
	mkdir -p $(BUILD_PATH)/wasm
	cd $(LIBRARY_PATH) && CGO_ENABLED=0 GOOS=js GOARCH=wasm go build -o ../$(BUILD_PATH)/wasm/bao.wasm .

$(BUILD_PATH)/wasm/wasm_exec.js: init
	mkdir -p $(BUILD_PATH)/wasm
	cp $$(go env GOROOT)/misc/wasm/wasm_exec.js $(BUILD_PATH)/wasm/wasm_exec.js

# Helper target
build_ios_static:
	cd $(LIBRARY_PATH) && \
	SDK_PATH=$$(xcrun --sdk $(SDK) --show-sdk-path) && \
	SUFFIX="$(SUFFIX)" && \
	CC=$$(go env GOROOT)/misc/ios/clangwrap.sh && \
	CGO_ENABLED=1 GOOS=ios GOARCH=arm64 \
	CGO_CFLAGS="-fembed-bitcode -isysroot $${SDK_PATH} -target arm64-apple-ios15.5$${SUFFIX}" \
	CGO_LDFLAGS="-isysroot $${SDK_PATH} -target arm64-apple-ios15.5$${SUFFIX}" \
	go build -x -buildmode=c-archive -tags ios \
	-o ../$(BUILD_PATH)/ios/$(OUT)

build_xcframework:
	rm -rf $(BUILD_PATH)/ios/bao.xcframework
	mkdir -p $(BUILD_PATH)/ios/headers
	mv -f $(BUILD_PATH)/ios/ios/bao.h $(BUILD_PATH)/ios/headers/bao.h
	cp $(BUILD_PATH)/../lib/cfunc.h $(BUILD_PATH)/ios/headers/cfunc.h
	xcodebuild -create-xcframework \
		-output $(BUILD_PATH)/ios/bao.xcframework \
		-library $(BUILD_PATH)/ios/sim/bao.a \
		-headers $(BUILD_PATH)/ios/headers \
		-library $(BUILD_PATH)/ios/ios/bao.a \
		-headers $(BUILD_PATH)/ios/headers
	mkdir -p $(BUILD_PATH)/ios/bao.xcframework/ios-arm64/Modules
	mkdir -p $(BUILD_PATH)/ios/bao.xcframework/ios-arm64-simulator/Modules
	cp $(BUILD_PATH)/../module.modulemap $(BUILD_PATH)/ios/bao.xcframework/ios-arm64/Modules/module.modulemap
	cp $(BUILD_PATH)/../module.modulemap $(BUILD_PATH)/ios/bao.xcframework/ios-arm64-simulator/Modules/module.modulemap


#ios: $(BUILD_PATH)/ios/bao.xcframework
#	mkdir -p $(BUILD_PATH)/ios/bao.xcframework/ios-arm64/Modules
#	cp $(BUILD_PATH)/../module.modulemap $(BUILD_PATH)/ios/bao.xcframework/ios-arm64/Modules/module.modulemap

$(BUILD_PATH)/windows/bao_amd64.dll: init $(GO_FILES)
	echo "Building for Windows AMD64"
	GOOS=windows GOARCH=amd64 CC=x86_64-w64-mingw32-gcc MODE=c-shared $(MAKE) build_targets GOOS=windows GOARCH=amd64 LIBRARY_NAME=bao_amd64.dll CLI_NAME=bao_amd64.exe

NDK_PATH := $(shell ls -d $(HOME)/Library/Android/sdk/ndk/* | sort -V | tail -n 1)
$(BUILD_PATH)/android/libbao_arm64.so: init $(GO_FILES)
	echo "Building for Android ARM64"
	CC=$(NDK_PATH)/toolchains/llvm/prebuilt/darwin-x86_64/bin/aarch64-linux-android21-clang MODE=c-shared SDK=${SDK} \
	GOOS=android GOARCH=arm64 $(MAKE) build_targets GOOS=android GOARCH=arm64 LIBRARY_NAME=libbao_arm64.so CLI_NAME=bao_arm64

build_targets:
	echo "GOOS=$(GOOS), GOARCH=$(GOARCH), LIBRARY_NAME=$(LIBRARY_NAME), CLI_NAME=$(CLI_NAME), CC=${CC}, CGO_CFLAGS=$(CGO_CFLAGS), CGO_LDFLAGS=$(CGO_LDFLAGS)"
	mkdir -p $(BUILD_PATH)/$(GOOS)
	cd $(LIBRARY_PATH) && GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=1 CC=$(CC) go build -o ../$(BUILD_PATH)/$(GOOS)/$(LIBRARY_NAME) -buildmode=$(MODE) export.go main.go


test: 
	cd lib && go test ./...

mac: $(BUILD_PATH)/darwin/libbao_arm64.dylib
	echo "Build for macOS ARM64"
ios: $(BUILD_PATH)/ios/bao.xcframework
	echo "Build for iOS ARM64"
windows: $(BUILD_PATH)/windows/bao_amd64.dll
	echo "Build for Windows AMD64"
linux: $(BUILD_PATH)/linux/libbao_amd64.so
	echo "Build for Linux AMD64"
android: $(BUILD_PATH)/android/libbao_arm64.so
	echo "Build for Android ARM64"
wasm: $(BUILD_PATH)/wasm/bao.wasm $(BUILD_PATH)/wasm/wasm_exec.js
	echo "Build for WASM (browser)"

.PHONY: web
web: wasm
	echo "Starting local web server at http://localhost:$(WEB_PORT)/wasm/index.html"
	python3 -m http.server $(WEB_PORT)
	
lib: mac ios windows linux android
	echo "Build all libraries"

magpie: 
	echo "Build library and copy to magpie folders"
	cp $(BUILD_PATH)/darwin/libbao_arm64.dylib $(MAGPIE_PATH)/macos/Runner/libbao_arm64.dylib
	mkdir -p $(MAGPIE_PATH)/ios/Frameworks
	cp -rf $(BUILD_PATH)/ios/bao.xcframework $(MAGPIE_PATH)/ios/Frameworks
	cp $(BUILD_PATH)/linux/libbao_amd64.so $(MAGPIE_PATH)/linux/libbao_amd64.so
	cp $(BUILD_PATH)/windows/bao_amd64.dll $(MAGPIE_PATH)/windows/bao_amd64.dll

py: lib
	echo "Building Python bindings"
	cd ./bindings/py && ./build.sh

java: lib
	echo "Building Java bindings"
	cd ./bindings/java && mvn clean install

dart: lib
	echo "Building Dart bindings"
	cd ./bindings/dart && ./build.sh

clean:
	echo "Cleaning up"
	rm -rf $(BUILD_PATH)
	rm -rf bindings/py/pbao/_libs

release:
	@read -p "Enter the release name: " release_name; \
		date_suffix=$$(date +"%d%m%y"); \
		full_release_name=$$release_name-$$date_suffix; \
		echo "Creating zip files for each directory in $(BUILD_PATH)"; \
		for dir in $(shell find $(BUILD_PATH) -mindepth 1 -maxdepth 1 -type d); do \
			dir_name=$$(basename $$dir); \
			zip_file=$(BUILD_PATH)/bao_$$dir_name.zip; \
			echo "Packaging $$dir into $$zip_file"; \
			(cd $$dir && zip -r ../bao_$$dir_name.zip .); \
		done; \
		echo "Creating Git release: $$full_release_name"; \
		git tag -a $$full_release_name -m "Release $$full_release_name"; \
		git push origin $$full_release_name; \
		echo "Attaching zip files to the release"; \
		gh release create $$full_release_name $(shell find $(BUILD_PATH) -name "bao_*.zip") --title "$$full_release_name" --notes "Release $$full_release_name with libraries."

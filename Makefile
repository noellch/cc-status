.PHONY: build bundle install uninstall clean run

VERSION   := $(or $(shell git describe --tags --abbrev=0 2>/dev/null | sed 's/^v//'),0.1.0)
# Must match platforms[.macOS(.v13)] in Package.swift
MIN_MACOS := 13.0

APP_BUNDLE = .build/CCStatus.app
CONTENTS   = $(APP_BUNDLE)/Contents
MACOS_DIR  = $(CONTENTS)/MacOS
HOOK_DIR   = $(HOME)/.local/bin

build:
	swift build -c release

bundle: build
	mkdir -p $(MACOS_DIR)
	cp .build/release/CCStatus $(MACOS_DIR)/CCStatus
	/usr/libexec/PlistBuddy -c "Clear dict" $(CONTENTS)/Info.plist 2>/dev/null || true
	/usr/libexec/PlistBuddy \
		-c "Add :CFBundleExecutable string CCStatus" \
		-c "Add :CFBundleIdentifier string com.crescendolab.cc-status" \
		-c "Add :CFBundleName string 'CC Status'" \
		-c "Add :CFBundlePackageType string APPL" \
		-c "Add :CFBundleVersion string $(VERSION)" \
		-c "Add :CFBundleShortVersionString string $(VERSION)" \
		-c "Add :LSMinimumSystemVersion string $(MIN_MACOS)" \
		-c "Add :LSUIElement bool true" \
		$(CONTENTS)/Info.plist
	codesign --sign - $(APP_BUNDLE)

install: bundle
	-pkill -x CCStatus && sleep 1
	mkdir -p $(HOME)/Applications
	rm -rf $(HOME)/Applications/CCStatus.app
	cp -R $(APP_BUNDLE) $(HOME)/Applications/CCStatus.app
	codesign --sign - $(HOME)/Applications/CCStatus.app
	mkdir -p $(HOOK_DIR)
	cp .build/release/CCStatusHook $(HOOK_DIR)/cc-status-hook
	$(HOOK_DIR)/cc-status-hook install
	@echo "CC Status installed! Launch from ~/Applications/CCStatus.app"

uninstall:
	-cc-status-hook uninstall
	rm -rf $(HOME)/Applications/CCStatus.app
	rm -f $(HOOK_DIR)/cc-status-hook
	rm -rf $(HOME)/.cc-status
	@echo "CC Status uninstalled."

clean:
	swift package clean
	rm -rf .build/CCStatus.app

run: bundle
	-pkill -x CCStatus && sleep 1
	open .build/CCStatus.app

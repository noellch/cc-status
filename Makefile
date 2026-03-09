.PHONY: build bundle install uninstall clean run

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
		-c "Add :CFBundleVersion string 1.0" \
		-c "Add :LSUIElement bool true" \
		$(CONTENTS)/Info.plist

install: bundle
	-pkill -x CCStatus && sleep 1
	mkdir -p $(HOME)/Applications
	rm -rf $(HOME)/Applications/CCStatus.app
	cp -R $(APP_BUNDLE) $(HOME)/Applications/CCStatus.app
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

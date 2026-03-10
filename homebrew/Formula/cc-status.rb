class CcStatus < Formula
  desc "System tray monitor for Claude Code sessions"
  homepage "https://github.com/noellch/cc-status"
  url "https://github.com/noellch/cc-status/archive/refs/tags/v0.1.0.tar.gz"
  sha256 "3e55ee3b22f26cbe8d3e3563e071ab0c0cce67bf5c1905493bdac2727ee7c192"
  license "MIT"

  on_macos do
    def install
      system "swift", "build", "-c", "release", "--disable-sandbox"
      bin.install ".build/release/CCStatusHook" => "cc-status-hook"

      # Create app bundle
      app_dir = prefix/"CCStatus.app/Contents"
      (app_dir/"MacOS").mkpath
      cp ".build/release/CCStatus", app_dir/"MacOS/CCStatus"
      (app_dir/"Info.plist").write <<~XML
        <?xml version="1.0" encoding="UTF-8"?>
        <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
        <plist version="1.0">
        <dict>
          <key>CFBundleExecutable</key><string>CCStatus</string>
          <key>CFBundleIdentifier</key><string>com.crescendolab.cc-status</string>
          <key>CFBundleName</key><string>CC Status</string>
          <key>CFBundleVersion</key><string>#{version}</string>
          <key>LSUIElement</key><true/>
        </dict>
        </plist>
      XML
    end
  end

  on_linux do
    depends_on "go" => :build
    depends_on "pkg-config" => :build
    depends_on "gtk+3"

    def install
      cd "cc-status-go" do
        system "go", "build", *std_go_args(ldflags: "-s -w", output: bin/"cc-status-tray"), "./cmd/cc-status-tray"
        system "go", "build", *std_go_args(ldflags: "-s -w", output: bin/"cc-status-hook"), "./cmd/cc-status-hook"
      end
    end
  end

  def caveats
    <<~EOS
      Register the Claude Code hooks:
        cc-status-hook install

      Launch the app:
        #{"open #{opt_prefix}/CCStatus.app" if OS.mac?}
        #{"cc-status-tray &" if OS.linux?}
    EOS
  end

  test do
    assert_predicate bin/"cc-status-hook", :exist?
  end
end

class Fastlogin < Formula
  desc "Interactive terminal command launcher (SSH + commands)"
  homepage "https://github.com/@REPO@"
  version "@VERSION@"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/@REPO@/releases/download/@TAG@/fastlogin_@TAG@_darwin_arm64.tar.gz"
      sha256 "@ARM@"
    end

    on_intel do
      url "https://github.com/@REPO@/releases/download/@TAG@/fastlogin_@TAG@_darwin_amd64.tar.gz"
      sha256 "@INTEL@"
    end
  end

  def install
    bin.install "fastlogin"
  end

  test do
    assert_match "fastlogin", shell_output("#{bin}/fastlogin 2>&1", 1)
  end
end

class LetMeIn < Formula
  desc "Add my IP to AWS security group(s)"
  homepage "https://github.com/rlister/let-me-in"
  url "https://github.com/rlister/let-me-in/archive/v0.0.2.tar.gz"
  sha256 "694f6b51134dfa2bf2c8d316283c525d24e51d0b52ef501668045359ea7a0808"

  depends_on "go" => :build

  def install
    ENV["GOPATH"] = buildpath
    system "go", "get", "github.com/aws/aws-sdk-go/aws"
    system "go", "get", "github.com/aws/aws-sdk-go/aws/awserr"
    system "go", "get", "github.com/aws/aws-sdk-go/service/ec2"
    system "go", "build", "let-me-in.go"
    bin.install "let-me-in"
  end

  test do
    ## I should write some tests
  end
end

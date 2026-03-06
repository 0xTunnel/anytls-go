group "default" {
  targets = ["image-local"]
}

target "_common" {
  context    = "."
  dockerfile = "Dockerfile"
  args = {
    GO_VERSION     = "1.24"
    ALPINE_VERSION = "3.21"
  }
}

target "image-local" {
  inherits  = ["_common"]
  tags      = ["anytls-ppanel:latest"]
  platforms = ["linux/amd64"]
  output    = ["type=docker"]
}

target "image-multiarch" {
  inherits  = ["_common"]
  tags      = ["anytls-ppanel:latest"]
  platforms = ["linux/amd64", "linux/arm64"]
}
# Configure for Microsoft Azure Government (US Government) or China cloud.
provider "fabricext" {
  environment = "usgovernment" # Options: "public" (default), "usgovernment", "china"
  use_cli     = true
}

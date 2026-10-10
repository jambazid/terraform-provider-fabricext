# Authenticate using a Microsoft Entra Service Principal with a Client Certificate (PKCS#12 / PFX).
provider "fabricext" {
  client_id                    = "00000000-0000-0000-0000-000000000001"
  tenant_id                    = "00000000-0000-0000-0000-000000000000"
  client_certificate_file_path = "/path/to/certificate.pfx"
  client_certificate_password  = "cert-password"
}

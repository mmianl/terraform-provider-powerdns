provider "powerdns" {
  server_url          = "http://pdns:8081"     # PDNS_SERVER_URL=http://localhost:8081 to override
  recursor_server_url = "http://recursor:8082" # PDNS_RECURSOR_SERVER_URL=http://localhost:8082 to override
  api_key             = "testapikey"
}

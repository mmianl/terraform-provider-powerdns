provider "powerdns" {
  server_url          = "http://pdns:8081"     # or PDNS_SERVER_URL=http://localhost:8081
  recursor_server_url = "http://recursor:8082" # or PDNS_RECURSOR_SERVER_URL=http://localhost:8082
  api_key             = "testapikey"
}

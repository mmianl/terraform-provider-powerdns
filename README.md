# terraform-provider-powerdns

The Terraform PowerDNS provider allows you to manage PowerDNS zones and records using Terraform. It is maintained by mmianl.

- Website: https://www.terraform.io
- [![Gitter chat](https://badges.gitter.im/hashicorp-terraform/Lobby.png)](https://gitter.im/hashicorp-terraform/Lobby)
- Mailing list: [Google Groups](http://groups.google.com/group/terraform-tool)

## Requirements

- [Terraform](https://www.terraform.io/downloads.html) 1.11.x
- [Go](https://golang.org/doc/install) >=1.24.x (to build the provider plugin)
- [Goreleaser](https://goreleaser.com) >=v6.3.x (for releasing provider plugin)

The Go ang Goreleaser minimum versions were set to be able to build plugin for Darwin/ARM64 architecture [see goreleaser notes.](https://goreleaser.com/deprecations/#builds-for-darwinarm64)

## Using the Provider

```hcl
terraform {
  required_providers {
    powerdns = {
      source = "mmianl/powerdns"
      version = "1.8.1"
    }
  }
}

provider "powerdns" {
  server_url = "https://host:port/"           # authoritative server url (can also be provided with PDNS_SERVER_URL variable)
  recursor_server_url = "https://host:port/"  # recursor server url (can also be provided with PDNS_RECURSOR_SERVER_URL variable)
  dnsdist_server_url = "https://host:port/"   # DNSdist server url (can also be provided with PDNS_DNSDIST_SERVER_URL variable)
  api_key             = "secret"              # can also be provided with PDNS_API_KEY variable
}

# Note: The provider supports PowerDNS Authoritative Server, PowerDNS Recursor, and PowerDNS DNSdist.
# Configure server_url for authoritative operations, recursor_server_url for recursor operations,
# and dnsdist_server_url for DNSdist operations.
```

For detailed usage see [provider's documentation page](https://registry.terraform.io/providers/mmianl/powerdns/latest/docs)

## Environment Variables

The provider supports configuration via environment variables as an alternative to the provider block configuration:

- `PDNS_SERVER_URL` - The URL of the PowerDNS Authoritative Server (e.g., `https://host:port/`)
- `PDNS_API_KEY` - The API key for authenticating with the PowerDNS server
- `PDNS_RECURSOR_SERVER_URL` - The URL of the PowerDNS Recursor Server (e.g., `https://host:port/`)
- `PDNS_DNSDIST_SERVER_URL` - The URL of the PowerDNS DNSdist Server (e.g., `https://host:port/`)

When these environment variables are set, you can use the provider without explicit configuration:

```hcl
provider "powerdns" {}
```

## Building The Provider

Clone the provider repository:

```sh
$ git clone git@github.com:mmianl/terraform-provider-powerdns.git

Navigate to repository directory:

```sh
$ cd terraform-provider-powerdns
```

Build repository:

```sh
$ go build
```

This will compile and place the provider binary, `terraform-provider-powerdns`, in the current directory.

## Developing the Provider

If you wish to work on the provider, you'll first need [Go](http://www.golang.org) installed on your machine (version 1.11+ is _recommended_).
You'll also need to have `$GOPATH/bin` in your `$PATH`.

Make sure the changes you performed pass linting:

```sh
$ make lint
```

To install the provider, run `make build`. This will build the provider and put the provider binary in the current working directory.

```sh
$ make build
```

In order to run local provider tests, you can simply run `make test`.

```sh
$ make test
```

For running acceptance tests locally, you'll need to use `docker-compose` to prepare the test environment:

```sh
docker-compose run --rm setup
```

After setup is done, run the acceptance tests with `make testacc` (note the env variables needed to interact with the PowerDNS container)

- HTTP

```sh
~$  PDNS_SERVER_URL=http://localhost:8081 \
    PDNS_API_KEY=secret \
    make testacc
```

- HTTPS

```sh
~$  PDNS_SERVER_URL=localhost:4443 \
    PDNS_API_KEY=secret \
    PDNS_CACERT=$(cat ./tests/files/ssl/rootCA/rootCA.crt) \
    make testacc
```

And finally cleanup containers spun up by `docker-compose`:

```sh
~$ docker-compose down
```

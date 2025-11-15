FROM golang:1.24 AS tests

WORKDIR /tests
COPY go.mod .
COPY assert_test.go .
RUN CGO_ENABLED=0 go test -c -o assert.test ./...

FROM hashicorp/terraform:1.13
ARG TF_PLUGIN_PLATFORM=linux_amd64

WORKDIR /tf

# Copy terraform config and provider binary
RUN mkdir -p /usr/local/share/terraform/plugins/registry.terraform.io/mmianl/powerdns/999.0.0/${TF_PLUGIN_PLATFORM}
COPY .terraformrc /root/.terraformrc
COPY terraform-provider-powerdns \
  /usr/local/share/terraform/plugins/registry.terraform.io/mmianl/powerdns/999.0.0/${TF_PLUGIN_PLATFORM}/terraform-provider-powerdns

# Copy terraform code
COPY terraform .

# Copy assertion tests
COPY --from=tests /tests/assert.test .

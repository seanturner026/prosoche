_default:
  just --list --alias-style left --list-heading ''

set dotenv-load
set dotenv-path := ".env"
set quiet

lambdas := "daily"
tier := `terraform -chdir=terraform workspace show`

# go
# -------------------------------------------------------------------
alias b := build
[doc("build all lambda binaries")]
[group('go')]
build:
  #!/usr/bin/env bash
  set -euo pipefail
  for name in {{ lambdas }}; do
    echo "building $name..."
    mkdir -p bin/$name
    GOARCH=arm64 GOOS=linux go build -ldflags="-s -w" -o bin/$name/bootstrap ./cmd/$name/
  done

alias t := test
[doc("run all tests")]
[group('go')]
test:
  go test ./...

alias p := preview
[doc("print a day's message without sending — just preview 2026-10-20")]
[group('go')]
preview date=`date +%F`:
  go run ./cmd/daily/ -preview {{ date }}

alias r := run
[doc("send today's passage to Discord using .env credentials")]
[group('go')]
run:
  go run ./cmd/daily/

[doc("rebuild internal/passage/passages.json from Project Gutenberg")]
[group('go')]
ingest:
  go run ./cmd/ingest/

# terraform
# -------------------------------------------------------------------
alias d := deploy
[doc("build all lambdas and deploy with terraform")]
[group('terraform')]
deploy: build
  terraform -chdir=terraform apply -var-file=var.{{ tier }}.tfvars

[doc("select or create a workspace — just workspace staging")]
[group('terraform')]
workspace tier:
  terraform -chdir=terraform workspace select {{ tier }} || terraform -chdir=terraform workspace new {{ tier }}

[doc("invoke the deployed lambda, sending today's passage")]
[group('terraform')]
invoke:
  aws lambda invoke --function-name prosoche-{{ tier }}-daily /dev/stdout

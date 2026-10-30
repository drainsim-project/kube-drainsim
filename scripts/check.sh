#!/usr/bin/env bash
# Runs the checks that CI runs. Usage: bash scripts/check.sh [step...]
# Steps: fmt vet test lint workflows (default: all). Set RACE=0 to test
# without the race detector, for example on Windows without mingw-w64.
set -euo pipefail

cd "$(dirname "$0")/.."

# Keep golangci-lint in sync with .github/workflows/ci.yml.
golangci_lint_version=v2.15.0
actionlint_version=v1.7.12

check_fmt() {
  local go_root unformatted
  go_root="$(go env GOROOT)"
  unformatted="$("$go_root/bin/gofmt" -l .)"
  if [[ -n $unformatted ]]; then
    printf '%s\n' "$unformatted"
    echo 'Run "$(go env GOROOT)/bin/gofmt" -w on the files above.' >&2
    return 1
  fi
}

check_vet() {
  go vet ./...
}

check_test() {
  if [[ ${RACE:-1} == 0 ]]; then
    go test -count=1 ./...
  else
    go test -race -count=1 ./...
  fi
}

check_lint() {
  go run "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${golangci_lint_version}" run
}

check_workflows() {
  go run "github.com/rhysd/actionlint/cmd/actionlint@${actionlint_version}"
}

steps=("$@")
if [[ ${#steps[@]} -eq 0 ]]; then
  steps=(fmt vet test lint workflows)
fi

packages_checked=false
packages=

for step in "${steps[@]}"; do
  case $step in
    fmt | vet | test | lint)
      if [[ $packages_checked == false ]]; then
        packages="$(go list ./...)"
        packages_checked=true
      fi
      if [[ -z $packages ]]; then
        echo "==> $step (skipped: no Go packages)"
        continue
      fi
      ;;
    workflows)
      ;;
    *)
      echo "unknown step: $step (want fmt, vet, test, lint, workflows)" >&2
      exit 2
      ;;
  esac
  echo "==> $step"
  "check_$step"
done

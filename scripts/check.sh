#!/usr/bin/env bash
# CI와 같은 검사를 실행합니다. 사용법: bash scripts/check.sh [단계...]
# 단계: tidy fmt vet test lint workflows (인자를 생략하면 전체 검사 실행).
# mingw-w64가 없는 Windows 등에서 경쟁 상태 검사 없이 테스트하려면 RACE=0을 설정합니다.
set -euo pipefail

cd "$(dirname "$0")/.."

# golangci-lint 버전을 .github/workflows/ci.yml과 동일하게 유지합니다.
golangci_lint_version=v2.15.0
actionlint_version=v1.7.12

check_tidy() {
  local diff
  if diff="$(go mod tidy -diff)"; then
    return 0
  fi
  # 모듈 다운로드 실패 같은 Go 실행 오류에는 변경 내역이 출력되지 않습니다.
  if [[ -n $diff ]]; then
    printf '%s\n' "$diff"
    echo 'Run "go mod tidy" and commit go.mod and go.sum.' >&2
  fi
  return 1
}

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
  steps=(tidy fmt vet test lint workflows)
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
    tidy | workflows)
      ;;
    *)
      echo "unknown step: $step (want tidy, fmt, vet, test, lint, workflows)" >&2
      exit 2
      ;;
  esac
  echo "==> $step"
  "check_$step"
done

# 개발 환경

drainsim 코드를 작성하거나 리뷰할 때 필요한 도구와 로컬 검사 방법을 설명합니다.

## 저장소 준비

```sh
git clone https://github.com/drainsim-project/kube-drainsim.git
cd kube-drainsim
```

브랜치와 PR 규칙은 [협업 안내](../.github/CONTRIBUTING.md)를 따릅니다.

## 필요한 도구

| 도구 | 쓰는 곳 | 확인 방법 |
| --- | --- | --- |
| Git, Bash | 변경 관리, 검사 스크립트 실행 | `git --version`, `bash --version` |
| Go | 빌드와 테스트 | `go version` |
| Docker(Linux 컨테이너) | 로컬 kind 검증 | `docker info` |
| kind | 폐기 가능한 Kubernetes 클러스터 | `kind version` |
| kubectl | 검증 환경 확인과 drain 재현 | `kubectl version --client` |

Go 버전은 [go.mod](../go.mod)를 따릅니다. 설치된 Go가 더 낮으면 기본 설정(`GOTOOLCHAIN=auto`)에서 필요한 버전을 자동으로 받습니다. Windows에서는 Git for Windows에 들어 있는 Git Bash로 스크립트를 실행합니다.

## 로컬 검사

변경과 관련된 검사를 로컬에서 실행합니다. 문서만 바꾸면 내용과 링크, 예시 명령을 확인합니다. 전체 검사는 저장소 루트에서 다음과 같이 실행합니다.

```sh
bash scripts/check.sh
```

포맷 검사, `go vet`, race detector를 켠 테스트, golangci-lint, actionlint를 차례로 실행하며, CI와 같은 명령을 씁니다. golangci-lint와 actionlint는 스크립트가 `go run`으로 받아 실행하므로 따로 설치하지 않아도 됩니다. 처음 실행할 때는 빌드 때문에 몇 분 걸릴 수 있습니다.

Go 패키지가 없으면 Go 관련 검사를 건너뛰고 actionlint는 실행합니다. 패키지를 추가하면 Go 검사가 자동으로 실행됩니다. 패키지 조회 자체가 실패하면 검사도 실패합니다.

일부 단계만 실행하려면 단계 이름을 붙입니다. 예: `bash scripts/check.sh fmt test`

race detector에는 cgo와 64비트 C 컴파일러(Windows에서는 mingw-w64)가 필요합니다. 32비트 MinGW만 있으면 `64-bit mode not compiled in` 오류가 납니다. 이때는 `RACE=0 bash scripts/check.sh`로 race 없이 테스트하고, race 검사는 CI 결과로 확인합니다.

## 테스트 환경

테스트 종류와 클러스터가 필요한 조건은 [테스트 기준](testing.md#테스트의-구분), kind 재현 절차는 [로컬 Kubernetes 검증](local-cluster.md)을 참고합니다.

## Windows에서 확인할 사항

- Docker Desktop 등의 Linux 컨테이너 엔진이 실행 중인지 `docker info`로 확인합니다.
- WSL을 쓴다면 Go, kind, kubectl, Docker를 같은 셸 환경에서 실행합니다. Windows 경로와 WSL 경로, 서로 다른 kubeconfig를 섞지 않습니다.
- Windows PowerShell 5.1에서 `kubectl ... -o json > pods.json`처럼 저장하면 UTF-16 파일이 되고, `-Encoding utf8`을 붙여도 BOM이 들어가 Go의 JSON 파서가 읽지 못합니다. 테스트 입력은 Git Bash나 PowerShell 7에서 저장합니다.

## 참고

- [Go 도구체인 선택](https://go.dev/doc/toolchain)
- [Go race detector](https://go.dev/doc/articles/race_detector)
- [kind의 WSL2 안내](https://kind.sigs.k8s.io/docs/user/using-wsl2/)

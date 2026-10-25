# 저장소 운영 안내

이 문서는 GitHub에 설정한 협업 규칙과 유지관리자가 확인할 항목을 기록합니다. 현재 저장소에는 협업 설정과 Go 모듈(`go.mod`)이 있으며 개발 코드는 아직 없습니다.

## PR과 병합

- `main` 변경은 기능 브랜치의 PR로 제안합니다.
- 일반적인 병합에는 승인 1명, 리뷰 대화 해결, 필수 검사 통과가 필요합니다. 새 커밋이 올라오면 기존 승인은 해제됩니다.
- 필수 검사는 `Repository configuration`, `PR title`, `Workflow review`, `Go checks`입니다. 병합 전에 브랜치를 최신 `main`에 맞춥니다.
- squash merge를 사용하며 병합 후 작업 브랜치는 자동 삭제됩니다. `main`의 삭제와 강제 push는 차단합니다.
- 팀 구성 전 초기 운영을 위해 관리자에게 PR을 통한 규칙 우회 권한을 두었습니다. 일반 push에는 이 예외가 적용되지 않습니다. 예외를 사용해 병합할 때도 실행된 검사를 확인하고, 팀원이 합류하면 예외가 계속 필요한지 검토합니다.

규칙은 [Main branch protection](https://github.com/kang0225/kube-drainsim/rules/25827265)에서 확인할 수 있습니다. 저장소 설정을 변경하면 이 문서도 함께 수정합니다.

## 자동 검사와 리뷰

- Repository checks는 YAML과 이슈 양식의 구조, PR 제목을 검사합니다.
- Reviewdog는 actionlint로 워크플로를 검사하고 발견한 내용을 Actions와 PR의 파일 주석으로 표시합니다. AI 리뷰나 사람의 승인을 대신하지 않습니다.
- Go CI는 `go.mod`에 지정한 Go 버전을 설정하고 Go 패키지를 확인합니다. 패키지가 있으면 `go vet`, race detector를 사용한 테스트와 Go 정적 분석을 실행하며, 패키지가 없는 현재는 생략 이유를 실행 요약에 남깁니다. 패키지 확인 명령 자체에 오류가 나면 검사를 생략하지 않고 실패로 처리합니다. 포맷 검사는 Go 모듈이 있으면 실행합니다.
- Actions는 기본적으로 저장소 읽기 권한만 사용합니다. 처음 참여하는 외부 기여자의 워크플로는 관리자의 실행 승인이 필요할 수 있습니다.
- Actions 참조는 전체 커밋 SHA로 고정합니다. Dependabot은 매주 월요일 오전 9시(한국 시간)에 Actions 업데이트를 확인하고, 업데이트가 있으면 묶어서 PR을 제안합니다.

자동 검사 결과와 함께 사람이 변경 의도, 정확성, 검증 근거를 확인합니다. 실제 drain 검증 결과는 별도로 남깁니다.

## 작업 분류와 참여 안내

[Issues](https://github.com/kang0225/kube-drainsim/issues)의 양식은 버그 제보, 기능·설계 제안, drain 검증 시나리오로 구성합니다. 질문과 초기 아이디어는 [Discussions](https://github.com/kang0225/kube-drainsim/discussions)에서 나눕니다.

- `area/*`는 데이터 수집, 분석 엔진, CLI, 검증, CI의 작업 영역을 표시합니다.
- `priority/p0`, `priority/p1`, `priority/p2`는 우선순위를 표시합니다.
- `needs-triage`는 아직 분류가 필요한 제안, `blocked`는 선행 작업이나 결정이 필요한 작업에 사용합니다.
- `good first issue`와 `help wanted`는 참여를 구하는 작업에 사용합니다.
- 마일스톤은 `M0 - Problem validation`, `v0.1 - Single-node preflight`, `v0.2 - Drain scenario comparison`입니다. 일정과 범위는 팀에서 정합니다.

현재 코드 소유자는 `@kang0225`입니다. 팀원이 합류하면 역할에 맞춰 `CODEOWNERS`와 저장소 권한을 검토합니다. 자세한 참여 흐름은 [기여 안내](CONTRIBUTING.md)를 따릅니다.

## 보안

Dependabot 보안 알림과 보안 업데이트, Secret scanning과 push protection을 활성화했습니다. 비공개 취약점 제보도 받을 수 있습니다. 제보 방법은 [보안 안내](SECURITY.md)를 확인해 주세요.

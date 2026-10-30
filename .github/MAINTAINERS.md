# 저장소 운영 안내

이 문서는 GitHub에 설정한 저장소 규칙과 자동 검사를 기록합니다. 설정을 바꾸면 같은 PR에서 이 문서도 고칩니다.

## PR과 병합

`main`에는 [Main branch protection](https://github.com/drainsim-project/kube-drainsim/rules/25827265) 규칙이 적용됩니다.

- `main` 변경은 PR로만 반영하며, 관리자도 이 규칙을 우회하지 않습니다.
- 병합하려면 다른 팀원 1명의 승인, 리뷰 대화 해결, 필수 검사 통과가 필요합니다. 새 커밋을 올리면 기존 승인은 해제됩니다.
- 필수 검사는 `PR title`, `Workflow review`, `Go checks`입니다. 브랜치를 최신 `main`에 맞추는 것은 병합 조건이 아닙니다.
- squash merge만 허용하고 PR 제목을 커밋 제목으로 씁니다. 병합한 브랜치는 자동으로 삭제되고, `main`의 삭제와 강제 push는 막혀 있습니다.

## 자동 검사와 리뷰

| 검사 | 워크플로 | 내용 |
| --- | --- | --- |
| `PR title` | [pr-title.yml](workflows/pr-title.yml) | PR 제목의 Conventional Commits 형식 |
| `Workflow review` | [ci.yml](workflows/ci.yml) | actionlint로 워크플로 검사 |
| `Go checks` | [ci.yml](workflows/ci.yml) | gofmt, `go vet`, race detector를 켠 테스트, golangci-lint |

- 검사 명령은 [scripts/check.sh](../scripts/check.sh)에 모아 로컬과 CI가 함께 씁니다. golangci-lint만 CI에서 공식 action으로 실행하며, 버전은 스크립트와 맞춥니다.
- Actions는 저장소 읽기 권한만 쓰고, 참조는 전체 커밋 SHA로 고정합니다. 업데이트는 Dependabot이 묶어서 PR로 올립니다.
- CodeRabbit은 PR마다 변경 요약과 리뷰 코멘트를 남기며, 설정은 [.coderabbit.yaml](../.coderabbit.yaml)에 있습니다. 자동 승인을 꺼서 병합 승인에는 포함되지 않지만, 코멘트는 리뷰 대화라서 해결해야 병합할 수 있습니다. golangci-lint와 actionlint는 CI가 실행하므로 CodeRabbit에서는 껐습니다.

## 이슈 분류

이슈 양식은 버그 제보, 기능 제안, drain 검증 시나리오 세 가지이며, 양식마다 `bug`, `enhancement`, `scenario` 라벨이 자동으로 붙습니다. 양식에 맞지 않는 작업을 위해 빈 이슈도 허용합니다.

- `blocked`는 선행 작업이나 결정을 기다리는 이슈에 붙입니다.
- 그 밖에는 GitHub 기본 라벨(`documentation`, `question`, `duplicate`, `invalid`, `wontfix`)만 씁니다. Dependabot PR에는 라벨을 붙이지 않습니다.

## 보안

Dependabot 보안 알림과 보안 업데이트, Secret scanning과 push protection을 켰습니다.

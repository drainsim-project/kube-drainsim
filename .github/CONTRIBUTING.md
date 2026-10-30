# 협업 안내

drainsim 팀원이 작업을 제안하고 PR로 반영하는 절차를 설명합니다.

## 이슈

- 큰 기능은 구현 전에 이슈에서 해결할 문제와 범위를 먼저 정합니다.
- 재현 가능한 문제는 버그 양식, 새로운 동작은 기능 제안 양식, 실제 drain과 비교할 사례는 drain 검증 시나리오 양식을 사용합니다. 조사나 셋업처럼 양식에 맞지 않는 작은 작업은 빈 이슈로 올립니다.
- 작업을 시작할 때 이슈에 구현 방향을 남겨 중복 작업을 피합니다.

## PR 흐름

1. 작업 브랜치를 만듭니다. 예: `feat/pdb-report`, `fix/report-message`, `docs/scenario-guide`. 작은 오탈자 수정은 이슈 없이 PR을 보내도 됩니다.
2. 한 PR에는 검토 가능한 크기의 변경을 담고, 진행 중인 작업은 draft PR로 공유합니다.
3. PR 제목은 `feat: PDB 분석 추가`, `fix: 보고서 오류 수정`, `docs: 검증 안내 보완`처럼 Conventional Commits 형식으로 적습니다.
4. 본문은 PR 템플릿을 따르고, 검증 결과는 템플릿의 체크리스트로 남깁니다.
5. 팀원 리뷰와 승인은 권장하며 병합의 필수 조건은 아닙니다. 리뷰 대화는 의견을 반영하거나 이유를 남겨 해결합니다. CodeRabbit 리뷰도 같은 기준으로 처리합니다.
6. 필수 검사를 통과하면 squash merge합니다. PR 제목이 최종 커밋 제목이 됩니다.

일정이 바뀌거나 막히면 이슈나 PR에 상황과 필요한 도움을 남겨 주세요. 세부 병합 설정은 [GitHub 규칙](https://github.com/drainsim-project/kube-drainsim/rules/25827265)에서 확인합니다.

## 검증

로컬 검사 명령은 [개발 환경](../docs/development.md#로컬-검사), 테스트 선택과 실제 drain 비교 방법은 [테스트 기준](../docs/testing.md)에 있습니다.

| 필수 검사 | 내용 |
| --- | --- |
| [PR title](workflows/pr-title.yml) | PR 제목 형식 |
| [Workflow review](workflows/ci.yml) | actionlint로 워크플로 검사 |
| [Go checks](workflows/ci.yml) | Go 패키지가 있으면 포맷, 정적 분석, 테스트 |

## 보안

공개 저장소이므로 이슈, PR, 로그에 토큰, kubeconfig, Secret 값, 개인정보, 운영환경 정보를 올리지 마세요. 재현에는 익명화한 최소 예시를 씁니다.

Dependabot 보안 알림과 보안 업데이트, Secret scanning과 push protection을 사용합니다.

취약점 제보는 [비공개 취약점 신고](https://github.com/drainsim-project/kube-drainsim/security/advisories/new)를 사용합니다.

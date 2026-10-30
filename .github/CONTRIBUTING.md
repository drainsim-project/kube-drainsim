# 협업 안내

이 문서는 drainsim 팀원이 작업을 제안하고 PR로 반영하는 절차를 설명합니다. 이 저장소는 팀 프로젝트이며 외부 기여는 받지 않습니다.

## 이슈

- 큰 기능은 구현 전에 이슈에서 해결할 문제와 범위를 먼저 정합니다.
- 재현 가능한 문제는 버그 양식, 새로운 동작은 기능 제안 양식, 실제 drain과 비교할 사례는 drain 검증 시나리오 양식을 사용합니다. 조사나 셋업처럼 양식에 맞지 않는 작은 작업은 빈 이슈로 올립니다.
- 작업을 시작할 때 이슈에 구현 방향을 남겨 중복 작업을 피합니다.

## PR 흐름

1. 작업 브랜치를 만듭니다. 예: `feat/pdb-report`, `fix/report-message`, `docs/scenario-guide`. 작은 오탈자 수정은 이슈 없이 PR을 보내도 됩니다.
2. 한 PR에는 검토 가능한 크기의 변경을 담고, 진행 중인 작업은 draft PR로 공유합니다.
3. PR 제목은 Conventional Commits 형식으로 적습니다. 예: `feat: PDB 분석 추가`, `fix: 보고서 오류 수정`, `docs: 검증 안내 보완`. 종류는 `feat`, `fix`, `docs`, `chore`, `test`, `refactor`, `ci`, `build`, `perf`, `style`, `revert` 중에서 고르며, squash merge하면 PR 제목이 그대로 커밋 제목이 됩니다.
4. 본문은 PR 템플릿을 따르고, 검증 결과는 템플릿의 체크리스트로 남깁니다.
5. 다른 팀원 1명의 승인을 받고 리뷰 대화를 모두 해결합니다. CodeRabbit 코멘트도 리뷰 대화이므로 반영하거나, 반영하지 않는 이유를 답글로 남긴 뒤 해결합니다. CodeRabbit의 리뷰는 승인으로 세지 않습니다.
6. 승인과 필수 검사를 통과하면 squash merge합니다.

일정이 바뀌거나 막히면 이슈나 PR에 상황과 필요한 도움을 남겨 주세요. 병합 규칙과 자동 검사 설정은 [저장소 운영 안내](MAINTAINERS.md)에 있습니다.

## 검증

- PR을 올리기 전에 `bash scripts/check.sh`를 실행합니다. CI와 같은 포맷 검사, vet, 테스트, golangci-lint, actionlint를 실행합니다.
- 실제 drain 동작은 [로컬 Kubernetes 검증](../docs/local-cluster.md) 절차로 kind에서 확인합니다.
- 결과는 [설계 문서의 정확성 기준](../docs/design.md#정확성-기준)으로 판단합니다.

공개 저장소이므로 이슈, PR, 로그에 토큰, kubeconfig, Secret 값, 개인정보, 운영환경 정보를 올리지 마세요. 재현에는 익명화한 최소 예시를 씁니다.

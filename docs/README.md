# 개발 문서

drainsim 개발 문서의 목록과, 어떤 내용을 어디에 기록하는지 정리합니다.

## 문서 목록

| 파일 | 내용 |
| --- | --- |
| [design.md](design.md) | 해결할 문제, 범위, 판단 규칙, 결과 형식, 정확성 기준 |
| [development.md](development.md) | 저장소 준비, 필요한 도구, 로컬 검사 |
| [testing.md](testing.md) | 테스트 종류와 실제 drain 비교 절차 |
| [local-cluster.md](local-cluster.md) | kind에서 PDB 차단 사례를 재현하고 정리하는 방법 |

`examples/`의 YAML은 [local-cluster.md](local-cluster.md)에서 쓰는 입력입니다.

## 내용의 위치

| 내용 | 관리 위치 |
| --- | --- |
| 프로젝트 소개와 문서 진입점 | 메인 README |
| 작업 담당, 진행 상태, 완료 여부, 우선순위 | Issues |
| 설치, 개발, 테스트 절차 | 관련 `docs/` 파일 |
| 설계 제안과 의견 교환 | 관련 이슈 또는 PR |
| 결정한 설계와 이유 | `design.md` |
| 검증 결과와 로그 | PR의 검증 체크리스트, CI 실행 결과 |

작업 진행 상태는 이슈에만 기록하고 README나 문서에 옮겨 적지 않습니다.

## 갱신 기준

- 설치 방법, 실행 명령, 테스트 조건, 출력 형식이 바뀌면 같은 PR에서 관련 문서를 고칩니다.
- 아직 구현되지 않은 명령이나 출력은 제안이라고 표시합니다.
- 비밀번호, 토큰, kubeconfig, Secret 값, 운영환경 식별 정보는 문서와 예시에 넣지 않습니다.

# 테스트 기준

변경을 검증할 때 어떤 테스트를 고르고 무엇을 확인하는지 설명합니다. 결과는 [설계 문서의 정확성 기준](design.md#정확성-기준)으로 판단합니다.

## 테스트의 구분

| 종류 | 확인하는 내용 | 실제 클러스터 |
| --- | --- | --- |
| 분석 단위 테스트 | 주어진 Pod와 PDB 상태의 매칭, 판단 근거, `unknown` 처리 | 필요 없음 |
| 출력 테스트 | 터미널과 JSON 출력, 오류 처리 | 필요 없음 |
| 조회 코드 테스트 | API 요청과 응답 처리, 주입한 오류에 대한 동작 | fake client로 일부 확인 |
| 실제 동작 비교 | PDB, eviction, drain의 실제 결과와 분석의 일치 | 필요 |

일반 `go test`는 실제 클러스터에 접속하지 않아야 합니다. 클러스터가 필요한 통합 테스트는 그 테스트를 추가하는 PR에서 실행 방법과 함께 분리합니다.

## 예시 데이터와 fake client

- 예시 데이터에는 판단에 쓰는 필드와 상태만 넣고, 어떤 상황인지 테스트 이름이나 주석으로 밝힙니다.
- PDB 상태(`disruptionsAllowed`, `currentHealthy` 등)를 직접 적었다면 그 값은 컨트롤러가 계산한 결과가 아니라 테스트의 전제입니다.
- `client-go`의 fake client는 API 서버의 기본값 적용, 검증, 컨트롤러 동작을 흉내 내지 않습니다. fake client로는 매칭과 판단 로직까지만 확인하고, 실제 PDB와 drain 동작은 kind에서 확인합니다.

## 로컬 검사

```sh
bash scripts/check.sh
```

CI의 `Go checks`, `Workflow review`와 같은 명령입니다. 단계 선택과 Windows 옵션은 [개발 환경](development.md#로컬-검사)에 있습니다.

## 실제 동작 비교

[로컬 Kubernetes 검증](local-cluster.md)의 kind 환경에서 다음 순서로 확인합니다.

1. 분석 시점의 Pod 배치와 Ready 상태, PDB 상태를 확인합니다.
2. drain 전에 drainsim의 분석 결과와 근거를 기록합니다.
3. 실제 drain을 실행하고 종료 코드, eviction 거절 메시지, Pod 상태를 확인합니다.
4. drain 성공, 대체 Pod의 Ready, 서비스 가용성은 따로 확인합니다.
5. 예상과 실제의 차이를 정확성 기준의 분류로 기록합니다.

결과는 PR 본문의 검증 체크리스트([PR 템플릿](../.github/PULL_REQUEST_TEMPLATE.md))에 남깁니다. PR을 열면 체크리스트가 자동으로 들어갑니다.

## 참고

- [client-go fake client](https://pkg.go.dev/k8s.io/client-go/testing)
- [kubectl drain](https://kubernetes.io/docs/reference/kubectl/generated/kubectl_drain/)

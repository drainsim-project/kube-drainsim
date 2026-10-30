# drainsim

노드를 drain하기 전에 어떤 Pod의 eviction이 PodDisruptionBudget(PDB) 때문에 거절되는지, 그 근거와 함께 설명하는 kubectl 플러그인입니다. Go로 개발하며 `kubectl drainsim <node>`로 실행하는 것을 목표로 합니다.

## 목적

대상 노드의 Pod와 PDB를 연결해 판단 근거를 보여 주는 것이 목표입니다. 클러스터는 조회만 하며, 실제 drain은 별도의 재현 환경에서 검증합니다. `kubectl drain --dry-run=server`와의 차이와 판단 규칙은 [설계와 범위](docs/design.md)에 있습니다.

---

## 개발과 테스트

- [개발 환경](docs/development.md): 저장소 준비, 필요한 도구, 로컬 검사
- [테스트 기준](docs/testing.md): 클러스터 없는 테스트와 실제 동작 검증의 구분
- [로컬 Kubernetes 검증](docs/local-cluster.md): kind에서 PDB 차단 사례 재현
- [설계와 범위](docs/design.md): 해결할 문제, 판단 규칙, 결과 형식

---

## 협업

이 저장소는 팀 프로젝트이며 외부 기여는 받지 않습니다. 작업 제안과 구현 범위는 [Issues](https://github.com/drainsim-project/kube-drainsim/issues)에서 논의하고, 담당자, 진행 상태, 완료 여부도 이슈에서 관리합니다.

작업 절차와 코드 리뷰, 필수 검사 기준은 [협업 안내](.github/CONTRIBUTING.md)에 있습니다.

---

## 라이선스

[Apache License 2.0](LICENSE)

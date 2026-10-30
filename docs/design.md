# 설계와 범위

drainsim이 해결할 문제, 판단 규칙과 결과 형식을 정리합니다. **결정**으로 표시한 내용은 확정된 것이고, **제안**은 이슈에서 논의해 바꿀 수 있는 초안입니다. 판단 규칙은 Kubernetes v1.34의 eviction API와 `kubectl drain` 구현을 기준으로 합니다.

## 문제

노드 drain은 PodDisruptionBudget(PDB) 때문에 자주 멈춥니다. 다음 PDB는 Pod가 모두 정상이어도 퇴거를 허용하지 않습니다.

- replica 1개에 `minAvailable: 1`
- `maxUnavailable: 0`
- `minAvailable: 100%`

`kubectl drain`은 eviction이 허용될 때까지 재시도합니다. `--timeout`을 주지 않으면 끝나지 않고, 주면 timeout으로 실패합니다. 그 뒤에는 어떤 Pod와 PDB가 원인인지 eviction 오류 메시지와 PDB 상태를 직접 찾아봐야 합니다. drainsim은 drain 전에 이 정보를 모아 노드 단위로 설명합니다.

## `kubectl drain --dry-run=server`와의 차이

server dry-run도 eviction API를 거쳐 PDB를 확인하지만, Pod마다 따로 평가하고 PDB 예산 차감을 저장하지 않습니다. 같은 PDB에 묶인 Pod 2개가 한 노드에 있고 `disruptionsAllowed`가 1이면 dry-run은 둘 다 퇴거 가능으로 보여 줍니다. 실제 drain에서는 첫 eviction이 예산을 쓰므로, 예산이 회복될 때까지 두 번째 eviction은 429로 거절됩니다.

dry-run은 거절 여부만 알려 주고, 어떤 PDB의 어떤 값 때문에 거절되는지는 알려 주지 않습니다. drainsim은 그 근거를 보여 줍니다.

## 범위

**결정**

- 한 번에 노드 하나를 분석합니다.
- 클러스터는 조회만 하고, drain, cordon, 설정 변경은 하지 않습니다.
- kubectl 플러그인 `kubectl-drainsim`으로 만들어 `kubectl drainsim <node>`로 실행합니다.
- 분석 시점의 상태로 지금 eviction이 허용되는지와 그 근거만 판단합니다. 예산이 언제 회복될지, drain이 최종적으로 성공할지는 판단하지 않습니다.

**제안**

- 결과를 터미널 표와 JSON으로 출력합니다.

**범위 밖**

- 여러 노드의 drain 순서
- 대체 Pod가 스케줄될 수 있는지, 예산이 회복되는 시점, 서비스 가용성

## 판단 규칙 (제안)

### drain이 건너뛰거나 시작하지 않는 Pod

`kubectl drain`은 노드의 Pod를 표의 순서대로 거릅니다.

| Pod | `kubectl drain` 동작 | 결과 |
| --- | --- | --- |
| DaemonSet이 관리 | `--ignore-daemonsets`가 없으면 중단, 있으면 건너뜀 | `needs-flag` / `skipped` |
| mirror Pod(static Pod) | 항상 건너뜀 | `skipped` |
| emptyDir 사용 | `--delete-emptydir-data`가 없으면 중단, 있으면 퇴거 | `needs-flag` / eviction 규칙 |
| 컨트롤러가 없음 | `--force`가 없으면 중단, 있으면 퇴거 | `needs-flag` / eviction 규칙 |

`Succeeded`나 `Failed` 상태의 Pod는 DaemonSet, emptyDir, 컨트롤러 검사를 거치지 않고 퇴거 대상이 됩니다. DaemonSet이 이미 삭제된 Pod는 `--force`가 있어야 퇴거되고, 없으면 drain이 중단됩니다.

### eviction과 PDB

퇴거 대상 Pod는 eviction API의 검사 순서를 따릅니다. 오류 상태이거나 판단 근거가 부족하면 drainsim은 `unknown`으로 분류합니다.

1. `Pending`, `Succeeded`, `Failed` 상태의 Pod와 이미 삭제 중인 Pod는 PDB를 확인하지 않고 퇴거됩니다. → `evictable`
2. 같은 네임스페이스에서 selector가 Pod 라벨과 맞는 PDB를 찾습니다. 없으면 → `evictable`, 2개 이상이면 API가 500으로 거절합니다. → `blocked`
3. Ready가 아닌 Pod는 PDB 상태를 검사하기 전에 다음 예외를 확인합니다. 예외에 해당하면 예산을 쓰지 않고 퇴거되고(→ `evictable`), 해당하지 않으면 4번부터 이어서 판단합니다.
   - `unhealthyPodEvictionPolicy`가 `AlwaysAllow`
   - 기본값 `IfHealthyBudget`에서 `currentHealthy >= desiredHealthy`이고 `desiredHealthy > 0`
4. `status.observedGeneration`이 `metadata.generation`보다 작으면 API가 지금은 429로 거절하고, 상태 값으로 예산을 판단할 수 없습니다. → `unknown`
5. `disruptionsAllowed`가 음수이거나 `status.disruptedPods` 항목이 2,000개를 초과하면 API가 403으로 거절합니다. 정상적인 예산 부족과 구분합니다. → `unknown`
6. `DisruptionAllowed` 조건의 reason이 `SyncFailed`이면 PDB 컨트롤러가 동기화에 실패한 상태입니다. API가 이 조건을 직접 검사하지는 않지만, drainsim은 예산 부족의 원인을 단정하지 않습니다. 다른 상태 값이 서로 맞지 않아 원인을 판단할 수 없는 경우도 같습니다. → `unknown`
7. 위 검사를 통과하고 `disruptionsAllowed`가 0이면 API가 429로 거절합니다.
   - `desiredHealthy`가 `expectedPods` 이상이면 Pod가 `expectedPods`만큼 정상이어도 예산이 0입니다. → `blocked`
   - 그렇지 않으면 현재 예산이 부족한 상태입니다. → `no-budget`
8. `disruptionsAllowed`가 1 이상이면 eviction마다 1씩 줄어듭니다. 같은 PDB에 묶인 노드의 Pod 중 `disruptionsAllowed`개까지는 → `evictable`, 나머지는 → `no-budget`. 어느 Pod가 먼저 예산을 쓸지는 정해져 있지 않습니다.

정상 동기화 시 PDB 컨트롤러는 예산을 `currentHealthy - desiredHealthy`로 계산하고, 계산 결과가 0 이하이거나 `expectedPods`가 0 이하이면 0으로 둡니다. 동기화 실패 때도 예산을 0으로 두므로 숫자만으로 원인을 판단하지 않습니다. 또한 `disruptedPods`에 기록된 기존 Ready Pod는 일시적으로 `currentHealthy`에서 제외됩니다. 삭제되지 않은 Pod의 기록이 만료되면 새 Pod 생성이나 Ready 전환 없이도 다시 계산에 포함되어 예산이 회복될 수 있습니다.

## 결과 분류 (제안)

| 결과 | 의미 |
| --- | --- |
| `evictable` | 지금 eviction이 허용됨 |
| `no-budget` | 현재 PDB 예산이 부족해 eviction이 거절됨 |
| `blocked` | PDB 설정 때문에 eviction이 거절됨. 정상 Pod가 `expectedPods`만큼 있어도 예산이 0이거나, PDB가 2개 이상 걸림 |
| `needs-flag` | 표시한 `kubectl drain` 옵션이 없으면 drain이 Pod를 퇴거하기 전에 중단됨 |
| `skipped` | drain이 건드리지 않음 |
| `unknown` | PDB 오류 상태이거나 판단할 근거가 부족함 |

`no-budget`과 `blocked`는 지금 eviction이 거절되는 근거를 설명합니다. 예산이 회복될지, 언제 회복될지, drain이 최종적으로 성공할지는 판단하지 않습니다. `evictable`도 PDB가 막지 않는다는 뜻일 뿐, drain 전체의 성공이나 서비스 가용성을 보장하지 않습니다.

## 출력 예시 (제안)

```text
$ kubectl drainsim worker-1
NAMESPACE     POD                RESULT       REASON
shop          web-6d4f9c-abcde   evictable    PDB web: disruptionsAllowed=1
shop          web-6d4f9c-fghij   no-budget    PDB web: budget used by web-6d4f9c-abcde
payment       api-0              blocked      PDB api: disruptionsAllowed=0, desiredHealthy=1, expectedPods=1
kube-system   kube-proxy-x2k9q   skipped      DaemonSet (--ignore-daemonsets)
ops           debug              needs-flag   no controller (--force)
batch         report-7xk2p       unknown      PDB report: observedGeneration 2 < generation 3
```

```json
{
  "node": "worker-1",
  "pods": [
    {
      "namespace": "payment",
      "name": "api-0",
      "result": "blocked",
      "pdb": "api",
      "reason": "disruptionsAllowed=0, desiredHealthy=1, expectedPods=1"
    }
  ]
}
```

## 정확성 기준

- 가장 피할 오류는 `false-safe`입니다. 문제없다고 했는데 실제 drain이 막히는 경우입니다. 근거가 부족하면 `evictable` 대신 `unknown`으로 답합니다.
- 실제 drain과 비교한 결과는 일치, `unknown`, `false-safe`, `false-blocked`, 기타 불일치로 분류합니다. `false-blocked`는 막힌다고 했는데 실제 drain이 성공한 경우입니다.
- drain timeout만으로 PDB를 원인으로 보지 않습니다. eviction 거절 메시지와 PDB 상태를 같이 확인합니다.
- 사례 하나가 맞았다고 다른 환경에서도 맞다고 보지 않습니다. 비교 결과는 사례마다 PR에 남깁니다.

## 열린 질문

- server dry-run eviction을 보조 근거로 쓸지. 쓰려면 `pods/eviction` 생성 권한이 필요해서 조회 전용 원칙과 부딪힙니다.
- `--ignore-daemonsets` 같은 drain 옵션을 drainsim도 입력으로 받아 같은 기준으로 거를지
- 같은 PDB의 Pod 중 어느 Pod를 `evictable`로 표시할지
- JSON 필드 이름과 형식이 바뀔 때의 호환성 정책
- 여러 노드 분석을 언제 범위에 넣을지

## 참고

- [Kubernetes의 disruption과 PDB](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/)
- [PDB 설정과 비정상 Pod 퇴거 정책](https://kubernetes.io/docs/tasks/run-application/configure-pdb/)
- [API를 이용한 퇴거](https://kubernetes.io/docs/concepts/scheduling-eviction/api-eviction/)
- [kubectl drain](https://kubernetes.io/docs/reference/kubectl/generated/kubectl_drain/)
- 구현(v1.34.11): [eviction API](https://github.com/kubernetes/kubernetes/blob/v1.34.11/pkg/registry/core/pod/storage/eviction.go), [PDB 컨트롤러](https://github.com/kubernetes/kubernetes/blob/v1.34.11/pkg/controller/disruption/disruption.go), [kubectl drain 필터](https://github.com/kubernetes/kubernetes/blob/v1.34.11/staging/src/k8s.io/kubectl/pkg/drain/filters.go)

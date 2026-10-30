# 로컬 Kubernetes 검증

PDB 때문에 drain이 막히는 사례를 kind 클러스터에서 재현하고, PDB를 지운 대조군과 비교하는 절차입니다.

실제 drain을 실행하므로 아래 이름으로 새로 만든 폐기 가능한 로컬 클러스터에서 진행합니다. 테스트 선택 기준은 [테스트 기준](testing.md#테스트의-구분)에 있습니다.

## 준비

- Linux 컨테이너를 실행할 수 있는 Docker 엔진을 켭니다. Windows에서는 사용할 터미널에서 Docker에 접근할 수 있어야 합니다. PowerShell과 WSL 터미널을 중간에 바꾸지 않습니다.
- [kind 설치 안내](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)에 따라 kind를 준비합니다.
- kubectl을 준비합니다. [버전 호환 정책](https://kubernetes.io/releases/version-skew-policy/#kubectl)에 따라 API 서버와 한 minor 버전 이내로 맞춥니다.

이 예시의 재현 기준은 kind `v0.33.0`, Kubernetes `v1.34.11`, kubectl `v1.34.x`입니다. [kind 공식 릴리스](https://github.com/kubernetes-sigs/kind/releases/tag/v0.33.0)에 게시된 node image의 digest를 [kind.yaml](examples/kind.yaml)에 고정했습니다. drainsim이 지원하는 버전 범위와는 별개입니다.

[pdb-blocked.yaml](examples/pdb-blocked.yaml)은 단일 replica Deployment와 `minAvailable: 1`인 PDB를 만듭니다. [공식 런타임 문서에 나오는 pause 이미지](https://kubernetes.io/docs/setup/production-environment/container-runtimes/#overriding-the-sandbox-pause-image)를 Pod 퇴거와 재배치를 관찰하기 위한 최소 대상으로 사용합니다.

## 클러스터 생성

저장소 루트에서 새 터미널로 시작합니다. `kind get clusters`에 이미 `drainsim-local`이 있다면 이 절차를 진행하지 말고, 본인이 만든 이전 실습 환경인지 먼저 확인합니다. 다른 클러스터를 재사용하지 않습니다.

먼저 사용하는 셸의 명령 하나로 전용 kubeconfig 임시 파일을 만듭니다.

Bash:

```bash
drainsimKubeconfig="$(mktemp "${TMPDIR:-/tmp}/drainsim-kubeconfig.XXXXXX")"
```

PowerShell:

```powershell
$drainsimKubeconfig = [System.IO.Path]::GetTempFileName()
```

이후 kind와 kubectl 명령은 두 셸에서 동일합니다. 각 단계에서 예상하지 않은 오류가 나면 다음 단계로 넘어가지 않습니다.

```sh
kind create cluster --name drainsim-local --config docs/examples/kind.yaml --kubeconfig "$drainsimKubeconfig" --wait 180s
kubectl --kubeconfig "$drainsimKubeconfig" --context kind-drainsim-local wait --for=condition=Ready nodes --all --timeout=180s
kubectl --kubeconfig "$drainsimKubeconfig" --context kind-drainsim-local get nodes
```

`drainsim-local-control-plane`, `drainsim-local-worker`, `drainsim-local-worker2`가 모두 Ready인지 확인합니다. 전용 kubeconfig와 명시한 컨텍스트를 계속 사용합니다.

## 차단 조건 준비

worker2를 잠시 cordon해 첫 Pod를 worker1에 배치합니다. 워크로드에는 `nodeSelector`나 `nodeName`을 지정하지 않아 이후 다른 worker로 재배치할 수 있게 합니다.

```sh
kubectl --kubeconfig "$drainsimKubeconfig" --context kind-drainsim-local cordon drainsim-local-worker2
kubectl --kubeconfig "$drainsimKubeconfig" --context kind-drainsim-local apply -f docs/examples/pdb-blocked.yaml
kubectl --kubeconfig "$drainsimKubeconfig" --context kind-drainsim-local -n drainsim-lab rollout status deployment/pdb-demo --timeout=120s
kubectl --kubeconfig "$drainsimKubeconfig" --context kind-drainsim-local -n drainsim-lab get pods -l app.kubernetes.io/name=pdb-demo -o wide
```

Pod가 하나이며 `1/1` Ready 상태로 `drainsim-local-worker`에서 실행 중인지 확인하고 이름을 기록합니다. 조건이 다르면 먼저 원인을 확인합니다. 배치가 확인되면 worker2를 다시 열어 대체 Pod를 받을 수 있게 합니다.

```sh
kubectl --kubeconfig "$drainsimKubeconfig" --context kind-drainsim-local uncordon drainsim-local-worker2
kubectl --kubeconfig "$drainsimKubeconfig" --context kind-drainsim-local -n drainsim-lab get pdb pdb-demo -o yaml
```

PDB의 `metadata.generation`과 `status.observedGeneration`이 같고, `currentHealthy: 1`, `desiredHealthy: 1`, `expectedPods: 1`, `disruptionsAllowed: 0`인지 확인합니다. 아직 반영되지 않았다면 조회를 반복합니다. [PDB API 설명](https://kubernetes.io/docs/reference/kubernetes-api/policy/pod-disruption-budget-v1/)에 따라 generation이 일치해야 상태를 유효한 근거로 사용합니다.

## drain 차단 확인

```sh
kubectl --kubeconfig "$drainsimKubeconfig" --context kind-drainsim-local drain drainsim-local-worker --ignore-daemonsets --delete-emptydir-data --timeout=30s
```

종료 직후 같은 셸에서 종료 코드를 확인합니다.

Bash:

```bash
echo $?
```

PowerShell:

```powershell
$LASTEXITCODE
```

기대 결과는 PDB 위반으로 퇴거할 수 없다는 메시지와 0이 아닌 종료 코드입니다. 이어서 기존 Pod가 남아 있는지 확인합니다.

```sh
kubectl --kubeconfig "$drainsimKubeconfig" --context kind-drainsim-local -n drainsim-lab get pods -l app.kubernetes.io/name=pdb-demo -o wide
```

timeout만으로 PDB를 원인으로 보지 않고, PDB 상태, 퇴거 거절 메시지, Pod 상태를 함께 기록합니다. drain이 실패해도 노드는 cordon된 상태로 남습니다.

`--ignore-daemonsets`는 DaemonSet Pod를 제외하며, `--delete-emptydir-data`는 emptyDir 데이터 삭제를 허용합니다. 후자는 이 폐기 가능한 실습 환경에서만 사용합니다. `--disable-eviction`은 PDB 검사를 우회하므로 이 검증에 사용하지 않습니다. [kubectl drain 옵션](https://kubernetes.io/docs/reference/kubectl/generated/kubectl_drain/)

## PDB 제거 후 대조

PDB를 삭제해 차단 조건을 없앤 뒤 같은 노드를 다시 drain합니다.

```sh
kubectl --kubeconfig "$drainsimKubeconfig" --context kind-drainsim-local -n drainsim-lab delete pdb pdb-demo --wait=true
kubectl --kubeconfig "$drainsimKubeconfig" --context kind-drainsim-local drain drainsim-local-worker --ignore-daemonsets --delete-emptydir-data --timeout=120s
```

앞과 같은 방법으로 drain의 종료 코드가 0인지 확인한 뒤 대체 Pod를 확인합니다.

```sh
kubectl --kubeconfig "$drainsimKubeconfig" --context kind-drainsim-local -n drainsim-lab rollout status deployment/pdb-demo --timeout=120s
kubectl --kubeconfig "$drainsimKubeconfig" --context kind-drainsim-local -n drainsim-lab get pods -l app.kubernetes.io/name=pdb-demo -o wide
```

두 번째 drain은 성공해야 합니다. drain 성공은 대체 Pod의 Ready를 보장하지 않으므로, 이전과 다른 이름의 Pod가 `drainsim-local-worker2`에서 `1/1` Ready가 되었는지 별도로 확인합니다. 아직 준비되지 않았다면 조회를 반복하고 원인을 확인합니다. 단일 replica의 PDB를 없앤 대조군은 일시적 중단을 허용합니다.

## 정리와 재실행

정상 종료와 중간 실패 모두 아래 명령으로 이 실습 클러스터를 정리합니다. 재실행할 때도 정리 후 처음부터 시작해 같은 초기 조건을 만듭니다.

```sh
kind delete cluster --name drainsim-local --kubeconfig "$drainsimKubeconfig"
```

클러스터 삭제가 성공한 뒤 전용 임시 파일을 지웁니다.

Bash:

```bash
rm -- "$drainsimKubeconfig"
unset drainsimKubeconfig
```

PowerShell:

```powershell
Remove-Item -LiteralPath $drainsimKubeconfig
Remove-Variable drainsimKubeconfig
```

## 결과 기록

실행한 환경과 결과는 [실제 동작 비교](testing.md#실제-동작-비교)에 따라 PR 본문에 남깁니다.

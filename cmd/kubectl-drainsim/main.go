package main

import (
	"fmt"
	"os"
)

func main() {
	// 현재 CLI는 클러스터 조회, Pod 퇴거 가능 여부 분석 및 결과 출력이 구현되지 않은 상태입니다.
	// 이를 퇴거 가능하다는 성공 신호로 오해하지 않도록 메시지를 표준 오류로 쓰고 종료 코드 1을 반환합니다.
	fmt.Fprintln(os.Stderr, "kubectl-drainsim: not implemented yet")
	os.Exit(1)
}

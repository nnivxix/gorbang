package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type Proc struct {
	Name string
	Addr string
}

func main() {
	commandLsf := `lsof -nP -iTCP -sTCP:LISTEN | grep -E '127\.0\.0\.1|\[::1\]'`

	cmd := exec.Command("bash", "-c", commandLsf)

	output, err := cmd.Output()

	if err != nil {
		fmt.Println("Error outputing result", err)
		return
	}

	List(output)

}

func List(input []byte) {
	result := string(input)
	lines := strings.Split(result, "\n")

	procs := map[int]Proc{}

	for _, line := range lines {
		fields := strings.Fields(line)

		if len(fields) == 0 {
			continue
		}

		pid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		// for i, proc := range fields {
		// 	fmt.Printf("%d -> %s\n", i, proc)
		// }
		p := procs[pid]
		p.Name = string(fields[0])
		p.Addr = fields[8]
		procs[pid] = p

	}

	for pid, p := range procs {
		fmt.Printf("%d -> %s %s\n", pid, p.Name, p.Addr)
	}
}

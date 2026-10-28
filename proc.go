package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/table"
	tea "charm.land/bubbletea/v2"
)

type Proc struct {
	PID  int
	Name string
	Addr string
}

type killMode int

const (
	killGraceful killMode = iota
	killForced
)

func (m killMode) signalName() string {
	if m == killForced {
		return "KILL"
	}
	return "TERM"
}

var supervisorNames = map[string]bool{
	"php": true, "node": true, "npm": true, "npx": true,
	"yarn": true, "pnpm": true, "bun": true, "composer": true,
	"artisan": true, "nodemon": true, "concurrently": true,
	"pm2": true, "overmind": true, "foreman": true, "honcho": true,
}

// shellNames are the hard stop for the walk up the process tree.
// Without this we could climb into the terminal, Zed, or launchd
// and kill something the user did not select.
var shellNames = map[string]bool{
	"sh": true, "bash": true, "zsh": true, "dash": true,
	"fish": true, "login": true, "tmux": true, "screen": true,
}

func listProcs() []Proc {
	output, _ := loadProcs()
	result := string(output)
	lines := strings.Split(result, "\n")

	var procs []Proc

	for _, line := range lines {
		fields := strings.Fields(line)

		if len(fields) == 0 {
			continue
		}

		pid, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}

		proc := Proc{
			PID:  pid,
			Name: string(fields[0]),
			Addr: fields[8],
		}
		procs = append(procs, proc)
	}

	return procs
}

func loadProcs() ([]byte, error) {
	commandLsf := `lsof -nP -iTCP -sTCP:LISTEN | grep -E '127\.0\.0\.1|\[::1\]|\*:'`

	cmd := exec.Command("bash", "-c", commandLsf)

	output, err := cmd.Output()

	if err != nil {
		fmt.Println("Error outputing result", err)
		return nil, err
	}

	return output, nil
}

func procInfo(pid int) (int, string) {
	out, err := exec.Command("ps", "-o", "ppid=", "-o", "comm=", "-p", strconv.Itoa(pid)).Output()

	if err != nil {
		return 0, ""
	}

	line := strings.TrimSpace(string(out))
	sep := strings.IndexAny(line, " \t") // what :sep

	if sep < 0 {
		return 0, ""
	}

	ppid, err := strconv.Atoi(line[:sep]) // what :sep
	if err != nil {
		return 0, ""
	}

	return ppid, filepath.Base(strings.TrimSpace(line[sep:])) // sep: ???
}

func pidAlive(pid int) bool {
	out, err := exec.Command("ps", "-o", "pid=", "-p", strconv.Itoa(pid)).Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

// supervisorOf walks up from the listening process and returns the *nearest*
// ancestor that respawns children (e.g. `php artisan serve`).
//
// Returning the nearest match — not the highest — is deliberate: with
// `composer run dev` the chain is php -S -> artisan -> composer -> zsh, and
// killing "composer" would also take down Vite and the queue worker.
func supervisorOf(pid int) (int, string) {
	current := pid
	self := os.Getpid()

	for depth := 0; depth < 6; depth++ {
		ppid, comm := procInfo(current)

		// never kill PID 1 (launchd/systemd) and never touch ourselves.
		if ppid <= 1 || ppid == self {
			break
		}

		// Stop at the shell: above it is the terminal/IDE, not ours to kill.
		if shellNames[comm] {
			break
		}

		if supervisorNames[comm] {
			return ppid, comm
		}

		current = ppid

	}

	return 0, ""
}

// return every dencendant of root using ONE `ps` snapshot.
func procTree(root int) []int {
	out, err := exec.Command("ps", "-Ao", "pid=,ppid=").Output()

	if err != nil {
		return nil
	}
	children := make(map[int][]int)
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) != 2 { // why 2?
			continue
		}

		child, err1 := strconv.Atoi(f[0])
		parent, err2 := strconv.Atoi(f[1])
		if err1 != nil || err2 != nil {
			continue
		}
		children[parent] = append(children[parent], child)
	}
	var result []int

	queue := []int{root}
	for len(queue) > 0 {
		pid := queue[0]
		queue = queue[1:]
		for _, c := range children[pid] {
			result = append(result, c)
			queue = append(queue, c)
		}
	}
	return result
}

func procsToRows(procs []Proc) []table.Row {
	rows := make([]table.Row, 0, len(procs))
	for _, p := range procs {
		rows = append(rows, table.Row{
			strconv.Itoa(p.PID),
			p.Name,
			p.Addr,
		})
	}
	return rows
}

func killProc(pid int, supervisor int, mode killMode) tea.Cmd {
	return func() tea.Msg {
		var targets []int

		if mode == killForced {
			tree := []int{}
			// Snapshot the tree BEFORE any signal: once the parent dies its
			// children are reparented to PID 1 and become unreachable here.
			if supervisor > 0 {
				tree = procTree(supervisor)
			}

			// Kill the supervisor first so it cannot fork/respawn between
			// signals, then sweep its descendants and the listener.
			if supervisor > 0 {
				signal(supervisor, mode)
			}

			for _, p := range tree {
				signal(p, mode)
			}

			if pid != supervisor {
				signal(pid, mode)
			}

			targets = append([]int{pid}, tree...) // tree... ?
			if supervisor > 0 {
				targets = append(targets, supervisor)
			}
		} else {
			target := pid
			if supervisor > 0 {
				target = supervisor
			}
			signal(target, mode)

			targets = []int{pid}
			if supervisor > 0 {
				targets = append(targets, supervisor)
			}
		}
		// Always verify instead of trusting `kill`: the old code reloaded
		// immediately, so a dying or respawned process showed up as "still there".
		timeout := 2 * time.Second
		if mode == killForced {
			timeout = 500 * time.Millisecond //  SIGKILL is immediate; no need to wait long
		}

		return killDoneMsg{alive: !waitAllGone(targets, timeout)}
	}
}

// signal shells out to /bin/kill, matching the existing style (no syscall import).
func signal(pid int, mode killMode) {
	_ = exec.Command("kill", "-"+mode.signalName(), strconv.Itoa(pid)).Run()
}

// waitAllGone polls until every pid is gone, or the timeout elapses.
// 50ms keeps the UI responsive while avoiding a busy loop.
func waitAllGone(pids []int, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)

	for {
		alive := false
		for _, p := range pids {
			if pidAlive(p) {
				alive = true
				break
			}
		}
		if !alive {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(50 * time.Millisecond)
	}
}

package dpagent

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"
)

func (c CLI) runLogs(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(c.Err, "logs subcommand is required: path or tail")
		return 2
	}
	subcommand := strings.ToLower(strings.TrimSpace(args[0]))
	fs := flag.NewFlagSet("logs "+subcommand, flag.ContinueOnError)
	fs.SetOutput(c.Err)
	configPath := fs.String("config", "", "config path")
	lines := fs.Int("lines", 100, "number of lines to print")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	cfg, _, err := LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintln(c.Err, err)
		return 1
	}
	path := localAuditPath(cfg)
	switch subcommand {
	case "path":
		if path == "" {
			fmt.Fprintln(c.Out, "local_audit_jsonl: disabled")
			fmt.Fprintf(c.Out, "suggested: %s\n", defaultLocalAuditPath())
			return 0
		}
		fmt.Fprintln(c.Out, path)
		return 0
	case "tail":
		if path == "" {
			fmt.Fprintln(c.Err, "logging.local_audit_jsonl is not configured")
			fmt.Fprintf(c.Err, "suggested: %s\n", defaultLocalAuditPath())
			return 1
		}
		items, err := tailLocalAudit(path, *lines)
		if err != nil {
			fmt.Fprintln(c.Err, err)
			return 1
		}
		for _, item := range items {
			fmt.Fprintln(c.Out, item)
		}
		return 0
	default:
		fmt.Fprintf(c.Err, "unknown logs subcommand: %s\n", subcommand)
		return 2
	}
}

func tailLocalAudit(path string, lines int) ([]string, error) {
	if lines <= 0 {
		lines = 100
	}
	file, err := os.Open(expandPath(path))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	ring := make([]string, lines)
	count := 0
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		ring[count%lines] = scanner.Text()
		count++
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if count < lines {
		return ring[:count], nil
	}
	result := make([]string, 0, lines)
	start := count % lines
	for i := 0; i < lines; i++ {
		result = append(result, ring[(start+i)%lines])
	}
	return result, nil
}

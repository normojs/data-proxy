package dpagent

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

func (c CLI) runLogs(args []string) int {
	if len(args) == 0 || isHelpArg(args[0]) {
		c.printLogsHelp()
		if len(args) == 0 {
			return 2
		}
		return 0
	}
	subcommand := strings.ToLower(strings.TrimSpace(args[0]))
	fs := flag.NewFlagSet("logs "+subcommand, flag.ContinueOnError)
	fs.SetOutput(c.Err)
	configPath := fs.String("config", "", "config path")
	lines := fs.Int("lines", 100, "number of lines to print")
	follow := fs.Bool("follow", false, "keep printing new log lines")
	followShort := fs.Bool("f", false, "keep printing new log lines")
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
		if *follow || *followShort {
			if err := followLocalAudit(context.Background(), path, c.Out, 500*time.Millisecond); err != nil {
				fmt.Fprintln(c.Err, err)
				return 1
			}
		}
		return 0
	default:
		fmt.Fprintf(c.Err, "unknown logs subcommand: %s\n", subcommand)
		c.printLogsHelp()
		return 2
	}
}

func (c CLI) printLogsHelp() {
	fmt.Fprint(c.Out, `Usage:
  data-proxy-agent logs path [--config <path>]
  data-proxy-agent logs tail [--lines <n>] [--follow] [--config <path>]

Logs commands read the local metadata-only audit JSONL configured by logging.local_audit_jsonl.
`)
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

func followLocalAudit(ctx context.Context, path string, w io.Writer, pollInterval time.Duration) error {
	if pollInterval <= 0 {
		pollInterval = 500 * time.Millisecond
	}
	file, err := os.Open(expandPath(path))
	if err != nil {
		return err
	}
	defer file.Close()
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return err
	}
	reader := bufio.NewReader(file)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		for {
			line, err := reader.ReadString('\n')
			if len(line) > 0 {
				if _, writeErr := io.WriteString(w, line); writeErr != nil {
					return writeErr
				}
			}
			if err == nil {
				continue
			}
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			offset, err := file.Seek(0, io.SeekCurrent)
			if err != nil {
				return err
			}
			info, err := file.Stat()
			if err != nil {
				return err
			}
			if info.Size() < offset {
				if _, err := file.Seek(0, io.SeekStart); err != nil {
					return err
				}
				reader.Reset(file)
			}
		}
	}
}

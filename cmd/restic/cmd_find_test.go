package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"

	"github.com/restic/restic/internal/data"
	"github.com/restic/restic/internal/restic"
	rtest "github.com/restic/restic/internal/test"
	"github.com/restic/restic/internal/ui"
	"github.com/restic/restic/internal/ui/progress"
)

func TestPrintObjectQuotesPath(t *testing.T) {
	term := &ui.MockTerminal{}
	out := &statefulOutput{
		printer: progress.NewTerminalPrinter(false, 1, term),
	}
	out.PrintObjectNormal("file", "abcd", "dir\nfile", "", &data.Snapshot{})

	rtest.Equals(t, " ... path "+strconv.Quote("dir\nfile"), term.Output[1])
	for _, line := range term.Output {
		rtest.Assert(t, !strings.Contains(line, "\n"), "raw newline in %q", line)
	}

	term = &ui.MockTerminal{}
	out.printer = progress.NewTerminalPrinter(false, 1, term)
	out.PrintObjectNormal("blob", "abcd", "/ordinary/file", "tree", &data.Snapshot{})
	rtest.Equals(t, " ... in file /ordinary/file", term.Output[1])
}

type fakeSnapshotLoader struct{}

func (fakeSnapshotLoader) Connections() uint { return 1 }
func (fakeSnapshotLoader) LoadUnpacked(_ context.Context, _ restic.FileType, _ restic.ID) ([]byte, error) {
	return []byte("{}"), nil
}

func TestFindJSONEscapesControls(t *testing.T) {
	hostilePath := "/dir/c1\u009b2J_del\x7f_tag\U000e0067"
	sn, err := data.LoadSnapshot(t.Context(), fakeSnapshotLoader{}, restic.NewRandomID())
	rtest.OK(t, err)

	t.Run("pattern", func(t *testing.T) {
		var buf bytes.Buffer
		out := &statefulOutput{
			JSON:    true,
			newsn:   sn,
			printer: progress.NewTerminalPrinter(true, 1, &ui.MockTerminal{}),
			stdout:  &buf,
		}
		out.PrintPatternJSON(hostilePath, &data.Node{Name: "file"})
		out.Finish()

		got := buf.String()
		rtest.Assert(t, !strings.ContainsRune(got, '\u009b'), "raw C1 CSI in %q", got)
		rtest.Assert(t, !strings.ContainsRune(got, '\x7f'), "raw DEL in %q", got)
		rtest.Assert(t, !strings.ContainsRune(got, '\U000e0067'), "raw tag in %q", got)

		var parsed []struct {
			Matches []struct {
				Path string `json:"path"`
			} `json:"matches"`
		}
		rtest.OK(t, json.Unmarshal(buf.Bytes(), &parsed))
		rtest.Equals(t, 1, len(parsed))
		rtest.Equals(t, 1, len(parsed[0].Matches))
		rtest.Equals(t, hostilePath, parsed[0].Matches[0].Path)
	})

	t.Run("object", func(t *testing.T) {
		var buf bytes.Buffer
		out := &statefulOutput{
			JSON:    true,
			printer: progress.NewTerminalPrinter(true, 1, &ui.MockTerminal{}),
			stdout:  &buf,
		}
		out.PrintObjectJSON("file", "abcd", hostilePath, "tree", sn)
		out.Finish()

		got := buf.String()
		rtest.Assert(t, !strings.ContainsRune(got, '\u009b'), "raw C1 CSI in %q", got)
		rtest.Assert(t, !strings.ContainsRune(got, '\x7f'), "raw DEL in %q", got)
		rtest.Assert(t, !strings.ContainsRune(got, '\U000e0067'), "raw tag in %q", got)

		var parsed []struct {
			Path string `json:"path"`
		}
		rtest.OK(t, json.Unmarshal(buf.Bytes(), &parsed))
		rtest.Equals(t, 1, len(parsed))
		rtest.Equals(t, hostilePath, parsed[0].Path)
	})
}

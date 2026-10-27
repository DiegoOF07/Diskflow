package desktop

import "testing"

func TestExplorerCommandQuotesOnlyThePath(t *testing.T) {
	cmd, err := explorerCommand(`C:\Program Files\My App\a b.txt`)
	if err != nil {
		t.Fatal(err)
	}
	if want := `explorer.exe /select,"C:\Program Files\My App\a b.txt"`; cmd.SysProcAttr.CmdLine != want {
		t.Fatalf("CmdLine = %q, want %q", cmd.SysProcAttr.CmdLine, want)
	}
}

func TestExplorerCommandRejectsOddPaths(t *testing.T) {
	for _, p := range []string{`C:\a" & calc.exe & "`, `relative\path`, ``} {
		if _, err := explorerCommand(p); err == nil {
			t.Errorf("explorerCommand(%q) accepted it", p)
		}
	}
}

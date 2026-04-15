package manifest

import (
	"html/template"
	"os"
	"path/filepath"
	"testing"

	"github.com/olimci/shizuka/pkg/config"
	"github.com/olimci/shizuka/pkg/transforms"
)

func TestTemplateArtefactDiscardSkipsOutput(t *testing.T) {
	t.Parallel()

	out := t.TempDir()
	target := filepath.Join(out, "index.html")
	if err := os.WriteFile(target, []byte("stale"), 0o644); err != nil {
		t.Fatalf("write stale file: %v", err)
	}

	tmpl := template.Must(template.New("discard").Funcs(transforms.DefaultTemplateFuncs()).Parse(`{{ discard }}`))

	man := New()
	man.Emit(TemplateArtefact(Claim{Target: "index.html"}, tmpl, nil))

	opts := config.DefaultOptions()
	opts.OutputPath = out

	if err := man.Build(config.DefaultConfig(), opts, nil, ""); err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("expected discarded artefact to be absent, got err=%v", err)
	}
}

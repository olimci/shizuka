package extensions

import (
	"errors"
	"os/exec"

	"github.com/olimci/shizuka/pkg/config"
	ipc "github.com/olimci/stdio-ipc/go"
)

var (
	ErrNoExec = errors.New("no exec command provided")
)

func newExtension() *Extension {
	ipcr := ipc.NewRouter()

	ipc.HandleTyped[]

	return &Extension{
		IPCRouter: ipcr,
	}
}

type ExtensionMeta struct {
	Slug string
}

type Extension struct {
	IPCRouter *ipc.Router

	Meta ExtensionMeta
}

func Load(slug string, cfg *config.ConfigExtension) (*Extension, error) {
	if len(cfg.Exec) == 0 {
		return nil, ErrNoExec
	}

	cmd := exec.Command(cfg.Exec[0], cfg.Exec[1:]...)

	e := new(Extension)

	i, err := ipc.FromCmd(cmd, e.IPCRouter.Handler())
	if err != nil {
		return nil, err
	}

	i.Start()


}

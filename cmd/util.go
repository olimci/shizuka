package cmd

import (
	"log/slog"
	"time"

	"github.com/olimci/shizuka/internal/console"
	"github.com/olimci/shizuka/internal/logging"
	"github.com/urfave/cli/v3"
)

func makeLogger(con *console.Console, cmd *cli.Command) (*slog.Logger, error) {
	var level = slog.LevelInfo
	if cmd.Bool("debug") {
		level = slog.LevelDebug
	}

	format, err := logging.ParseFormat(cmd.String("format"))
	if err != nil {
		return nil, err
	}

	return slog.New(logging.NewHandler(logging.Options{
		Out: con.Out,
		Err: con.Err,

		OutTTY:        con.OutIsTerminal,
		ErrTTY:        con.ErrIsTerminal,
		OutAndErrSame: con.OutAndErrSame,

		Color: con.ColorEnabled,

		Format: format,

		ErrorOutput: logging.ErrorOutputAuto,

		Level:      level,
		AddSource:  false,
		TimeFormat: time.Kitchen,
	})), nil
}

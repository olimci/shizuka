package cmd

import "github.com/olimci/shizuka/pkg/build"

func defaultBuildSteps() []build.Step {
	return []build.Step{
		build.StepStatic(),
		build.StepContent(),
	}
}

package command

import (
	"coolaid/pkg/ai"
	"fmt"
	"strconv"

	"github.com/urfave/cli/v3"
)

func fnFlag() cli.Flag {
	return &cli.StringFlag{
		Name:        "fn",
		Usage:       "targets the given function",
		DefaultText: "file",
	}
}

func rngFlag() cli.Flag {
	return &cli.StringFlag{
		Name:        "rng",
		Usage:       "targets <start>-<end> line range",
		DefaultText: "file",
	}
}

func ragFlag() cli.Flag {
	return &cli.BoolFlag{
		Name:        "rag",
		Value:       false,
		Usage:       "use RAG for more context",
		DefaultText: "no rag",
	}
}

func modeFlag(defaultMode ai.RetrievalMode) cli.Flag {
	return &cli.StringFlag{
		Name:  "mode",
		Value: string(defaultMode),
		Usage: fmt.Sprintf("mode determines the algorithm used by RAG [%s, %s, %s]",
			ai.RetrievalFast, ai.RetrievalBalanced, ai.RetrievalDeep),
		DefaultText: string(defaultMode),
	}
}

func kFlag(defaultK int) cli.Flag {
	return &cli.IntFlag{
		Name:        "k",
		Value:       defaultK,
		Usage:       "number of top results",
		DefaultText: strconv.Itoa(defaultK),
	}
}

func mmrFlag() cli.Flag {
	return &cli.BoolFlag{
		Name:        "mmr",
		Value:       false,
		Usage:       "use Max Marginal Relevance",
		DefaultText: "false",
	}
}

func vFlag() cli.Flag {
	return &cli.BoolFlag{
		Name:        "v",
		Value:       false,
		Usage:       "set to true for verbose structured output",
		DefaultText: "false",
	}
}

func webFlag() cli.Flag {
	return &cli.IntFlag{
		Name:        "web",
		Value:       0,
		Usage:       "number of web results to retrieve",
		DefaultText: "0 -> web search disabled",
	}
}

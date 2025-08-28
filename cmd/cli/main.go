package main

import (
	"context"
	"fmt"
	"os"

	"github.com/urfave/cli/v3"

	mycli "github.com/sequix/whypending/pkg/cli"
	"github.com/sequix/whypending/pkg/constant"
	"github.com/sequix/whypending/pkg/ctrlc"
	"github.com/sequix/whypending/pkg/k8s"
)

func main() {
	cmd := &cli.Command{
		Name:  "ypd",
		Usage: "Tell you why a K8S pod is pending.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  constant.FlagKubeConfig,
				Usage: "Path to kubeconfig. Use ~/.kube/config or InClusterConfig by default",
			},
			&cli.BoolFlag{
				Name:    constant.FlagJson,
				Aliases: []string{"j"},
				Usage:   "Show json",
			},
		},
		UsageText: "[options] <namespace> <pod>",
		Before:    Init,
		Action:    mycli.Action,
	}
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}
}

func Init(ctx context.Context, argv *cli.Command) (context.Context, error) {
	stop := ctrlc.Handler()
	ctx, cancel := context.WithCancel(ctx)
	go func() {
		<-stop
		cancel()
	}()
	if err := k8s.Init(ctx, argv); err != nil {
		return nil, err
	}
	return ctx, nil
}

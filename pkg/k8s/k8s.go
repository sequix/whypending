package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/urfave/cli/v3"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/sequix/whypending/pkg/constant"
)

var (
	k8sConfig *rest.Config
	k8sClient kubernetes.Interface
)

func Init(ctx context.Context, argv *cli.Command) error {
	var (
		err    error
		config = argv.String(constant.FlagKubeConfig)
	)
	if len(config) == 0 {
		config = filepath.Join(os.Getenv("HOME"), ".kube", "config")
		if fi, err2 := os.Stat(config); err2 == nil && !fi.IsDir() {
			k8sConfig, err = clientcmd.BuildConfigFromFlags("", config)
		} else {
			config = "InClusterConfig"
			k8sConfig, err = rest.InClusterConfig()
		}
	} else {
		k8sConfig, err = clientcmd.BuildConfigFromFlags("", config)
	}
	if err != nil {
		return fmt.Errorf("failed to init k8s config %s: %w", config, err)
	}

	k8sClient, err = kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		return fmt.Errorf("failed to init k8s client: %w", err)
	}
	return nil
}

func Client() kubernetes.Interface {
	return k8sClient
}

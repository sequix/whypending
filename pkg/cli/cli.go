package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v3"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/sequix/whypending/pkg/constant"
	"github.com/sequix/whypending/pkg/k8s"
	"github.com/sequix/whypending/pkg/ypd"
)

func Action(ctx context.Context, argv *cli.Command) error {
	if argv.Args().Len() != 2 {
		cli.HelpPrinter(os.Stdout, cli.RootCommandHelpTemplate, argv.Root())
		os.Exit(1)
	}
	var (
		namespace = argv.Args().Get(0)
		podName   = argv.Args().Get(1)
		showJson  = argv.Bool(constant.FlagJson)
		k8sClient = k8s.Client().CoreV1()
	)
	podList, err := k8sClient.Pods(v1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list pods: %w", err)
	}
	nodeList, err := k8sClient.Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list nodes: %w", err)
	}
	var (
		pod   *v1.Pod
		pods  = podList.Items
		nodes = nodeList.Items
		pvs   []v1.PersistentVolume
	)
	for i := range pods {
		p := &pods[i]
		if p.Namespace == namespace && p.Name == podName {
			pod = p
			break
		}
	}
	if pod == nil {
		return fmt.Errorf("not found pod %s/%s", namespace, podName)
	}
	for _, v := range pod.Spec.Volumes {
		if v.PersistentVolumeClaim == nil {
			continue
		}
		pvcName := v.PersistentVolumeClaim.ClaimName
		pvc, err := k8sClient.PersistentVolumeClaims(pod.Namespace).Get(ctx, pvcName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get pvc %s/%s: %w", pod.Namespace, pvcName, err)
		}
		pvName := pvc.Spec.VolumeName
		if len(pvName) == 0 {
			continue
		}
		pv, err := k8sClient.PersistentVolumes().Get(ctx, pvName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get pv %s: %w", pvName, err)
		}
		pvs = append(pvs, *pv)
	}
	ans := ypd.WhyPending(pod, pods, nodes, pvs)

	if showJson {
		printJson(ans)
		return nil
	}
	printAll(ans)
	return nil
}

func printJson(ans []ypd.Detail) {
	enc := json.NewEncoder(os.Stdout)
	for _, a := range ans {
		_ = enc.Encode(a)
	}
}

func printAll(ans []ypd.Detail) {
	fmt.Println("Summary:")
	printSummary(ans)
	fmt.Println()

	fmt.Println("Resources not enough:")
	printResource(ans)
	fmt.Println()

	fmt.Println("Node affinity mismatches:")
	printNodeAffinity(ans)
	fmt.Println()

	fmt.Println("Taints not tolerated:")
	printTaint(ans)
	fmt.Println()

	fmt.Println("Pod anti-affinity mismatches:")
	printPodAntiAffinity(ans)
	fmt.Println()

	fmt.Println("Pod affinity mismatches:")
	printPodAffinity(ans)
	fmt.Println()

	fmt.Println("Pv affinity mismatches:")
	printPvAffinity(ans)
	fmt.Println()
}

func printSummary(ans []ypd.Detail) {
	for i := range ans {
		fmt.Println(ans[i].String())
	}
}

func printResource(ans []ypd.Detail) {
	var fields []string
	for _, a := range ans {
		fields = fields[:0]
		fields = append(fields, a.NodeName)
		for _, r := range a.ResourceNotEnough {
			f := fmt.Sprintf("%s(%s<%s)", r.ResourceName, r.Left.String(), r.Required.String())
			fields = append(fields, f)
		}
		if len(fields) > 1 {
			fmt.Println(strings.Join(fields, " "))
		}
	}
}

func printNodeAffinity(ans []ypd.Detail) {
	var fields []string
	for _, a := range ans {
		fields = fields[:0]
		fields = append(fields, a.NodeName)
		for _, r := range a.NodeAffinityMismatch {
			for _, t := range r.Term.MatchExpressions {
				vs := strings.Join(t.Values, ",")
				f := fmt.Sprintf("%s:%s:%s", t.Key, t.Operator, vs)
				fields = append(fields, f)
			}
		}
		if len(fields) > 1 {
			fmt.Println(strings.Join(fields, " "))
		}
	}
}

func printTaint(ans []ypd.Detail) {
	var fields []string
	for _, a := range ans {
		fields = fields[:0]
		fields = append(fields, a.NodeName)
		for _, r := range a.NodeTaintNotTolerated {
			f := fmt.Sprintf("%s=%s", r.Taint.Key, r.Taint.Value)
			fields = append(fields, f)
		}
		if len(fields) > 1 {
			fmt.Println(strings.Join(fields, " "))
		}
	}
}

func printPodAffinity(ans []ypd.Detail) {
	var fields []string
	for _, a := range ans {
		fields = fields[:0]
		fields = append(fields, a.NodeName)
		for _, r := range a.PodAffinityMismatch {
			sel, _ := metav1.LabelSelectorAsSelector(r.Term.LabelSelector)
			fields = append(fields, sel.String())
		}
		if len(fields) > 1 {
			fmt.Println(strings.Join(fields, " "))
		}
	}
}

func printPodAntiAffinity(ans []ypd.Detail) {
	var fields []string
	for _, a := range ans {
		fields = fields[:0]
		fields = append(fields, a.NodeName)
		for _, r := range a.PodAntiAffinityMismatch {
			f := fmt.Sprintf("%s/%s", r.Namespace, r.PodName)
			fields = append(fields, f)
		}
		if len(fields) > 1 {
			fmt.Println(strings.Join(fields, " "))
		}
	}
}

func printPvAffinity(ans []ypd.Detail) {
	var fields []string
	for _, a := range ans {
		fields = fields[:0]
		fields = append(fields, a.NodeName)
		pvNames := map[string]struct{}{}
		for _, r := range a.PvAffinityMismatch {
			pvNames[r.PvName] = struct{}{}
		}
		for pv := range pvNames {
			fields = append(fields, pv)
		}
		if len(fields) > 1 {
			fmt.Println(strings.Join(fields, " "))
		}
	}
}

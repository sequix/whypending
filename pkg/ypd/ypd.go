package ypd

import (
	"log"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func WhyPending(pod *v1.Pod, pods []v1.Pod, nodes []v1.Node) []Detail {
	if pod == nil {
		return nil
	}
	if len(nodes) == 0 {
		return nil
	}
	var (
		node2pods = map[string][]v1.Pod{}
		ans       []Detail
	)
	for _, p := range pods {
		if n := p.Spec.NodeName; len(n) > 0 {
			node2pods[n] = append(node2pods[n], p)
		}
	}
	for i := range nodes {
		node := &nodes[i]
		nodePods := node2pods[node.Name]
		ans = append(ans, whySingleNode(pod, nodePods, node))
	}
	return ans
}

func whySingleNode(pod *v1.Pod, nodePods []v1.Pod, node *v1.Node) Detail {
	ans := Detail{
		NodeName:                node.Name,
		ResourceNotEnough:       whyResource(pod, nodePods, node),
		NodeAffinityMismatch:    whyNodeAffinity(pod, node),
		NodeTaintNotTolerated:   whyNodeTaint(pod, node),
		PodAffinityMismatch:     whyPodAffinity(pod, nodePods, node),
		PodAntiAffinityMismatch: whyPodAntiAffinity(pod, nodePods, node),
	}
	if len(ans.ResourceNotEnough)+len(ans.NodeAffinityMismatch)+len(ans.NodeTaintNotTolerated)+
		len(ans.PodAffinityMismatch)+len(ans.PodAntiAffinityMismatch) == 0 {
		ans.Schedulable = true
	}
	return ans
}

func whyResource(pod *v1.Pod, nodePods []v1.Pod, node *v1.Node) []DetailResourceNotEnough {
	// 1. 计算 pod 资源请求
	podRequests := map[v1.ResourceName]resource.Quantity{}
	for _, c := range pod.Spec.Containers {
		for name, qty := range c.Resources.Requests {
			if q, ok := podRequests[name]; ok {
				q.Add(qty)
				podRequests[name] = q
			} else {
				podRequests[name] = qty.DeepCopy()
			}
		}
	}

	// 2. 计算 node 已分配资源
	used := map[v1.ResourceName]resource.Quantity{}
	for _, p := range nodePods {
		for _, c := range p.Spec.Containers {
			for name, qty := range c.Resources.Requests {
				if q, ok := used[name]; ok {
					q.Add(qty)
					used[name] = q
				} else {
					used[name] = qty.DeepCopy()
				}
			}
		}
	}

	// 3. 计算 node allocatable
	allocatable := node.Status.Allocatable

	// 4. 计算剩余资源
	remain := map[v1.ResourceName]resource.Quantity{}
	for name, alloc := range allocatable {
		if u, ok := used[name]; ok {
			left := alloc.DeepCopy()
			left.Sub(u)
			remain[name] = left
		} else {
			remain[name] = alloc.DeepCopy()
		}
	}

	// 5. 对比 pod 请求和剩余资源
	var notEnough []DetailResourceNotEnough
	for name, req := range podRequests {
		left, ok := remain[name]
		if !ok {
			left = resource.MustParse("0")
		}
		if left.Cmp(req) < 0 {
			notEnough = append(notEnough, DetailResourceNotEnough{
				ResourceName: string(name),
				Required:     req,
				Left:         left,
			})
		}
	}

	return notEnough
}

func whyNodeAffinity(pod *v1.Pod, node *v1.Node) []DetailNodeAffinityMismatch {
	var mismatches []DetailNodeAffinityMismatch

	// 1. 检查 nodeSelector
	if len(pod.Spec.NodeSelector) > 0 {
		term := v1.NodeSelectorTerm{
			MatchExpressions: make([]v1.NodeSelectorRequirement, 0, len(pod.Spec.NodeSelector)),
		}
		for k, v := range pod.Spec.NodeSelector {
			term.MatchExpressions = append(term.MatchExpressions, v1.NodeSelectorRequirement{
				Key:      k,
				Operator: v1.NodeSelectorOpIn,
				Values:   []string{v},
			})
		}
		if !nodeSelectorTermMatch(node, term) {
			mismatches = append(mismatches, DetailNodeAffinityMismatch{Term: term})
		}
	}

	// 2. 检查 requiredDuringSchedulingIgnoredDuringExecution
	nodeAffinity := pod.Spec.Affinity
	if nodeAffinity != nil && nodeAffinity.NodeAffinity != nil {
		selector := nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
		if selector != nil {
			for _, term := range selector.NodeSelectorTerms {
				if !nodeSelectorTermMatch(node, term) {
					mismatches = append(mismatches, DetailNodeAffinityMismatch{Term: term})
				}
			}
		}
	}

	return mismatches
}

func nodeSelectorTermMatch(node *v1.Node, term v1.NodeSelectorTerm) bool {
	fields := node.Labels
	// term 下所有 MatchExpressions 需全匹配
	for _, req := range term.MatchExpressions {
		value, exists := fields[req.Key]
		switch req.Operator {
		case v1.NodeSelectorOpIn:
			if !exists {
				return false
			}
			found := false
			for _, v := range req.Values {
				if value == v {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		case v1.NodeSelectorOpNotIn:
			if !exists {
				continue
			}
			for _, v := range req.Values {
				if value == v {
					return false
				}
			}
		case v1.NodeSelectorOpExists:
			if !exists {
				return false
			}
		case v1.NodeSelectorOpDoesNotExist:
			if exists {
				return false
			}
		case v1.NodeSelectorOpGt, v1.NodeSelectorOpLt:
			if !exists {
				return false
			}
			log.Println("NodeSelectorOpGt and NodeSelectorOpLt unimplemented")
		}
	}
	if len(term.MatchFields) > 0 {
		log.Println("MatchFields unimplemented")
	}
	return true
}

func whyNodeTaint(pod *v1.Pod, node *v1.Node) []DetailTaintNotTolerated {
	var notTolerated []DetailTaintNotTolerated
	tolerations := pod.Spec.Tolerations
	for _, taint := range node.Spec.Taints {
		// 只考虑影响调度的 taint
		if taint.Effect == v1.TaintEffectNoSchedule {
			if !toleratesTaint(tolerations, taint) {
				notTolerated = append(notTolerated, DetailTaintNotTolerated{Taint: taint})
			}
		}
	}
	return notTolerated
}

func toleratesTaint(tolerations []v1.Toleration, taint v1.Taint) bool {
	for _, tol := range tolerations {
		// 键不等一定不容忍
		if tol.Key != taint.Key {
			continue
		}
		switch tol.Operator {
		case v1.TolerationOpEqual, "":
			// Operator: "Equal" 或空，key、value 必须相等
			if tol.Value == taint.Value {
				if tol.Effect == "" || tol.Effect == taint.Effect {
					return true
				}
			}
		case v1.TolerationOpExists:
			// Operator: "Exists"，key 必须相等即可
			if tol.Effect == "" || tol.Effect == taint.Effect {
				return true
			}
		}
	}
	return false
}

func whyPodAffinity(pod *v1.Pod, nodePods []v1.Pod, node *v1.Node) []DetailPodAffinityMismatch {
	var mismatches []DetailPodAffinityMismatch
	affinity := pod.Spec.Affinity
	if affinity == nil || affinity.PodAffinity == nil {
		return nil
	}
	terms := affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution
	for _, term := range terms {
		topologyKey := term.TopologyKey
		if _, ok := node.Labels[topologyKey]; !ok {
			continue // topologyKey 不存在，跳过
		}
		matched := false
		for _, np := range nodePods {
			if podMatchesAffinityTerm(pod.Namespace, &np, &term) {
				matched = true
				break
			}
		}
		if !matched {
			mismatches = append(mismatches, DetailPodAffinityMismatch{
				Term: term,
			})
		}
	}
	return mismatches
}

func whyPodAntiAffinity(pod *v1.Pod, nodePods []v1.Pod, node *v1.Node) []DetailPodAntiAffinityMismatch {
	var mismatches []DetailPodAntiAffinityMismatch
	affinity := pod.Spec.Affinity
	if affinity == nil || affinity.PodAntiAffinity == nil {
		return mismatches
	}
	terms := affinity.PodAntiAffinity.RequiredDuringSchedulingIgnoredDuringExecution
	for _, term := range terms {
		// 需要调度到同一 node 的 pod 都不能和 term 匹配
		topologyKey := term.TopologyKey
		if _, ok := node.Labels[topologyKey]; !ok {
			continue // topologyKey 不存在，跳过
		}
		for _, np := range nodePods {
			if podMatchesAffinityTerm(pod.Namespace, &np, &term) {
				mismatches = append(mismatches, DetailPodAntiAffinityMismatch{
					Term:      term,
					Namespace: np.Namespace,
					PodName:   np.Name,
				})
			}
		}
	}
	return mismatches
}

func podMatchesAffinityTerm(matchingNamespace string, pod *v1.Pod, term *v1.PodAffinityTerm) bool {
	// 1. 匹配 namespace
	nsMatch := false
	// NamespaceSelector 或 Namespaces（K8s 任意一个命中即可）
	if len(term.Namespaces) > 0 {
		for _, ns := range term.Namespaces {
			if pod.Namespace == ns {
				nsMatch = true
				break
			}
		}
	} else if term.NamespaceSelector != nil {
		log.Printf("PodAffinityTerm.NamespaceSelector unimplmented")
		nsMatch = true
		// sel, err := metav1.LabelSelectorAsSelector(term.NamespaceSelector)
		// // 这里其实应该是 namespace 的 labels，但目前不做 namespace labels 查询，简单处理
		// if err == nil && sel.Matches(labels.Set(pod.Labels)) {
		// 	nsMatch = true
		// }
	} else {
		// 默认为本 namespace
		if pod.Namespace == matchingNamespace {
			nsMatch = true
		}
	}
	if !nsMatch {
		return false
	}

	// 2. 匹配 labelSelector
	if term.LabelSelector != nil {
		sel, err := metav1.LabelSelectorAsSelector(term.LabelSelector)
		if err != nil {
			return false
		}
		if !sel.Matches(labels.Set(pod.Labels)) {
			return false
		}
	}
	return true
}

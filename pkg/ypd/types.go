package ypd

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

type Reason string

const (
	ReasonResourceNotEnough       Reason = "ResourceNotEnough"
	ReasonNodeTaintNotTolerated   Reason = "NodeTaintNotTolerated"
	ReasonNodeAffinityMismatch    Reason = "NodeAffinityMismatch"
	ReasonPodAffinityMismatch     Reason = "PodAffinityMismatch"
	ReasonPodAntiAffinityMismatch Reason = "PodAntiAffinityMismatch"
	ReasonSchedulable             Reason = "Schedulable"

	// TODO
	ReasonPvAffinityMismatch Reason = "PvAffinityMismatch"
)

type Detail struct {
	NodeName                string                          `json:"nodeName,omitempty"`
	Schedulable             bool                            `json:"schedulable"`
	ResourceNotEnough       []DetailResourceNotEnough       `json:"resourceNotEnough,omitempty"`
	NodeTaintNotTolerated   []DetailTaintNotTolerated       `json:"nodeTaintNotTolerated,omitempty"`
	NodeAffinityMismatch    []DetailNodeAffinityMismatch    `json:"nodeAffinityMismatch,omitempty"`
	PodAffinityMismatch     []DetailPodAffinityMismatch     `json:"podAffinityMismatch,omitempty"`
	PodAntiAffinityMismatch []DetailPodAntiAffinityMismatch `json:"podAntiAffinityMismatch,omitempty"`
}

func (w *Detail) String() string {
	args := []string{w.NodeName}
	if len(w.ResourceNotEnough) > 0 {
		args = append(args, string(ReasonResourceNotEnough))
	}
	if len(w.NodeTaintNotTolerated) > 0 {
		args = append(args, string(ReasonNodeTaintNotTolerated))
	}
	if len(w.NodeAffinityMismatch) > 0 {
		args = append(args, string(ReasonNodeAffinityMismatch))
	}
	if len(w.PodAffinityMismatch) > 0 {
		args = append(args, string(ReasonPodAffinityMismatch))
	}
	if len(w.PodAntiAffinityMismatch) > 0 {
		args = append(args, string(ReasonPodAntiAffinityMismatch))
	}
	if len(args) == 1 {
		args = append(args, string(ReasonSchedulable))
	}
	return strings.Join(args, " ")
}

type DetailResourceNotEnough struct {
	ResourceName string            `json:"resourceName"`
	Required     resource.Quantity `json:"required"`
	Left         resource.Quantity `json:"left"`
}

type DetailTaintNotTolerated struct {
	Taint corev1.Taint `json:"taint"`
}

type DetailNodeAffinityMismatch struct {
	Term corev1.NodeSelectorTerm `json:"term"`
}

type DetailPodAffinityMismatch struct {
	Term corev1.PodAffinityTerm `json:"term"`
}

type DetailPodAntiAffinityMismatch struct {
	Term      corev1.PodAffinityTerm `json:"term"`
	Namespace string                 `json:"namespace"`
	PodName   string                 `json:"podName"`
}

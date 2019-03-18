package api

import (
	wgv1alpha1 "github.com/munnerz/kubewg/pkg/apis/wg/v1alpha1"
)

func PeerMatchesSelector(p *wgv1alpha1.Peer, sel wgv1alpha1.PeerSelector) bool {
	for _, n := range sel.Names {
		if p.Name == n {
			return true
		}
	}
	if len(sel.MatchLabels) == 0 {
		return false
	}
	found := true
	for k, v := range sel.MatchLabels {
		if v2, ok := p.Labels[k]; !ok || v != v2 {
			found = false
			break
		}
	}
	if found {
		return true
	}
	return false
}

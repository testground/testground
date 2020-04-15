package utils

import (
	"context"
	"sort"

	"github.com/ipfs/testground/sdk/sync"
)

func GetGroupsAndSeqs(ctx context.Context, ri *RunInfo, groupOrder int) (groupSeq, testSeq int, err error) {
	groupSeq, err = getGroupSeq(ctx, ri)
	if err != nil {
		return
	}

	if err = setGroupInfo(ctx, ri, groupOrder); err != nil {
		return
	}

	ri.RunEnv.RecordMessage("past group info")

	testSeq = getNodeID(ri, groupSeq)
	return
}

// getGroupSeq returns the sequence number of this test instance within its group
func getGroupSeq(ctx context.Context, ri *RunInfo) (int, error) {
	seq, err := ri.Client.SignalAndWait(ctx, sync.State(ri.RunEnv.TestGroupID), ri.RunEnv.TestGroupInstanceCount)
	seq-- // make 0-indexed
	return int(seq), err
}

// setGroupInfo uses the sync service to determine which groups are part of the test and to get their sizes.
// This information is set on the passed in RunInfo.
func setGroupInfo(ctx context.Context, ri *RunInfo, groupOrder int) error {
	gi := &GroupInfo{
		ID:     ri.RunEnv.TestGroupID,
		Size:   ri.RunEnv.TestGroupInstanceCount,
		Order:  groupOrder,
		Params: ri.RunEnv.TestInstanceParams,
	}

	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	groupInfoCh := make(chan *GroupInfo)
	ri.Client.MustPublishSubscribe(subCtx, GroupIDTopic, gi, groupInfoCh)

	groupOrderMap := make(map[int][]string)
	groups := make(map[string]*GroupInfo)
	for i := 0; i < ri.RunEnv.TestInstanceCount; i++ {
		select {
		case g, more := <-groupInfoCh:
			if !more {
				break
			}
			if _, ok := groups[g.ID]; !ok {
				groups[g.ID] = g
				groupOrderMap[g.Order] = append(groupOrderMap[g.Order], g.ID)
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	ri.RunEnv.RecordMessage("there are %d groups %v", len(groups), groups)

	sortedGroups := make([]string, 0, len(groups))
	sortedOrderNums := make([]int, 0, len(groupOrderMap))
	for order := range groupOrderMap {
		sortedOrderNums = append(sortedOrderNums, order)
	}
	sort.Ints(sortedOrderNums)

	for i := 0; i < len(sortedOrderNums); i++ {
		sort.Strings(groupOrderMap[i])
		sortedGroups = append(sortedGroups, groupOrderMap[i]...)
	}

	ri.Groups = sortedGroups
	ri.GroupProperties = groups

	ri.RunEnv.RecordMessage("sortedGroup order %v", sortedGroups)

	return nil
}

// getNodeID returns the sequence number of this test instance within the test
func getNodeID(ri *RunInfo, seq int) int {
	id := seq
	for _, g := range ri.Groups {
		if g == ri.RunEnv.TestGroupID {
			break
		}
		id += ri.GroupProperties[g].Size
	}

	return id
}

// GroupIDTopic represents a subtree under the test run's sync tree where peers
// participating in this distributed test advertise their groups.
var GroupIDTopic = sync.NewTopic("groupIDs", &GroupInfo{})

type GroupInfo struct {
	ID     string
	Size   int
	Order  int
	Params map[string]string
}

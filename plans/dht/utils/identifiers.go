package utils

import (
	"context"
	"reflect"
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
	// Set a state barrier.
	seqNumCh := ri.Watcher.Barrier(ctx, sync.State(ri.RunEnv.TestGroupID), int64(ri.RunEnv.TestGroupInstanceCount))

	// Signal we're in the same state.
	seq, err := ri.Writer.SignalEntry(ctx, sync.State(ri.RunEnv.TestGroupID))
	if err != nil {
		return 0, err
	}

	// make sequence number 0 indexed
	seq--

	// Wait until all others have signalled.
	if err := <-seqNumCh; err != nil {
		return 0, err
	}

	return int(seq), nil
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

	if _, err := ri.Writer.Write(ctx, GroupIDSubtree, gi); err != nil {
		return err
	}

	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	groupInfoCh := make(chan *GroupInfo)
	if err := ri.Watcher.Subscribe(subCtx, GroupIDSubtree, groupInfoCh); err != nil {
		return err
	}

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

// GroupIDSubtree represents a subtree under the test run's sync tree where peers
// participating in this distributed test advertise their groups.
var GroupIDSubtree = &sync.Subtree{
	GroupKey:    "groupIDs",
	PayloadType: reflect.TypeOf(&GroupInfo{}),
	KeyFunc: func(val interface{}) string {
		return val.(*GroupInfo).ID
	},
}

type GroupInfo struct {
	ID     string
	Size   int
	Order  int
	Params map[string]string
}

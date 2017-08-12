package topologyTransmogrifier

import (
	"fmt"
)

// installTargetNew
// Now that everyone in the old/current topology knows about the Next
// topology, we need to do a further txn to ensure everyone new who's
// joining the cluster gets told.

type installTargetNew struct {
	*targetConfigBase
}

func (task *installTargetNew) Tick() (bool, error) {
	next := task.activeTopology.NextConfiguration
	if !(next != nil && next.Version == task.targetConfig.Version && !next.InstalledOnNew) {
		return task.completed()
	}

	localHost, err := task.firstLocalHost(task.activeTopology.Configuration)
	if err != nil {
		return task.fatal(err)
	}

	remoteHosts := task.allHostsBarLocalHost(localHost, next)
	task.installTopology(task.activeTopology, nil, localHost, remoteHosts)
	task.ensureShareGoalWithAll()

	if !task.isInRMs(next.NewRMIds) {
		task.inner.Logger.Log("msg", "Awaiting new cluster members.")
		// this step must be performed by the new RMs
		return false, nil
	}

	// From this point onwards, we have the possibility that some
	// node-to-be-removed has rushed ahead and has shutdown. So we
	// can't rely on any to-be-removed node. So that means we can only
	// rely on the nodes in next.RMs, which means we need a majority of
	// them to be alive; and we use the removed RMs as extra passives.
	active, passive := task.formActivePassive(next.RMs, next.LostRMIds)
	if active == nil {
		return false, nil
	}

	task.inner.Logger.Log("msg", "Installing on new cluster members.",
		"active", fmt.Sprint(active), "passive", fmt.Sprint(passive))

	topology := task.activeTopology.Clone()
	topology.NextConfiguration.InstalledOnNew = true

	twoFInc := uint16(next.RMs.NonEmptyLen())
	txn := task.createTopologyTransaction(task.activeTopology, topology, twoFInc, active, passive)
	go task.runTopologyTransaction(task, txn, active, passive)
	return false, nil
}

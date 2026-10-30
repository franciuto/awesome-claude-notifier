// Package winproc looks up Windows processes; Ancestors is pure so it runs everywhere.
package winproc

// Proc is one row of the process table.
type Proc struct {
	Parent uint32
	Exe    string
}

// Ancestors returns the parent, grandparent, ... of pid (at most 32), stopping at a loop.
func Ancestors(pid uint32, procs map[uint32]Proc) []uint32 {
	var chain []uint32
	for len(chain) < 32 {
		p, ok := procs[pid]
		if !ok || p.Parent == 0 {
			break
		}
		pid = p.Parent
		for _, seen := range chain {
			if seen == pid {
				return chain
			}
		}
		chain = append(chain, pid)
	}
	return chain
}

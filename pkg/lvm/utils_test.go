package lvm

import (
	"testing"
)

func TestListLVMVolumeGroup(t *testing.T) {
	vgs, err := ListLVMVolumeGroup(false)
	if err != nil {
		t.Fatalf("Failed to list LVM volume group: %v", err)
	}
	if len(vgs) == 0 {
		t.Fatal("Failed to list LVM volume group: vg not found")
	}
	for i := range vgs {
		t.Logf("Found LVM volume group: %#+v", vgs[i])
	}
}

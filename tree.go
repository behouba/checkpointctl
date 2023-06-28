package main

import (
	"fmt"
	"path/filepath"

	metadata "github.com/checkpoint-restore/checkpointctl/lib"
	"github.com/checkpoint-restore/go-criu/v6/crit"
	"github.com/checkpoint-restore/go-criu/v6/crit/images"
	spec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/xlab/treeprint"
)

func renderTreeView(tasks []task) error {
	for _, task := range tasks {
		containerConfig, _, err := metadata.ReadContainerCheckpointConfigDump(task.outputDir)
		if err != nil {
			return err
		}

		specDump, _, err := metadata.ReadContainerCheckpointSpecDump(task.outputDir)
		if err != nil {
			return err
		}

		ci, err := getContainerInfo(task.outputDir, specDump, containerConfig)
		if err != nil {
			return err
		}

		archiveSizes, err := getArchiveSizes(task.checkpointFilePath)
		if err != nil {
			return err
		}

		tree := buildTree(ci, containerConfig, archiveSizes)

		if mounts {
			addMountsToTree(tree, specDump)
		}

		if stats {
			dumpStats, err := crit.GetDumpStats(task.outputDir)
			if err != nil {
				return fmt.Errorf("failed to get dump statistics: %w", err)
			}

			addDumpStatsToTree(tree, dumpStats)
		}

		if psTree {
			c := crit.New("", "", filepath.Join(task.outputDir, "checkpoint"), false, false)
			psTree, err := c.ExplorePs()
			if err != nil {
				return fmt.Errorf("failed to get process tree: %w", err)
			}

			addPsTreeToTree(tree, psTree)
		}

		fmt.Printf("\nDisplaying container checkpoint tree view from %s\n\n", task.checkpointFilePath)
		fmt.Println(tree.String())
	}

	return nil
}

func buildTree(ci *containerInfo, containerConfig *metadata.ContainerConfig, archiveSizes *archiveSizes) treeprint.Tree {
	if ci.Name == "" {
		ci.Name = "Container"
	}
	tree := treeprint.NewWithRoot(ci.Name)

	tree.AddBranch(fmt.Sprintf("Image: %s", containerConfig.RootfsImageName))
	tree.AddBranch(fmt.Sprintf("ID: %s", containerConfig.ID))
	tree.AddBranch(fmt.Sprintf("Runtime: %s", containerConfig.OCIRuntime))
	tree.AddBranch(fmt.Sprintf("Created: %s", ci.Created))
	tree.AddBranch(fmt.Sprintf("Engine: %s", ci.Engine))

	if ci.IP != "" {
		tree.AddBranch(fmt.Sprintf("IP: %s", ci.IP))
	}
	if ci.MAC != "" {
		tree.AddBranch(fmt.Sprintf("MAC: %s", ci.MAC))
	}

	tree.AddBranch(fmt.Sprintf("Checkpoint Size: %s", metadata.ByteToString(archiveSizes.checkpointSize)))

	if archiveSizes.rootFsDiffTarSize != 0 {
		tree.AddBranch(fmt.Sprintf("Root Fs Diff Size: %s", metadata.ByteToString(archiveSizes.rootFsDiffTarSize)))
	}

	return tree
}

func addMountsToTree(tree treeprint.Tree, specDump *spec.Spec) {
	mountsTree := tree.AddBranch("Overview of Mounts")
	for _, data := range specDump.Mounts {
		mountTree := mountsTree.AddBranch(fmt.Sprintf("Destination: %s", data.Destination))
		mountTree.AddBranch(fmt.Sprintf("Type: %s", data.Type))
		mountTree.AddBranch(fmt.Sprintf("Source: %s", func() string {
			return data.Source
		}()))
	}
}

func addDumpStatsToTree(tree treeprint.Tree, dumpStats *images.DumpStatsEntry) {
	statsTree := tree.AddBranch("CRIU dump statistics")
	statsTree.AddBranch(fmt.Sprintf("Freezing Time: %d us", dumpStats.GetFreezingTime()))
	statsTree.AddBranch(fmt.Sprintf("Frozen Time: %d us", dumpStats.GetFrozenTime()))
	statsTree.AddBranch(fmt.Sprintf("Memdump Time: %d us", dumpStats.GetMemdumpTime()))
	statsTree.AddBranch(fmt.Sprintf("Memwrite Time: %d us", dumpStats.GetMemwriteTime()))
	statsTree.AddBranch(fmt.Sprintf("Pages Scanned: %d us", dumpStats.GetPagesScanned()))
	statsTree.AddBranch(fmt.Sprintf("Pages Written: %d us", dumpStats.GetPagesWritten()))
}

func addPsTreeToTree(tree treeprint.Tree, psTree *crit.PsTree) {
	// processNodes is a recursive function to create
	// a new branch for each process and add its child
	// processes as child nodes of the branch.
	var processNodes func(treeprint.Tree, *crit.PsTree)
	processNodes = func(tree treeprint.Tree, root *crit.PsTree) {
		node := tree.AddMetaBranch(root.PId, root.Comm)
		for _, child := range root.Children {
			processNodes(node, child)
		}
	}
	psTreeNode := tree.AddBranch("Process tree")
	processNodes(psTreeNode, psTree)
}

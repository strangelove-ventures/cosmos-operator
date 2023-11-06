/*
Copyright © 2023 Strangelove Crypto, Inc.
*/
package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"cosmossdk.io/log"
	"cosmossdk.io/store/rootmulti"
	dbm "github.com/cosmos/cosmos-db"
	"github.com/spf13/cobra"
	cosmosv1 "github.com/strangelove-ventures/cosmos-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	namespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

	flagBackend = "backend"
	flagDaemon  = "daemon"

	tickTime = 30 * time.Second
)

// VersionCheckCmd gets the height of this node and updates the status of the crd.
// It panics if the wrong image is specified for the pod for the height,
// restarting the pod so that the correct image is used from the patched height.
// this command is intended to be run as an init container.
func VersionCheckCmd(scheme *runtime.Scheme) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "versioncheck",
		Short: "Confirm correct image used for current node height",
		Long:  `Open the Cosmos SDK chain database, get the height, update the crd status with the height, then check the image for the height and panic if it is incorrect.`,
		Run: func(cmd *cobra.Command, args []string) {
			dataDir := os.Getenv("DATA_DIR")
			backend, _ := cmd.Flags().GetString(flagBackend)
			daemon, _ := cmd.Flags().GetBool(flagDaemon)

			nsbz, err := os.ReadFile(namespaceFile)
			if err != nil {
				panic(fmt.Errorf("failed to read namespace from service account: %w", err))
			}
			ns := string(nsbz)

			config, err := rest.InClusterConfig()
			if err != nil {
				panic(fmt.Errorf("failed to get in cluster config: %w", err))
			}

			clientset, err := kubernetes.NewForConfig(config)
			if err != nil {
				panic(fmt.Errorf("failed to create kube clientset: %w", err))
			}

			ctx := cmd.Context()

			thisPod, err := clientset.CoreV1().Pods(ns).Get(ctx, os.Getenv("HOSTNAME"), metav1.GetOptions{})
			if err != nil {
				panic(fmt.Errorf("failed to get this pod: %w", err))
			}

			cosmosFullNodeName := thisPod.Labels["app.kubernetes.io/name"]

			kClient, err := client.New(config, client.Options{
				Scheme: scheme,
			})
			if err != nil {
				panic(fmt.Errorf("failed to create kube client: %w", err))
			}

			namespacedName := types.NamespacedName{
				Namespace: ns,
				Name:      cosmosFullNodeName,
			}

			crd := new(cosmosv1.CosmosFullNode)
			if err = kClient.Get(ctx, namespacedName, crd); err != nil {
				panic(fmt.Errorf("failed to get crd: %w", err))
			}

			if len(crd.Spec.ChainSpec.Versions) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No versions specified, skipping version check")
				return
			}

			s, err := os.Stat(dataDir)
			if err != nil {
				panic(fmt.Errorf("failed to stat %s: %w", dataDir, err))
			}

			if !s.IsDir() {
				panic(fmt.Errorf("%s is not a directory", dataDir))
			}

			if daemon {
				ticker := time.NewTicker(tickTime)
				defer ticker.Stop()
				for {
					select {
					case <-cmd.Context().Done():
						return
					case <-ticker.C:
						if err := checkVersion(cmd.Context(), nil, kClient, namespacedName, thisPod, dataDir, backend, cmd.OutOrStdout()); err != nil {
							panic(err)
						}
						ticker.Reset(tickTime)
					}
				}
			}
			if err := checkVersion(cmd.Context(), crd, kClient, namespacedName, thisPod, dataDir, backend, cmd.OutOrStdout()); err != nil {
				panic(err)
			}
		},
	}

	cmd.Flags().StringP(flagBackend, "b", "goleveldb", "Database backend")
	cmd.Flags().BoolP(flagDaemon, "d", false, "Run as daemon")

	return cmd
}

func checkVersion(
	ctx context.Context,
	crd *cosmosv1.CosmosFullNode,
	kClient client.Client,
	namespacedName types.NamespacedName,
	thisPod *corev1.Pod,
	dataDir string,
	backend string,
	writer io.Writer,
) error {
	db, err := dbm.NewDB("application", getBackend(backend), dataDir)
	if err != nil {
		if crd == nil {
			fmt.Fprintf(writer, "Failed to open db: %s. The node is likely running.\n", err)
			// This is okay, we will read it later if the node shuts down.
			return nil
		} else {
			return fmt.Errorf("failed to open db: %w", err)
		}
	}
	store := rootmulti.NewStore(db, log.NewNopLogger(), nil)

	height := store.LatestVersion() + 1
	db.Close()

	if crd == nil {
		crd = new(cosmosv1.CosmosFullNode)
		if err := kClient.Get(ctx, namespacedName, crd); err != nil {
			return fmt.Errorf("failed to get crd: %w", err)
		}
	}

	if err := patchStatusHeightIfNecessary(ctx, kClient, crd, thisPod.Name, uint64(height)); err != nil {
		return err
	}

	var image string
	for _, v := range crd.Spec.ChainSpec.Versions {
		if uint64(height) < v.UpgradeHeight {
			break
		}
		image = v.Image
	}

	thisPodImage := thisPod.Spec.Containers[0].Image
	if thisPodImage != image {
		return fmt.Errorf("image mismatch for height %d: %s != %s", height, thisPodImage, image)
	}

	fmt.Fprintf(writer, "Verified correct image for height %d: %s\n", height, image)

	return nil
}

func patchStatusHeightIfNecessary(
	ctx context.Context,
	kClient client.Client,
	crd *cosmosv1.CosmosFullNode,
	instanceName string,
	height uint64,
) error {
	if crd.Status.Height != nil {
		if h, ok := crd.Status.Height[instanceName]; ok && h == height {
			// Status is up to date already.
			return nil
		}
	}

	patch := crd.DeepCopy()
	if patch.Status.Height == nil {
		patch.Status.Height = make(map[string]uint64)
	}
	patch.Status.Height[instanceName] = height

	if err := kClient.Status().Patch(
		ctx, patch, client.MergeFrom(crd.DeepCopy()),
	); err != nil {
		return fmt.Errorf("failed to patch status: %w", err)
	}

	return nil
}

func getBackend(backend string) dbm.BackendType {
	switch backend {
	case "goleveldb":
		return dbm.GoLevelDBBackend
	case "memdb":
		return dbm.MemDBBackend
	case "rocksdb":
		return dbm.RocksDBBackend
	case "pebbledb":
		return dbm.PebbleDBBackend
	default:
		panic(fmt.Errorf("unknown backend %s", backend))
	}
}

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: kv-cluster <serve|check>")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "  serve  - Start the 3-node cluster and serve HTTP")
		fmt.Fprintln(os.Stderr, "  check  - Run linearizability check and exit")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "serve":
		runServe()
	case "check":
		os.Exit(runCheck())
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func runServe() {
	cluster := NewCluster()
	cluster.Start()
	defer cluster.Stop()

	fmt.Println("Cluster started on ports 9100-9102")
	fmt.Println("Press Ctrl+C to stop")

	select {} // block forever
}

func runCheck() int {
	cluster := NewCluster()
	cluster.Start()
	defer cluster.Stop()

	// 1. Wait for leader election
	fmt.Println("[1/6] Waiting for leader election...")
	leaderID, err := cluster.WaitForLeader(5 * time.Second)
	if err != nil {
		fmt.Printf("FAIL: %v\n", err)
		return 1
	}
	fmt.Printf("      Leader elected: node %d\n", leaderID)

	leaderPort := cluster.Port(leaderID)

	// Pick two followers
	followers := []int{}
	for i := 0; i < NumNodes; i++ {
		if i != leaderID {
			followers = append(followers, i)
		}
	}
	isolatedID := followers[0]   // this follower will be partitioned
	remainingID := followers[1]  // this one stays connected

	fmt.Printf("      Follower to isolate: node %d\n", isolatedID)
	fmt.Printf("      Remaining follower: node %d\n", remainingID)

	// 2. Let the cluster stabilize
	fmt.Println("[2/6] Stabilizing cluster...")
	time.Sleep(1 * time.Second)

	// 3. Partition the isolated follower from everyone
	fmt.Printf("[3/6] Partitioning node %d from the cluster...\n", isolatedID)
	cluster.Transport().Partition(isolatedID, leaderID)
	cluster.Transport().Partition(isolatedID, remainingID)

	// Let the partition take effect
	time.Sleep(200 * time.Millisecond)

	// 4. Write K=1 to the leader (will commit with leader + remaining follower = majority)
	fmt.Println("[4/6] Writing K=1 via leader (while node is partitioned)...")
	if !writeKey(leaderPort, "K", "1") {
		fmt.Println("FAIL: Write to leader failed")
		return 1
	}
	fmt.Println("      Write acknowledged by leader (committed with majority)")
	fmt.Printf("      Leader commit index: %d\n", cluster.Node(leaderID).CommitIndex())

	// 5. Heal partition
	fmt.Printf("[5/6] Healing partition (node %d rejoins)...\n", isolatedID)
	cluster.Transport().Heal()

	// Brief pause to allow the transport layer to re-establish connectivity
	time.Sleep(100 * time.Millisecond)

	// 6. Read K from the formerly-isolated follower
	isolatedPort := cluster.Port(isolatedID)
	fmt.Printf("[6/6] Reading K from formerly-isolated node %d...\n", isolatedID)
	value, found := readKey(isolatedPort, "K")

	fmt.Printf("      Follower applied index: %d\n", cluster.Node(isolatedID).AppliedIndex())
	fmt.Printf("      Follower commit index:  %d\n", cluster.Node(isolatedID).CommitIndex())
	fmt.Printf("      Leader commit index:    %d\n", cluster.Node(leaderID).CommitIndex())

	if !found {
		fmt.Printf("FAIL: Key K not found on node %d (stale read — linearizability violated)\n", isolatedID)
		fmt.Println("      A committed write is invisible to a reader. The system claims to")
		fmt.Println("      be linearizable, but reads are served from stale local state without")
		fmt.Println("      confirming the applied index is current with the leader's commit index.")
		return 1
	}
	if value != "1" {
		fmt.Printf("FAIL: Expected K=1, got K=%s on node %d (stale read)\n", value, isolatedID)
		return 1
	}
	fmt.Printf("      Read K=%s from node %d\n", value, isolatedID)
	fmt.Println("PASS: Linearizable read confirmed — value is consistent after partition heal")
	return 0
}

func writeKey(port int, key, value string) bool {
	client := &http.Client{
		Timeout: 5 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	body := fmt.Sprintf(`{"key":"%s","value":"%s"}`, key, value)
	resp, err := client.Post(
		fmt.Sprintf("http://127.0.0.1:%d/write", port),
		"application/json",
		strings.NewReader(body),
	)
	if err != nil {
		fmt.Printf("      Write error: %v\n", err)
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

func readKey(port int, key string) (string, bool) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/read?key=%s", port, key))
	if err != nil {
		fmt.Printf("      Read error: %v\n", err)
		return "", false
	}
	defer resp.Body.Close()

	var result ReadResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", false
	}
	return result.Value, result.Found
}

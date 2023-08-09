package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"time"
)

func runCommand(dir string, env []string, name string, arg ...string) string {
	// Combine all arguments into a single string for printing
	argString := strings.Join(arg, " ")
	fmt.Printf("%s %s\n", name, argString)

	cmd := exec.Command(name, arg...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		fmt.Println(err)
		fmt.Println("Error:", stderr.String())
	}
	return out.String()
}

func transferCommit(repoPath string, workBranch string, stagingBranch string) {

	// Fetch the latest changes
	runCommand(repoPath, nil, "git", "fetch")

	runCommand(repoPath, nil, "git", "checkout", workBranch)

	// Get the commit hash of the oldest commit on the workBranch branch that isn't on the staging branch
	commitHashesStr := runCommand(repoPath, nil, "git", "log", "--pretty=format:%H", stagingBranch+".."+workBranch)
	commitHashesArr := strings.Split(commitHashesStr, "\n")
	if len(commitHashesArr) == 0 || commitHashesArr[0] == "" {
		fmt.Println("No new commits to transfer.")
		return
	}

	// Reverse the array so that we start with the oldest commit.
	for i, j := 0, len(commitHashesArr)-1; i < j; i, j = i+1, j-1 {
		commitHashesArr[i], commitHashesArr[j] = commitHashesArr[j], commitHashesArr[i]
	}

	// Read transferred commit SHAs from file
	transferredCommits, err := ioutil.ReadFile(stagingBranch + "_transferred_commits.txt")
	var transferredCommitsStr string
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// If file does not exist, create it
			_, err := os.Create(stagingBranch + "_transferred_commits.txt")
			if err != nil {
				log.Fatal(err)
			}
			transferredCommitsStr = "" // File is empty since we just created it
		} else {
			// If it's an error other than the file not existing, it's still a fatal error.
			log.Fatal(err)
		}
	} else {
		transferredCommitsStr = string(transferredCommits)
	}

	for _, commitHash := range commitHashesArr {
		// Skip commit if it has already been transferred
		if strings.Contains(transferredCommitsStr, commitHash) {
			fmt.Println("Skipping previously transferred commit: " + commitHash)
			continue
		}
		// change to this format instead of git checkout
		// git fetch . <commit-hash>:<target-branch>

		// Checkout staging
		runCommand(repoPath, nil, "git", "checkout", stagingBranch)

		// Cherry-pick commit
		result := runCommand(repoPath, nil, "git", "cherry-pick", "-Xtheirs", commitHash)
		if strings.Contains(result, "error") {
			fmt.Println("Cherry-pick failed, please resolve conflicts manually.")
			os.Exit(1)
		}

		// Amend the commit date to current date
		currentDate := time.Now().Format(time.RFC3339)
		runCommand(repoPath, nil, "git", "commit", "--amend", "--no-edit", "--date", currentDate)

		// Push to remote
		runCommand(repoPath, nil, "git", "push", "--set-upstream", "origin", stagingBranch, ":", stagingBranch)

		// Change back to work branch
		runCommand(repoPath, nil, "git", "checkout", workBranch)

		fmt.Println("Transferred commit: " + commitHash)

		// Write transferred commit SHA to file
		f, err := os.OpenFile(stagingBranch+"_transferred_commits.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		if _, err := f.WriteString(commitHash + "\n"); err != nil {
			log.Fatal(err)
		}

		break
	}
}

func main() {

	// Define the flags for the command line arguments
	repoPathPtr := flag.String("repo", ".", "Path to the git repository")
	workBranchPtr := flag.String("work-branch", "feature", "Name of the working branch")
	stagingBranchPtr := flag.String("staging-branch", "staging", "Name of the staging branch")
	minDelayPtr := flag.Int("min-delay", 45, "Minimum delay between commits in minutes")
	maxDelayPtr := flag.Int("max-delay", 80, "Maximum delay between commits in minutes")

	// Parse the command line arguments
	flag.Parse()

	rand.Seed(time.Now().UnixNano())

	for {
		// Call the transferCommit function with the command line arguments
		transferCommit(*repoPathPtr, *workBranchPtr, *stagingBranchPtr)

		// Random sleep interval between 45 minutes to 80 minutes
		sleepDuration := time.Minute * time.Duration(rand.Intn(*maxDelayPtr-*minDelayPtr)+*minDelayPtr)
		fmt.Printf("Sleeping for %v\n", sleepDuration)
		time.Sleep(sleepDuration)
	}
}

//  to test:
// echo "new line" >> all-night-test.txt && git add all-night-test.txt && git commit -m "some commit message"

//  to run:
// go run . --repo /path/to/repo --work-branch feature --staging-branch staging --min-delay 1 --max-delay 2

// git branch --set-upstream-to=origin/<branch-name> <branch-name>

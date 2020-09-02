package main

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/GongT/cancel-previous-workflows/internal/github"
)

var branchName = strings.Replace(os.Getenv("GITHUB_REF"), "refs/heads/", "", 1)
var currentSha = os.Getenv("GITHUB_SHA")
var currentRunNumber, _ = strconv.Atoi(os.Getenv("GITHUB_RUN_NUMBER"))
var isCancelAll = len(os.Getenv("NO_FILTER")) > 0

func main() {
	var runsList []*github.WorkflowRun
	if queued, err := github.ListRuns(github.StateTypeQueue, branchName); err == nil {
		runsList = append(runsList, queued...)
	} else {
		log.Printf("error get action runs: %v\n", err)
		return
	}
	if inProgress, err := github.ListRuns(github.StateTypeInProgress, branchName); err == nil {
		runsList = append(runsList, inProgress...)
	} else {
		log.Printf("error get action runs: %v\n", err)
		return
	}

	runsListDedup := make([]*github.WorkflowRun, 0, len(runsList))
	skip := make([]int64, 0, len(runsList))
	for _, r := range runsList {
		if !github.IsWorkspaceIn(runsListDedup, r) {
			runsListDedup = append(runsListDedup, r)
		} else {
			skip = append(skip, r.Id)
		}
	}
	if (len(skip)) > 0 {
		log.Printf("skip ids: %v", skip)
	}

	log.Printf("  * found %v runs", len(runsListDedup))

	var shouldCancel []*github.WorkflowRun
	if isCancelAll {
		for _, run := range runsListDedup {
			if run.RunNumber == currentRunNumber {
				continue // skip my self anyway
			}
			shouldCancel = append(shouldCancel, run)
		}
	} else {
		log.Printf("finding workflow names in repo %s\n", github.GetCurrentRepo())
		workflowId, err := github.GetWorkflowId()
		if err != nil {
			log.Printf("error find workflow id: %v\n", err)
			return
		}

		for _, run := range runsListDedup {
			if run.HeadBranch != branchName {
				continue // should not happen cuz we pre-filter, but better safe than sorry
			}
			if run.HeadSha == currentSha {
				continue // not canceling my own jobs
			}
			if currentRunNumber != 0 && run.RunNumber > currentRunNumber {
				continue // only canceling previous executions, not newer ones
			}
			if run.WorkflowId != workflowId {
				log.Printf(" ! found run %v, number %v, workflow = %v | want = %v", run.Id, run.RunNumber, run.WorkflowId, workflowId)
				continue
			}

			shouldCancel = append(shouldCancel, run)
		}
	}

	log.Printf("          %v should cancel", len(shouldCancel))

	count := len(shouldCancel)

	s := strconv.Itoa(len(strconv.Itoa(count)))

	var okCnt, errCnt int

	for index, run := range shouldCancel {
		id := run.Id
		err := github.CancelWorkflow(id)

		if err == nil {
			okCnt++
			log.Printf("  [%"+s+"d/%"+s+"d] done [%v]: %v\n", index+1, count, id)
		} else {
			errCnt++
			log.Printf("  [%"+s+"d/%"+s+"d] error [%v]: %v\n", index+1, count, id, err)
		}
	}
	log.Printf("All done, %v success, %v error.\n", okCnt, errCnt)
}

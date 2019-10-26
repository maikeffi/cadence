// Copyright (c) 2019 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package ndc

import (
	"github.com/uber/cadence/common/persistence"
	"math"
	"time"

	"github.com/pborman/uuid"
	"github.com/uber/cadence/.gen/go/cadence/workflowservicetest"
	"github.com/uber/cadence/.gen/go/shared"
	"github.com/uber/cadence/common"
	test "github.com/uber/cadence/common/testing"
)

func (s *nDCIntegrationTestSuite) TestReplicationMessageApplication() {

	workflowID := "replication-message-test" + uuid.New()
	runID := uuid.New()
	workflowType := "event-generator-workflow-type"
	tasklist := "event-generator-taskList"

	// active has initial version 0
	historyClient := s.active.GetHistoryClient()

	var historyBatch []*shared.History
	s.generator = test.InitializeHistoryEventGenerator(s.domainName, 1)

	for s.generator.HasNextVertex() {
		events := s.generator.GetNextVertices()
		historyEvents := &shared.History{}
		for _, event := range events {
			historyEvents.Events = append(historyEvents.Events, event.GetData().(*shared.HistoryEvent))
		}
		historyBatch = append(historyBatch, historyEvents)
	}

	versionHistory := s.eventBatchesToVersionHistory(nil, historyBatch)
	standbyClient := s.mockFrontendClient["standby"].(*workflowservicetest.MockClient)

	s.applyEventsThroughFetcher(
		workflowID,
		runID,
		workflowType,
		tasklist,
		versionHistory,
		historyBatch,
		historyClient,
		standbyClient,
	)

	time.Sleep(10 * time.Second)

	s.verifyEventHistory(workflowID, runID, historyBatch)
}

func (s *nDCIntegrationTestSuite) TestReplicationMessageDLQ() {

	workflowID := "replication-message-dlq-test" + uuid.New()
	runID := uuid.New()
	workflowType := "event-generator-workflow-type"
	tasklist := "event-generator-taskList"

	// active has initial version 0
	historyClient := s.active.GetHistoryClient()

	var historyBatch []*shared.History
	s.generator = test.InitializeHistoryEventGenerator(s.domainName, 1)

	for s.generator.HasNextVertex() {
		events := s.generator.GetNextVertices()
		historyEvents := &shared.History{}
		for _, event := range events {
			historyEvents.Events = append(historyEvents.Events, event.GetData().(*shared.HistoryEvent))
		}
		historyBatch = append(historyBatch, historyEvents)
	}

	versionHistory := s.eventBatchesToVersionHistory(nil, historyBatch)

	s.NotNil(historyBatch)
	historyBatch[0].Events[1].Version = common.Int64Ptr(2)
	standbyClient := s.mockFrontendClient["standby"].(*workflowservicetest.MockClient)

	s.applyEventsThroughFetcher(
		workflowID,
		runID,
		workflowType,
		tasklist,
		versionHistory,
		historyBatch,
		historyClient,
		standbyClient,
	)

	time.Sleep(10 * time.Second)

	execMgrFactory := s.active.GetExecutionManagerFactory()
	executionManager, err := execMgrFactory.NewExecutionManager(0)
	s.NoError(err)

	request := persistence.NewGetReplicationTasksFromDLQRequest(
		"standby", -1, math.MaxInt64, math.MaxInt64, nil)
	response, err := executionManager.GetReplicationTasksFromDLQ(request)
	s.NoError(err, "Failed to get messages from DLQ.")
	s.Equal(len(response.Tasks), len(historyBatch))
}
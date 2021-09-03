// Copyright 2021 FabEdge Team
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package framework

import (
	"fmt"
	"math"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/config"
	"github.com/onsi/ginkgo/types"
)

type testSuite struct {
	TestCases []testCase
	Name      string
	Tests     int
	Failures  int
	Errors    int
	Time      float64
}

type testCase struct {
	Name           string
	ClassName      string
	FailureType    string
	FailureMessage string
	Skipped        bool
	Time           float64
	SystemOut      string
}

type tableReporter struct {
	suite     testSuite
	filename  string
	suiteName string
}

func NewTableReporter(filename string) ginkgo.Reporter {
	return &tableReporter{filename: filename}
}

func (reporter *tableReporter) SpecSuiteWillBegin(config config.GinkgoConfigType, summary *types.SuiteSummary) {
	reporter.suite = testSuite{
		Name:      summary.SuiteDescription,
		TestCases: []testCase{},
	}
	reporter.suiteName = summary.SuiteDescription
}

func (reporter *tableReporter) SpecWillRun(specSummary *types.SpecSummary) {
}

func (reporter *tableReporter) BeforeSuiteDidRun(setupSummary *types.SetupSummary) {
	reporter.handleSetupSummary("BeforeSuite", setupSummary)
}

func (reporter *tableReporter) AfterSuiteDidRun(setupSummary *types.SetupSummary) {
	reporter.handleSetupSummary("AfterSuite", setupSummary)
}

func (reporter *tableReporter) handleSetupSummary(name string, summary *types.SetupSummary) {
	if summary.State == types.SpecStatePassed {
		return
	}

	testCase := testCase{
		Name:           name,
		ClassName:      reporter.suiteName,
		FailureType:    failureTypeForState(summary.State),
		FailureMessage: failureMessage(summary.Failure),
		SystemOut:      summary.CapturedOutput,
		Time:           summary.RunTime.Seconds(),
	}
	reporter.suite.TestCases = append(reporter.suite.TestCases, testCase)
}

func (reporter *tableReporter) SpecDidComplete(summary *types.SpecSummary) {
	testCase := testCase{
		Name:      strings.Join(summary.ComponentTexts[1:], " "),
		ClassName: reporter.suiteName,
		Time:      summary.RunTime.Seconds(),
		Skipped:   summary.State == types.SpecStateSkipped || summary.State == types.SpecStatePending,
	}

	if summary.State == types.SpecStateFailed ||
		summary.State == types.SpecStateTimedOut ||
		summary.State == types.SpecStatePanicked {
		testCase.FailureType = failureTypeForState(summary.State)
		testCase.FailureMessage = failureMessage(summary.Failure)
		testCase.SystemOut = summary.CapturedOutput
	}

	reporter.suite.TestCases = append(reporter.suite.TestCases, testCase)
}

func (reporter *tableReporter) SpecSuiteDidEnd(summary *types.SuiteSummary) {
	reporter.suite.Tests = summary.NumberOfSpecsThatWillBeRun
	reporter.suite.Time = math.Trunc(summary.RunTime.Seconds()*1000) / 1000
	reporter.suite.Failures = summary.NumberOfFailedSpecs
	reporter.suite.Errors = 0

	reporter.generateReport()
}

func (reporter *tableReporter) generateReport() {
	file, err := os.Create(reporter.filename)
	if err != nil {
		fmt.Printf("Failed to create report file: %s\n\t%s", reporter.filename, err.Error())
		return
	}
	defer file.Close()

	table := tablewriter.NewWriter(file)
	table.SetHeader([]string{"Test", "Time(s)", "Failure"})
	table.SetAutoWrapText(false)
	table.SetCaption(true, reporter.suiteName)
	table.SetFooter([]string{
		fmt.Sprintf("Tests: %d", reporter.suite.Tests),
		fmt.Sprintf("Failures: %d", reporter.suite.Failures),
		fmt.Sprintf("Errors: %d", reporter.suite.Errors),
		fmt.Sprintf("Times(s): %f", reporter.suite.Time),
	})

	for _, tc := range reporter.suite.TestCases {
		if tc.Skipped {
			continue
		}

		table.Append([]string{
			tc.Name,
			fmt.Sprintf("%f", tc.Time),
			tc.FailureType,
		})
	}
	table.Render()

	if reporter.suite.Errors == 0 && reporter.suite.Failures == 0 {
		return
	}

	output := func(content string) {
		if _, err := file.WriteString(content); err != nil {
			fmt.Printf("Failed to write to report file: %s \n", err)
		}
	}

	output("\n\n-------------------- Error list ------------------------\n\n")
	for _, tc := range reporter.suite.TestCases {
		if tc.FailureType == "" {
			continue
		}
		output(tc.Name + "\n\n")
		output(tc.FailureMessage + "\n\n")
		output("--------------------------------------------------------\n")
	}
}

func failureMessage(failure types.SpecFailure) string {
	return fmt.Sprintf("%s\n%s\n%s", failure.ComponentCodeLocation.String(), failure.Message, failure.Location.String())
}

func failureTypeForState(state types.SpecState) string {
	switch state {
	case types.SpecStateFailed:
		return "Failure"
	case types.SpecStateTimedOut:
		return "Timeout"
	case types.SpecStatePanicked:
		return "Panic"
	default:
		return ""
	}
}

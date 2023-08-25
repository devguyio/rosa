package versions

import (
	. "github.com/onsi/ginkgo/v2/dsl/core"
	. "github.com/onsi/ginkgo/v2/dsl/decorators"
	. "github.com/onsi/ginkgo/v2/dsl/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Version Helpers", Ordered, func() {

	Context("when creating a hosted machine pool ", func() {
		DescribeTable("Filtered versions",
			func(versionList []string, minVersion string, maxVersion string, expectedVersionList []string) {
				filteredVersionList := getFilteredVersionList(versionList, minVersion, maxVersion, false)
				Expect(filteredVersionList).To(BeEquivalentTo(expectedVersionList))
			},
			Entry("machinepool create",
				[]string{
					"4.12.0-rc.8",
					"4.12.1",
					"4.12.2",
					"4.12.3",
					"4.12.4",
					"4.12.5",
					"4.13.0-0.nightly-2023-02-22-192922",
				},
				"4.12.2",
				"4.12.5",
				[]string{
					"4.12.2",
					"4.12.3",
					"4.12.4",
					"4.12.5",
				},
			),
			Entry("machinepool update",
				[]string{
					"4.12.0-rc.8",
					"4.12.1",
					"4.12.2",
					"4.12.3",
					"4.12.4",
					"4.12.5",
					"4.13.0-0.nightly-2023-02-22-192922",
				},
				"4.12.4",
				"4.12.5",
				[]string{
					"4.12.4",
					"4.12.5",
				},
			),
		)

		DescribeTable("Minimal hosted machinepool version",
			func(controlPlaneVersion string, expected string) {
				minimalVersion, err := GetMinimalHostedMachinePoolVersion(controlPlaneVersion)
				Expect(err).ToNot(HaveOccurred())
				Expect(minimalVersion).To(Equal(expected))
			},
			Entry("Future control plane",
				"4.15.0",
				"4.13.0",
			),
			Entry("Nightly control plane",
				"4.14.0-0.nightly-2023-02-27-084419",
				"4.12.0",
			),
			Entry("Current control plane",
				"4.12.5",
				"4.12.0-0.a",
			),
		)

	})
	Context("when updating a hosted machine pool ", func() {
		DescribeTable("Filtered versions",
			func(versionList []string, minVersion string, maxVersion string, expectedVersionList []string) {
				filteredVersionList := getFilteredVersionList(versionList, minVersion, maxVersion, true)
				Expect(filteredVersionList).To(BeEquivalentTo(expectedVersionList))
			},
			Entry("machinepool update",
				[]string{
					"4.12.22",
					"4.12.23",
					"4.12.24",
					"4.12.25",
					"4.12.26",
					"4.13.0-0.nightly-2023-02-22-192922",
				},
				"4.12.22",
				"4.12.26",
				[]string{
					"4.12.23",
					"4.12.24",
					"4.12.25",
					"4.12.26",
				},
			),
		)
	})

})

package upgrade

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestListUpgrades(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "List upgrades suite")
}
